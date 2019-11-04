package main

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nogproject/nog/backend/cmd/tartt/drivers"
	"github.com/nogproject/nog/backend/pkg/iox"
	"github.com/nogproject/nog/backend/pkg/ratelimit"
)

var ErrTarWarning = errors.New("tar warnings")
var ErrTarError = errors.New("tar errors")
var ErrTarFatal = errors.New("fatal tar failure")

type ErrorPolicy int

const (
	WarningFatal ErrorPolicy = 1 + iota
	WarningContinue
	ErrorContinue
)

type WithSecret func(dir string) (string, error)

func cmdTar(args map[string]interface{}) {
	policy := WarningContinue
	switch {
	case args["--warning-fatal"].(bool):
		policy = WarningFatal
	case args["--error-continue"].(bool):
		policy = ErrorContinue
	}

	lockWait := args["--lock-wait"].(time.Duration)

	var limit *ratelimit.Bucket
	if v, ok := args["--limit"].(uint64); ok {
		// Rate from arg, fixed 1 MiB capacity.
		limit = ratelimit.NewBucketWithRate(float64(v), 1024*1024)
	}

	var hook string
	if arg, ok := args["--full-hook"].(string); ok {
		hook = arg
	}

	var storeExtraArgs []string
	var storeMetadataExtraArgs []string
	var withSecret WithSecret
	if rs := args["--recipient"].([]string); len(rs) > 0 {
		storeExtraArgs = append(storeExtraArgs,
			"--split-zstd-gpg-split",
			"--cipher-algo", args["--cipher-algo"].(string),
		)
		storeMetadataExtraArgs = append(storeMetadataExtraArgs,
			"--gpg",
			"--cipher-algo", args["--cipher-algo"].(string),
		)
		withSecret = func(dir string) (string, error) {
			file := filepath.Join(dir, "secret.asc")
			return newArmoredSecret(file, rs)
		}
	} else if args["--plaintext-secret"].(bool) {
		storeExtraArgs = append(storeExtraArgs,
			"--split-zstd-gpg-split",
			"--cipher-algo", args["--cipher-algo"].(string),
		)
		storeMetadataExtraArgs = append(storeMetadataExtraArgs,
			"--gpg",
			"--cipher-algo", args["--cipher-algo"].(string),
		)
		withSecret = func(dir string) (string, error) {
			file := filepath.Join(dir, "secret")
			return newPlaintextSecret(file)
		}
	} else if args["--insecure-plaintext"].(bool) {
		storeExtraArgs = append(storeExtraArgs,
			"--split-zstd-split",
		)
		storeMetadataExtraArgs = append(storeMetadataExtraArgs,
			"--split-zstd-split",
		)
	} else {
		panic("args logic error")
	}

	repo, err := OpenRepo(".")
	if err != nil {
		lg.Fatalw("Failed to open repo.", "err", err)
	}
	defer repo.Close()

	storeName, ok := args["--store"].(string)
	if !ok {
		storeName = repo.DefaultStoreName()
	}
	store, err := repo.OpenStore(storeName)
	if err != nil {
		lg.Fatalw("Failed to open store.", "err", err)
	}
	defer store.Close()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, lockWait)
	if err := store.TryLock(ctx); err != nil {
		cancel()
		lg.Fatalw(
			"Failed to lock store.",
			"store", store.Dir(),
			"err", err,
		)
	}
	cancel()
	defer store.Unlock()

	now := time.Now().UTC()
	loc := func() AppendLocation {
		if args["--full"].(bool) {
			loc, err := store.WhereAppendFull(now)
			if err != nil {
				lg.Fatalw(
					"Failed to find path for full archive.",
					"err", err,
				)
			}
			return loc
		} else {
			tree, err := store.LsTree()
			if err != nil {
				lg.Fatalw("Failed to list tree.", "err", err)
			}
			loc, err := store.WhereAppend(now, tree)
			if err != nil {
				lg.Fatalw(
					"Failed to find path for new archive.",
					"err", err,
				)
			}
			return loc
		}
	}()

	dstRel := loc.TarPath()
	dst := store.AbsPath(dstRel)
	if exists(dst) {
		lg.Fatalw("The archive already exists.", "dest", dst)
	}

	var parent string
	switch loc.TarType {
	case TarFull:
		lg.Infow("Started full archive.", "dest", dst)
		parent = "" // Indicates full archive.
	case TarPatch:
		lg.Infow("Started incremental archive.", "dest", dst)
		parent = store.AbsPath(loc.ParentTarPath())
	default:
		panic("invalid TarType")
	}
	err = archive(
		dst, parent, repo.OriginDir(),
		policy,
		limit,
		storeExtraArgs,
		storeMetadataExtraArgs,
		withSecret,
		hook,
		store.ArchiveHandler(),
		dstRel,
	)
	if errorPolicyShouldStop(policy, err) {
		lg.Fatalw("Failed to create archive.", "dest", dst, "err", err)
	}
	switch err {
	case nil:
		lg.Infow("Completed archive.", "dest", dst)
		os.Exit(0)
	case ErrTarWarning:
		lg.Warnw("Completed archive with tar warnings.", "dest", dst)
		os.Exit(10)
	case ErrTarError:
		lg.Errorw("Completed archive with tar errors.", "dest", dst)
		os.Exit(11)
	default:
		// errorPolicyShouldStop() handles unknown error.
		panic("logic error")
	}
}

