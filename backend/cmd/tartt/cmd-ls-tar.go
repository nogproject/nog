package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/cmd/tartt/drivers"
	"github.com/nogproject/nog/backend/pkg/iox"
	"github.com/nogproject/nog/backend/pkg/ratelimit"
	"github.com/nogproject/nog/backend/pkg/tarquote"
)

type QuoteStyle int

const (
	QsUnspecified QuoteStyle = iota
	QsLiteral
	QsEscaped
)

// XXX A lot of code below is similar to `./cmd-restore.go` and could perhaps
// be refactored to reduce duplication.

func cmdLsTar(args map[string]interface{}) {
	eol := '\n'
	if args["-z"].(bool) {
		eol = 0
	}

	quoteStyle := QsEscaped
	if args["--unquote"].(bool) {
		quoteStyle = QsLiteral
	}

	tspath := args["<tspath>"].(string)

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
	for _, ar := range archives {
		err := lsTar(
			store.AbsPath(ar.Path),
			limit,
			secrets[ar.Path],
			unh, ar.Path,
			quoteStyle, eol,
		)
		if err != nil {
			lg.Fatalw("Listing tar failed.", "err", err)
		}
	}
}

func lsTar(
	archive string,
	limit *ratelimit.Bucket,
	secret string,
	unh drivers.UntarHandler,
	arRel string,
	quoteStyle QuoteStyle,
	eol rune,
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
		"metadata.tar",
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
		"--to-stdout",
		"--file=-", // from `loadCmd`.
		"out.log",
	}
	tarCmd := exec.Command(tarTool.Path, args...)
	if limit == nil {
		tarCmd.Stdin = loadTarPipe.R
	} else {
		tarCmd.Stdin = ratelimit.Reader(loadTarPipe.R, limit)
	}
	tarStdout, err := tarCmd.StdoutPipe()
	if err != nil {
		return err
	}
	tarCmd.Stderr = os.Stderr

	if err := loadCmd.Start(); err != nil {
		return err
	}
	if err := tarCmd.Start(); err != nil {
		_ = loadCmd.Wait()
		return err
	}

	var errPrint error
	tarLines := bufio.NewScanner(tarStdout)
	for tarLines.Scan() {
		path := tarLines.Text()
		switch quoteStyle {
		case QsEscaped:
			// `path` is already escaped.
		case QsLiteral:
			lit, err := tarquote.UnquoteEscape(path)
			if err != nil {
				path = "<INVALID TAR-QUOTED PATH> " + lit
				if errPrint == nil {
					errPrint = err
				}
			} else {
				path = lit
			}
		default:
			panic("invalid quoteStyle")
		}
		fmt.Printf("%s: %s%c", arRel, path, eol)
	}
	if err := tarLines.Err(); errPrint == nil {
		errPrint = err
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
		return fmt.Errorf("untar `out.log` failed: %v", errTar)
	}
	if errPrint != nil {
		return fmt.Errorf("failed to print tar member: %v", errPrint)
	}

	return nil
}
