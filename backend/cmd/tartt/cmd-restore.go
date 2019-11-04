package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	slashpath "path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nogproject/nog/backend/cmd/tartt/drivers"
	"github.com/nogproject/nog/backend/pkg/iox"
	"github.com/nogproject/nog/backend/pkg/ratelimit"
)

type UntarOptions struct {
	SameOwner       bool
	SamePermissions bool
}

func NewUntarOptionsFromArgs(args map[string]interface{}) *UntarOptions {
	o := &UntarOptions{
		SameOwner:       true,
		SamePermissions: true,
	}
	if args["--no-same-owner"].(bool) {
		o.SameOwner = false
	}
	if args["--no-same-permissions"].(bool) {
		o.SamePermissions = false
	}
	return o
}

func (o *UntarOptions) TarArgs() []string {
	args := make([]string, 0, 2)

	if o.SameOwner {
		args = append(args, "--same-owner")
	} else {
		args = append(args, "--no-same-owner")
	}

	if o.SamePermissions {
		args = append(args, "--same-permissions")
	} else {
		args = append(args, "--no-same-permissions")
	}

	return args
}

func cmdRestore(args map[string]interface{}) {
	dest := args["--dest"].(string)
	if !filepath.IsAbs(dest) {
		lg.Fatalw("--dest must be an absolute path.")
	}
	if !isEmptyDir(dest) {
		lg.Fatalw("--dest is not an empty dir.")
	}

	tspath := args["<tspath>"].(string)
	members := args["<members>"].([]string)
	untarOpts := NewUntarOptionsFromArgs(args)

	var limit *ratelimit.Bucket
	if v, ok := args["--limit"].(uint64); ok {
		// Rate from arg, fixed 1 MiB capacity.
		limit = ratelimit.NewBucketWithRate(float64(v), 1024*1024)
	}

	repo, err := OpenRepo(".")
	if err != nil {
		lg.Fatalw("Failed to open repo.", "err", err)
	}
	defer repo.Close()

	storeName, relTspath, err := SplitStoreTspath(tspath)
	if err != nil {
		lg.Fatalw("Invalid path.", "tspath", tspath)
	}

	store, err := repo.OpenStore(storeName)
	if err != nil {
		lg.Fatalw("Failed to open store.", "err", err)
	}
	defer store.Close()

	if args["--no-lock"].(bool) {
		// Don't lock the store, which MUST be safe with concurrent
		// operations that only append to the store, like tar.  It
		// MAY be unsafe with concurrent operations that may delete
		// content, like gc.  If in doubt, document the behavior for
		// individual operations.
	} else {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
	}

	tree, err := store.LsTree()
	if err != nil {
		lg.Fatalw("Failed to list tree.", "err", err)
	}

	archives, err := store.GatherArchives(tree, relTspath)
	if err != nil {
		lg.Fatalw("Failed to gather archives for tspath.", "err", err)
	}

	var secrets map[string]string
	if args["--no-preload-secrets"].(bool) {
		lg.Infow("Skipped preloading secrets.")
	} else {
		secrets = loadSecretsMust(store, archives)
	}
	if file, ok := args["--notify-preload-secrets-done"].(string); ok {
		notifyFileMust(file, "preload-secrets-done")
	}

	unh := store.UntarHandler()
	var errFirst error
	for _, ar := range archives {
		lg.Infow("Started untar.", "archive", ar.Path)
		err := untarIncremental(
			dest, store.AbsPath(ar.Path),
			limit,
			secrets[ar.Path],
			unh, ar.Path,
			members, untarOpts,
		)
		if err != nil {
			lg.Warnw("Untar failed; continuing.", "err", err)
		}
		if errFirst == nil {
			errFirst = err
		}
		lg.Infow("Completed untar.", "archive", ar.Path)
	}
	if errFirst != nil {
		lg.Fatalw(
			"Exiting with fatal error due to previous errors.",
			"errFirst", errFirst,
		)
	}
}