func archive(
	dst, parent, origin string,
	policy ErrorPolicy,
	limit *ratelimit.Bucket,
	storeExtraArgs []string,
	storeMetadataExtraArgs []string,
	withSecret WithSecret,
	hook string,
	handler drivers.ArchiveHandler,
	dstRel string,
) error {
	tmp, err := mkdirArchiveInProgress(dst)
	if err != nil {
		return err
	}

	// If `tmp` still exists on return, rename it to `${dst}.error`.
	//
	// But abort the handler transaction first; see next defer below.
	defer func() {
		if exists(tmp) {
			_ = os.Rename(tmp, fmt.Sprintf("%s.error", dst))
		}
	}()

	// Copy the exclude list if it exists.
	if exists(excludePath(".")) {
		err := cp(excludePath("."), excludePath(tmp))
		if err != nil {
			err := fmt.Errorf("failed to copy `exclude`: %v", err)
			return err
		}
		lg.Infow("Copied anchored tar exclude patterns.")
	}

	// `har` is the new archive that is managed by the handler.  It is
	// designed as a transaction, so that the handler can use temporary
	// files that are either moved in place on `Commit()` or deleted on
	// `Abort()`.
	har, err := handler.BeginArchive(dstRel, tmp)
	if err != nil {
		return err
	}
	// Abort the transaction before removing `tmp`; see defer above.
	defer func() {
		if exists(tmp) {
			_ = har.Abort()
		}
	}()

	// `metadataFiles` are included in `metadata.tar`.
	var metadataFiles []string
	// `deleteFiles` are deleted after packing `metadata.tar`.
	var deleteFiles []string
	// `readmeMore` are text blocks for `README.md`.
	var readmeMore []string

	if parent != "" {
		if err := cp(snarPath(parent), snarPath(tmp)); err != nil {
			err := fmt.Errorf("failed to copy snar file: %v", err)
			return err
		}
	}

	secret := ""
	if withSecret != nil {
		s, err := withSecret(tmp)
		if err != nil {
			return err
		}
		secret = s
	}

	errTar := tarIncremental(
		har, tmp, origin, limit, storeExtraArgs, secret,
	)
	if errorPolicyShouldStop(policy, errTar) {
		return errTar
	}

	if parent == "" && hook != "" {
		fs, err := runHook(hook, tmp)
		if err != nil {
			return err
		}
		for _, f := range fs {
			if f == "README.md" {
				// Read and remove right away.
				file := filepath.Join(tmp, f)
				b, err := ioutil.ReadFile(file)
				if err != nil {
					return err
				}
				readmeMore = append(readmeMore, string(b))
				if err := os.Remove(file); err != nil {
					return err
				}
			} else {
				// Append for `metadata.tar` and deletion.
				metadataFiles = append(metadataFiles, f)
				deleteFiles = append(deleteFiles, f)
			}
		}
	}

	// Always store logs as metadata.
	logs, err := lsLogs(tmp)
	if err != nil {
		return err
	}
	metadataFiles = append(metadataFiles, logs...)

	if len(metadataFiles) > 0 {
		err := tarMetadata(
			har,
			tmp, metadataFiles, storeMetadataExtraArgs, secret,
		)
		if err != nil {
			return err
		}
	}

	// Delete plaintext files from disk after they have been packed and
	// encrypted as metadata.
	for _, f := range deleteFiles {
		if err := os.Remove(filepath.Join(tmp, f)); err != nil {
			return err
		}
	}

	// Save README in full archive, because a full archive may be kept for
	// a long time and should be self-explaining.  Incremental archives
	// should be kept only temporarily.
	if parent == "" {
		if err := saveReadme(har, tmp, readmeMore); err != nil {
			return err
		}
	}

	if err := har.Commit(); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}

	return errTar
}

// `mkdirArchiveInProgress(dest)` creates a preliminary archive directory whose
// final location is `dest`.  The parent directories of the final location are
// created, so that the preliminary directory can later be simply renamed to
// `dest`.
//
// `dest` must be a pathname `<prefix>/<level>/<ts>/<type>`, where `<level>`
// may be a store toplevel directory, for example
// `.../stores/foo/2018-10-07T112102Z/full`, with the following restrictions:
//
//  - The `<prefix>` directory must already exist.
//  - The `<level>` directory may already exits.
//  - The `<ts>` directory must not yet exist.
//
// These restrictions protect against a potential race where a previous command
// failed and created `<prefix>/<level>/<ts>/<type>.error`.  In this case, the
// `<ts>` directory should not be re-used for a successful archive in order to
// avoid potential confusion during garbage collection.
func mkdirArchiveInProgress(dest string) (string, error) {
	tmp := fmt.Sprintf("%s.inprogress", dest)
	ts := filepath.Dir(tmp)
	level := filepath.Dir(ts)
	if err := os.Mkdir(level, 0777); err != nil {
		// Ok if `<level>` already existed.
		if err.(*os.PathError).Err != syscall.EEXIST {
			return "", err
		}
	}
	if err := os.Mkdir(ts, 0777); err != nil {
		return "", err
	}
	if err := os.Mkdir(tmp, 0777); err != nil {
		return "", err
	}
	return tmp, nil
}

func errorPolicyShouldStop(policy ErrorPolicy, errTar error) bool {
	switch errTar {
	case nil: // ok
	case ErrTarWarning:
		if policy == WarningFatal {
			return true
		}
	case ErrTarError:
		if policy != ErrorContinue {
			return true
		}
	default:
		return true
	}
	return false
}