func loadSecretsMust(store *Store, archives []Archive) map[string]string {
	secrets := make(map[string]string)
	for _, ar := range archives {
		plain := filepath.Join(store.AbsPath(ar.Path), "secret")
		crypt := plain + ".asc"
		switch {
		case exists(crypt):
			sec, err := loadEncryptedSecret(crypt)
			if err != nil {
				lg.Fatalw(
					"Failed to preload encrypted secret.",
					"archive", ar.Path,
					"secret", crypt,
					"err", err,
				)
			}
			secrets[ar.Path] = sec
			lg.Infow(
				"Preloaded encrypted secret.",
				"archive", ar.Path,
			)

		case exists(plain):
			sec, err := loadPlaintextSecret(plain)
			if err != nil {
				lg.Fatalw(
					"Failed to preload plaintext secret.",
					"archive", ar.Path,
					"secret", plain,
					"err", err,
				)
			}
			secrets[ar.Path] = sec
			lg.Infow(
				"Preloaded plaintext secret.",
				"archive", ar.Path,
			)

		default:
			err := errors.New(
				"found neither file `secret.asc` nor `secret`",
			)
			lg.Fatalw(
				"Failed to preload secret.",
				"archive", ar.Path,
				"err", err,
			)
		}
	}
	lg.Infow(
		"Completed preloading secrets.",
		"n", len(secrets),
	)
	return secrets
}

func notifyFileMust(file, message string) {
	fp, err := os.OpenFile(file, os.O_WRONLY, 0)
	if err == nil {
		_, err = io.WriteString(fp, message+"\n")
	}
	if err == nil {
		err = fp.Close()
	}
	if err != nil {
		lg.Fatalw(
			"Failed to notify.",
			"file", file,
			"message", message,
			"err", err,
		)
	}
}

const TarMsgNotFound = "Not found in archive"
const TarMsgExitFailure = "Exiting with failure status due to previous errors"

func untarIncremental(
	dest, archive string,
	limit *ratelimit.Bucket,
	secret string,
	unh drivers.UntarHandler,
	arRel string,
	members []string,
	untarOpts *UntarOptions,
) error {
	// Use `os.Pipe()` to copy data directly between sub-processes and not
	// through `tartt`, unless needed for rate limiting.
	//
	// `load | tar` or `load | limit | tar`.
	loadTarPipe, err := iox.WrapPipe3(os.Pipe())
	if err != nil {
		return err
	}
	defer loadTarPipe.CloseBoth()

	// Delegate loading data to a separate command.  Currently, there is
	// only `tartt-store`.  In the future, the command may depend on the
	// store driver.
	loadArgs := []string{"load"}
	if secret != "" {
		loadArgs = append(loadArgs, "--secret-stdin")
	}
	loadArgs = append(loadArgs,
		"data.tar",
	)
	loadCmd := exec.Command(
		unh.LoadProgram(tarttStoreTool.Path),
		unh.LoadArgs(arRel, loadArgs)...,
	)
	loadCmd.Dir = archive
	if secret == "" {
		loadCmd.Stdin = nil
	} else {
		loadCmd.Stdin = strings.NewReader(secret)
	}
	loadCmd.Stdout = loadTarPipe.W
	loadCmd.Stderr = os.Stderr

	args := []string{
		"--extract",
		"--verbose",
		"--listed-incremental=/dev/null",
		"--file=-", // from `loadCmd`.
		fmt.Sprintf("--directory=%s", dest),
	}
	args = append(args, untarOpts.TarArgs()...)
	args = append(args, "--")
	args = append(args, members...)
	tarCmd := exec.Command(tarTool.Path, args...)
	if limit == nil {
		tarCmd.Stdin = loadTarPipe.R
	} else {
		tarCmd.Stdin = ratelimit.Reader(loadTarPipe.R, limit)
	}
	tarCmd.Stdout = os.Stdout
	tarStderr, err := tarCmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := loadCmd.Start(); err != nil {
		return err
	}
	if err := tarCmd.Start(); err != nil {
		_ = loadCmd.Process.Kill()
		_ = loadCmd.Wait()
		return err
	}

	// Analyze Tar stderr to ignore 'not found in archive' errors.
	var errScan error
	tarLines := bufio.NewScanner(tarStderr)
	for tarLines.Scan() {
		msg := tarLines.Text()
		if strings.HasSuffix(msg, TarMsgNotFound) {
			lg.Infow(msg)
			continue
		}
		if strings.HasSuffix(msg, TarMsgExitFailure) {
			continue
		}
		if errScan == nil {
			errScan = fmt.Errorf("tar stderr message: %s", msg)
		}
		fmt.Fprint(os.Stderr, msg)
	}
	if err := tarLines.Err(); errScan == nil {
		errScan = err
	}

	errLoad := loadCmd.Wait()
	if err := loadTarPipe.CloseW(); errLoad == nil {
		errLoad = err
	}

	errTar := tarCmd.Wait()

	if errLoad != nil {
		return fmt.Errorf("failed to load tar data: %v", errLoad)
	}
	if errTar != nil {
		err := fmt.Errorf("untar failed: %v", errTar)
		if errScan != nil {
			return err
		}
		exitError, ok := errTar.(*exec.ExitError)
		if !ok {
			return err
		}
		code := exitError.Sys().(syscall.WaitStatus).ExitStatus()
		if code != 2 {
			return err
		}
		lg.Infow("Ignored tar 'not found in archive' errors.")
	}
	if errScan != nil {
		return fmt.Errorf("failed to parse untar stderr: %v", errScan)
	}
	return nil
}