func saveReadme(
	har drivers.ArchiveTx,
	dst string,
	more []string,
) error {
	// Delegate saving data to a separate command.
	saveArgs := []string{
		"save", "--direct", "README.md",
	}
	cmd := exec.Command(
		har.SaveProgram(tarttStoreTool.Path),
		har.SaveArgs(saveArgs)...,
	)
	cmd.Dir = dst
	cmdStdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	var errWrite error
	writeString := func(s string) {
		if errWrite != nil {
			return
		}
		_, errWrite = io.WriteString(cmdStdin, s)
	}

	writeString(readmeMd)
	for _, m := range more {
		writeString("\n---\n\n")
		writeString(m)
	}
	if err := cmdStdin.Close(); errWrite == nil {
		errWrite = err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return errWrite
}

func runHook(hook string, dst string) ([]string, error) {
	cmd := exec.Command("bash")
	cmd.Dir = dst
	cmd.Stdin = strings.NewReader(hook)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	files := strings.Split(string(bytes.TrimSpace(out)), "\n")
	return files, nil
}

/* #TAROPTIONS

`--listed-incremental-mtime` is available in the patched GNU tar from
<https://github.com/sprohaska/gnu-tar> branch `next^`.  It works like
`--listed-incremental` but uses only mtime to detect modified files, ignoring
ctime.

`--no-check-device` tells GNU Tar to always ignore device numbers.  We've
observed unexpectedly many files in incremental dumps on an NFS filesystem,
although Tar should ignore device numbers on NFS filesystems by default.
Assuming a reasonable setup, in which files never move between filesystems, it
should always be safe to ignore device numbers.

Do not use `--atime-preserve=system`.  It would tell GNU Tar to open files with
`O_NOATIME` and thus leave atime unmodified, which seems desirable at first
glance, see <https://www.gnu.org/software/tar/manual/html_section/tar_69.html>.
But Tar would require `CAP_FOWNER`, see
<http://man7.org/linux/man-pages/man2/open.2.html>.  If it does not have the
capability, it will fail to open files that have a different owner than the
user who runs Tar, which could cause unexpected problems.  Therefore, we do not
use `--atime-preserve=system`.

Note that performance should not be a concern on a modern Linux, which uses the
mount option `relatime` by default since Linux 2.6.30, released in 2009, see
<https://unix.stackexchange.com/a/17849>.

If we wanted to preserve atime nonetheless, for example to completely hide from
users the fact that Tar read a file, we could add a Tartt option to explicitly
enable `--atime-preserve=system` for scenarios where Tar is guaranteed to have
`CAP_FOWNER`.

`--sparse` enables hole detection to efficiently store sparse files.

*/

func tarIncremental(
	har drivers.ArchiveTx,
	dst, origin string,
	limit *ratelimit.Bucket,
	storeExtraArgs []string,
	secret string,
) error {
	// Use `os.Pipe()` to copy data directly between sub-processes and not
	// through `tartt`, unless needed for rate limiting.
	//
	// `tar | save` or `tar | limit | save`.
	tarSavePipe, err := iox.WrapPipe3(os.Pipe())
	if err != nil {
		return err
	}
	defer tarSavePipe.CloseBoth()
	// `tar stderr | awk`.
	tarAwkPipe, err := iox.WrapPipe3(os.Pipe())
	if err != nil {
		return err
	}
	defer tarAwkPipe.CloseBoth()

	tarFeatures, err := detectTarFeatures()
	if err != nil {
		return err
	}

	// See comment #TAROPTIONS.
	var optListed string
	if tarFeatures.ListedIncrementalMtime {
		optListed = "--listed-incremental-mtime"
	} else {
		optListed = "--listed-incremental"
		lg.Warnw(
			"Using tar --listed-incremental, " +
				"which uses mtime and ctime " +
				"to detect modified files.",
		)
	}

	tarArgs := []string{
		"--create",
		"--verbose",
		"--file=-", // pipe to `saveCmd`.
		fmt.Sprintf("%s=%s", optListed, snarPath(dst)),
		"--no-check-device", // See comment #TAROPTIONS.
		// No `--atime-preserve=system`, see comment #TAROPTIONS.
		"--sparse", // See comment #TAROPTIONS.
	}
	if exists(excludePath(dst)) {
		tarArgs = append(tarArgs,
			"--anchored",
			fmt.Sprintf("--exclude-from=%s", excludePath(dst)),
		)
	}

	// If origin does not exist, use a temporary placeholder directory.
	// Using `--files-from=/dev/null`, as suggested in
	// <http://www.wezm.net/technical/2008/03/create-empty-tar-file/>, does
	// not work together with `--listed-incremental`.  But we want
	// `--listed-incremental`, so that `tartt` works as expected if origin
	// re-appears.
	if ok, err := tarttIsDir(origin); err != nil {
		return err
	} else if ok {
		tarArgs = append(tarArgs,
			fmt.Sprintf("--directory=%s", origin),
			".",
		)
	} else {
		lg.Warnw(
			"Origin does not exist; storing placeholder tar.",
			"origin", origin,
		)
		placeholder, err := mktempOriginPlaceholder()
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(placeholder) }()
		tarArgs = append(tarArgs,
			fmt.Sprintf("--directory=%s", placeholder),
			".",
		)
	}

	tarCmd := exec.Command(tarTool.Path, tarArgs...)
	tarCmd.Env = append(os.Environ(),
		// Ensure English for awk processing below.  See
		// <http://perlgeek.de/en/article/set-up-a-clean-utf8-environment>
		// for relevant env variables.
		//
		// But use `C.UTF-8` instead of `en_US.UTF-8`.  Most distros
		// seem to include `C.UTF-8` now as a fallback.  There is an
		// ongoing discussion to add `C.UTF-8` to glibc,
		// <https://sourceware.org/bugzilla/show_bug.cgi?id=17318>.
		// Ubuntu and Docker container base images come with only the
		// locales `C`, `C.UTF-8`, and `POSIX`.
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
		"LANGUAGE=C.UTF-8",
	)
	tarCmd.Stdin = nil
	tarCmd.Stdout = tarSavePipe.W
	tarCmd.Stderr = tarAwkPipe.W

	// Delegate saving data to a separate command.  Currently, there is
	// only `tartt-store`.  In the future, the command may depend on the
	// store driver.
	//
	// If we have a secret, pass it via fd 3.
	saveArgs := append([]string{"save"}, storeExtraArgs...)
	if secret != "" {
		saveArgs = append(saveArgs,
			"--secret-fd=3",
		)
	}
	saveArgs = append(saveArgs,
		"data.tar",
	)
	saveCmd := exec.Command(
		har.SaveProgram(tarttStoreTool.Path),
		har.SaveArgs(saveArgs)...,
	)
	saveCmd.Dir = dst
	if limit == nil {
		saveCmd.Stdin = tarSavePipe.R
	} else {
		saveCmd.Stdin = ratelimit.Reader(tarSavePipe.R, limit)
	}
	saveCmd.Stdout = os.Stdout
	saveCmd.Stderr = os.Stderr

	// Pass secret via fd 3.
	var secDone chan error
	if secret != "" {
		secDone = make(chan error)
		secR, secW, err := os.Pipe()
		if err != nil {
			return err
		}
		defer secR.Close()
		// `secW.Close()` in goroutine after write.
		saveCmd.ExtraFiles = []*os.File{
			secR, // fd 3
		}
		go func() {
			_, err := io.WriteString(secW, secret)
			if err2 := secW.Close(); err == nil {
				err = err2
			}
			secDone <- err
		}()
	}

	// Split tar stderr into multiple files using awk; see
	// <https://unix.stackexchange.com/a/414043>.
	//
	// Lines that start with `./` are the archived paths.  Tar reports them
	// to stderr when writing the data to stdout.
	awkCmd := exec.Command(awkTool.Path, `
/^[.][/]/{ print >"out.log"; fflush("out.log"); next }
/: Directory is new$/{ print >"info.log"; fflush("info.log"); next }
/: Directory has been renamed$/{ print >"info.log"; fflush("info.log"); next }
/: Directory has been renamed from /{ print >"info.log"; fflush("info.log"); next }
/Cannot open: Permission denied$/{ print >"error.log"; fflush("error.log"); next }
/: File removed before we read it$/{ print >"error.log"; fflush("error.log"); next }
/: file changed as we read it$/{ print >"error.log"; fflush("error.log"); next }
/Exiting with failure status due to previous errors$/{ print >"error.log"; fflush("error.log"); next }
/.*/{ print >"fatal.log"; fflush("fatal.log"); next }
`)
	awkCmd.Dir = dst
	awkCmd.Stdin = tarAwkPipe.R
	awkCmd.Stdout = os.Stdout
	awkCmd.Stderr = os.Stderr

	// Start children.  When all are started, concurrently wait for them,
	// so that unexpected exits are handled in any order.
	if err := tarCmd.Start(); err != nil {
		return err
	}
	if err := saveCmd.Start(); err != nil {
		_ = tarCmd.Process.Kill()
		_ = tarCmd.Wait()
		return err
	}
	if err := awkCmd.Start(); err != nil {
		_ = saveCmd.Process.Kill()
		_ = saveCmd.Wait()
		_ = tarCmd.Process.Kill()
		_ = tarCmd.Wait()
		return err
	}

	// See `secDone` above.
	tarDone := make(chan error)
	saveDone := make(chan error)
	awkDone := make(chan error)
	allAreDone := func() bool {
		return tarDone == nil && saveDone == nil && secDone == nil &&
			awkDone == nil
	}

	go func(done chan<- error) {
		done <- tarCmd.Wait()
		close(done)
	}(tarDone)

	go func(done chan<- error) {
		done <- saveCmd.Wait()
		close(done)
	}(saveDone)

	go func(done chan<- error) {
		done <- awkCmd.Wait()
		close(done)
	}(awkDone)

	// `forceExit()` kills all children that are not yet done.
	forceExit := func() {
		if tarDone != nil {
			tarCmd.Process.Kill()
		}
		if saveDone != nil {
			saveCmd.Process.Kill()
		}
		if awkDone != nil {
			awkCmd.Process.Kill()
		}
	}

	var errForce error
	var errTar error
	var errSave error
	var errSec error
	var errAwk error
	for !allAreDone() {
		select {
		case errTar = <-tarDone:
			tarDone = nil
			if err := tarSavePipe.CloseW(); errTar == nil {
				errTar = err
			}
			if err := tarAwkPipe.CloseW(); errTar == nil {
				errTar = err
			}
		case errSave = <-saveDone:
			saveDone = nil
			// If save failed before tar has completed, force
			// cleanup, ignoring further errors.
			if errSave != nil && tarDone != nil && errForce == nil {
				forceExit()
				errForce = fmt.Errorf(
					"save failed early: %v", errSave,
				)
			}
		case errSec = <-secDone:
			secDone = nil
		case errAwk = <-awkDone:
			awkDone = nil
			// If awk failed before tar has completed, force
			// cleanup, ignoring further errors.
			if errAwk != nil && tarDone != nil && errForce == nil {
				forceExit()
				errForce = fmt.Errorf(
					"awk failed early: %v", errSave,
				)
			}
		}
	}

	if errForce != nil {
		return errForce
	}
	// If tar fails:
	//
	//  - no exit code: fatal.
	//  - code 1 "some files differ": warning.
	//  - other code without `fatal.log`: error.
	//  - other code with `fatal.log`: fatal.
	//
	// This approach allows us to add known error messages to awk in order
	// to handle them as warnings.
	if errTar != nil {
		exitError, ok := errTar.(*exec.ExitError)
		if !ok {
			return errTar
		}
		code := exitError.Sys().(syscall.WaitStatus).ExitStatus()
		if code == 1 { // GNU tar "Some files differ"
			return ErrTarWarning
		}
		if exists(filepath.Join(dst, "fatal.log")) {
			return ErrTarFatal
		}
		return ErrTarError
	}
	if errSave != nil {
		return fmt.Errorf("failed to save tar data: %v", errSave)
	}
	if errSec != nil {
		return fmt.Errorf("failed to pass save secret: %v", errSec)
	}
	if errAwk != nil {
		return fmt.Errorf("awk on tar stderr failed: %v", errAwk)
	}
	return nil
}

func tarMetadata(
	har drivers.ArchiveTx,
	dst string, files []string,
	storeMetadataExtraArgs []string, secret string,
) error {
	// `tar | save`
	tarSavePipe, err := iox.WrapPipe3(os.Pipe())
	if err != nil {
		return err
	}
	defer tarSavePipe.CloseBoth()

	// `tar 2>"metadata.log"`
	tarStderrFp, err := os.Create(filepath.Join(dst, "metadata.log"))
	if err != nil {
		return err
	}
	// Fallback if close after `tarCmd.Wait()` is not reached.
	defer func() {
		if tarStderrFp != nil {
			_ = tarStderrFp.Close()
		}
	}()

	tarArgs := []string{
		"--create",
		"--verbose",
		// Pass list of files in `dst` via stdin.
		"--directory", dst, "--files-from", "-",
		"--file", "-", // to `saveCmd`.
	}
	tarCmd := exec.Command(tarTool.Path, tarArgs...)
	tarCmd.Stdin = strings.NewReader(strings.Join(files, "\n"))
	tarCmd.Stdout = tarSavePipe.W
	tarCmd.Stderr = tarStderrFp

	// Delegate save to `tartt-store`.
	//
	// If we have a secret, pass it via fd 3.
	saveArgs := append([]string{"save"}, storeMetadataExtraArgs...)
	if secret != "" {
		saveArgs = append(saveArgs,
			"--secret-fd=3",
		)
	}
	saveArgs = append(saveArgs,
		"metadata.tar",
	)
	saveCmd := exec.Command(
		har.SaveProgram(tarttStoreTool.Path),
		har.SaveArgs(saveArgs)...,
	)
	saveCmd.Dir = dst
	saveCmd.Stdin = tarSavePipe.R
	saveCmd.Stdout = os.Stdout
	saveCmd.Stderr = os.Stderr

	// Pass secret via fd 3.
	var secDone chan error
	if secret != "" {
		secDone = make(chan error)
		secR, secW, err := os.Pipe()
		if err != nil {
			return err
		}
		defer secR.Close()
		// `secW.Close()` in goroutine after write.
		saveCmd.ExtraFiles = []*os.File{
			secR, // fd 3
		}
		go func() {
			_, err := io.WriteString(secW, secret)
			if err2 := secW.Close(); err == nil {
				err = err2
			}
			secDone <- err
		}()
	}

	if err := tarCmd.Start(); err != nil {
		return err
	}
	if err := saveCmd.Start(); err != nil {
		_ = tarCmd.Wait()
		return err
	}

	errTar := tarCmd.Wait()
	if err := tarSavePipe.CloseW(); errTar == nil {
		errTar = err
	}
	if err := tarStderrFp.Close(); errTar == nil {
		errTar = err
	}
	tarStderrFp = nil

	errSave := saveCmd.Wait()
	var errSec error
	if secret != "" {
		errSec = <-secDone
	}

	if errTar != nil {
		return fmt.Errorf("failed to tar metadata: %v", errTar)
	}
	if errSave != nil {
		return fmt.Errorf("failed to save metadata tar: %v", errSave)
	}
	if errSec != nil {
		return fmt.Errorf("failed to pass save secret: %v", errSec)
	}

	return nil
}