var ErrMalformedTspath = errors.New("malformed tspath")

type Archive struct {
	Path    string
	TarType TarType
}

func (s *Store) GatherArchives(
	t *Tree, tspathString string,
) ([]Archive, error) {
	if t.Root == nil {
		return nil, errors.New("missing root")
	}

	tspath := strings.Split(tspathString, "/")
	if len(tspath) == 0 {
		return nil, ErrMalformedTspath
	}

	ts := tspath[0]
	for _, child := range t.Root.Times {
		childName := child.Name()
		if childName != ts {
			continue
		}
		ar := Archive{
			Path: slashpath.Join(
				childName,
				child.TarType.Path(),
			),
			TarType: child.TarType,
		}
		childBaks, err := child.gatherArchives(tspath[1:], childName)
		if err != nil {
			return nil, err
		}
		return append([]Archive{ar}, childBaks...), nil
	}

	return nil, fmt.Errorf("missing full archive `%s`", ts)
}

func (t *TimeTree) gatherArchives(
	tspath []string, prefix string,
) ([]Archive, error) {
	// A tspath ends with an archive.  It can be empty here, where a level
	// is expected.
	if len(tspath) == 0 {
		return nil, nil
	}

	lvName := tspath[0]
	child, ok := t.SubLevels[lvName]
	if !ok {
		err := fmt.Errorf("missing level `%s/%s`", prefix, lvName)
		return nil, err
	}
	return child.gatherArchives(tspath[1:], slashpath.Join(prefix, lvName))
}

func (t *LevelTree) gatherArchives(
	tspath []string, prefix string,
) ([]Archive, error) {
	// A tspath must end with an archive.  It cannot be empty here, where a
	// archive is expected.
	if len(tspath) == 0 {
		return nil, errors.New("malformed tspath")
	}

	ts := tspath[0]
	var ars []Archive
	for _, child := range t.Times {
		childName := child.Name()
		ars = append(ars, Archive{
			Path: slashpath.Join(
				prefix,
				childName,
				child.TarType.Path(),
			),
			TarType: child.TarType,
		})
		if childName == ts {
			childBaks, err := child.gatherArchives(
				tspath[1:], slashpath.Join(prefix, childName),
			)
			if err != nil {
				return nil, err
			}
			return append(ars, childBaks...), nil
		}
	}

	return nil, fmt.Errorf("missing archive `%s/%s`", prefix, ts)
}

func loadEncryptedSecret(path string) (string, error) {
	gpgArgs := []string{
		"--batch",
		"--decrypt", path,
	}
	gpgCmd := exec.Command(gpg2Tool.Path, gpgArgs...)
	gpgCmd.Stderr = os.Stderr
	var secret bytes.Buffer
	gpgCmd.Stdout = &secret
	if err := gpgCmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(secret.String()), nil
}

func loadPlaintextSecret(path string) (string, error) {
	secret, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(secret)), nil
}