func newArmoredSecret(file string, gpgIds []string) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", err
	}

	// The secret itself remains fixed.  To allow key rotation, the secret
	// is encrypted to GPG recipients.  Any of the recipients can restore
	// data or re-encrypt the secret to change the recipients.
	//
	// Always use AES256 to make paranoid users happy.  Secrets are so
	// small that speed is irrelevant.
	gpgArgs := []string{
		"--batch",
		"--encrypt", "--armor",
		"--cipher-algo", "AES256",
	}
	for _, r := range gpgIds {
		gpgArgs = append(gpgArgs,
			"--recipient", r,
		)
	}
	gpgCmd := exec.Command(gpg2Tool.Path, gpgArgs...)
	gpgCmd.Stderr = os.Stderr
	gpgCmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n", secret))

	fp, err := os.Create(file)
	if err != nil {
		return "", err
	}
	defer fp.Close()
	gpgCmd.Stdout = fp
	if err := gpgCmd.Run(); err != nil {
		return "", err
	}
	if err := fp.Close(); err != nil {
		return "", err
	}

	return secret, nil
}

func newPlaintextSecret(file string) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(file, []byte(secret+"\n"), 0640)
	if err != nil {
		return "", err
	}

	return secret, nil
}

func newSecret() (string, error) {
	const nBits = 256
	rnd := make([]byte, nBits/8)
	_, err := crand.Read(rnd)
	if err != nil {
		return "", err
	}
	secret := fmt.Sprintf("S%x", rnd)
	return secret, nil
}

func lsLogs(dir string) ([]string, error) {
	fp, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	ls, err := fp.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	var logs []string
	for _, n := range ls {
		if strings.HasSuffix(n, ".log") {
			logs = append(logs, n)
		}
	}

	return logs, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func snarPath(dir string) string {
	return filepath.Join(dir, "origin.snar")
}

func excludePath(dir string) string {
	return filepath.Join(dir, "exclude")
}

func cp(src, dst string) error {
	cmd := exec.Command(cpTool.Path, src, dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// `tarttIsDir()` runs `tartt-is-dir` to determine whether `path` is a
// directory.  It uses a separate program, because capabilities may be required
// to read origin.
//
// See `tartt-is-dir --help` for exit codes.
func tarttIsDir(path string) (bool, error) {
	out, err := exec.Command(tarttIsDirTool.Path, "--", path).Output()
	if err == nil {
		return true, nil
	}
	if errExit, ok := err.(*exec.ExitError); ok {
		code := errExit.Sys().(syscall.WaitStatus).ExitStatus()
		if code == 10 {
			return false, nil
		}
	}
	msg := strings.TrimSpace(string(out))
	err = fmt.Errorf("`tartt-is-dir` failed: %s; %v", msg, err)
	return false, err
}

func mktempOriginPlaceholder() (string, error) {
	tmp, err := ioutil.TempDir("", "tartt-origin-placeholder")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		return "", err
	}

	var msg = strings.TrimSpace(`
The archive contains only this placeholder, because the origin directory did
not exist when the archive was created.
`) + "\n"
	readme := filepath.Join(tmp, "README-NODATA.txt")
	if err := ioutil.WriteFile(readme, []byte(msg), 0644); err != nil {
		return "", err
	}

	return tmp, nil
}
