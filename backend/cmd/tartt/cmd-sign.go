package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/nogproject/nog/backend/pkg/flock"
)

type SignConfig struct {
	SkipSigned   bool
	SkipGoodFrom string
}

func cmdSign(args map[string]interface{}) {
	cfg := SignConfig{
		SkipSigned: true,
	}
	if args["--no-skip-signed"].(bool) {
		cfg.SkipSigned = false
	}
	if a, ok := args["--skip-good-from"].(string); ok {
		cfg.SkipSigned = false
		cfg.SkipGoodFrom = a
	}

	repo, err := OpenRepo(".")
	if err != nil {
		lg.Fatalw("Failed to open repo.", "err", err)
	}
	defer repo.Close()

	// Open stores only once.  `cache` contains the open stores.
	cache := make(map[string]struct {
		store *Store
		tree  *Tree
	})
	for _, tsp := range args["<tspaths>"].([]string) {
		storeName, p, err := SplitStoreTspath(tsp)
		if err != nil {
			lg.Fatalw("Invalid path.", "tspath", tsp)
		}

		st, ok := cache[storeName]
		if !ok {
			store, err := repo.OpenStore(storeName)
			if err != nil {
				lg.Fatalw(
					"Failed to open store.",
					"store", storeName,
					"err", err,
				)
			}
			defer store.Close()

			tree, err := store.LsTree()
			if err != nil {
				lg.Fatalw(
					"Failed to list tree.",
					"store", storeName,
					"err", err,
				)
			}

			st = struct {
				store *Store
				tree  *Tree
			}{store, tree}
			cache[storeName] = st
		}
		store := st.store
		tree := st.tree

		t := tree.Find(p)
		if t == nil {
			lg.Fatalw("Unknown path.", "tspath", tsp)
		}

		tt, ok := t.(*TimeTree)
		if !ok {
			lg.Fatalw("Not a archive.", "tspath", tsp)
		}

		if err := sign(
			filepath.Join(store.AbsPath(p), tt.TarType.Path()),
			cfg,
		); err != nil {
			lg.Fatalw("Failed to sign.", "tspath", tsp, "err", err)
		}
	}
}

func sign(path string, cfg SignConfig) error {
	manifest := filepath.Join(path, "manifest.shasums")
	sig := fmt.Sprintf("%s.asc", manifest)
	if !exists(manifest) {
		return errors.New("missing manifest")
	}

	if cfg.SkipSigned && exists(sig) {
		lg.Infow("Skipped: existing signature file.", "path", path)
		return nil
	}

	// Lock the manifest to avoid concurrent writes to the signature file.
	lock, err := flock.Open(manifest)
	if err != nil {
		return err
	}
	defer lock.Close()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := lock.TryLock(ctx, 500*time.Millisecond); err != nil {
		cancel()
		return err
	}
	cancel()
	defer lock.Unlock()

	if cfg.SkipGoodFrom != "" && exists(sig) {
		ok, err := gpgCheckGoodFrom(sig, manifest, cfg.SkipGoodFrom)
		if err != nil {
			return err
		}
		if ok {
			lg.Infow("Skipped: good signature from.", "path", path)
			return nil
		}
	}

	fp, err := os.OpenFile(sig, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	err = gpgSign(fp, manifest)
	if err2 := fp.Close(); err == nil {
		err = err2
	}
	// Avoid empty signature files.
	if inf, err := os.Stat(sig); err == nil && inf.Size() == 0 {
		os.Remove(sig)
	}
	if err == nil {
		lg.Infow("Signed.", "path", path)
	}
	return err
}

var rgxGoodSignatureFrom = regexp.MustCompile(
	`^gpg: Good signature from "([^"]+)" \[([^]]+)\]$`,
)

func gpgCheckGoodFrom(sig, file, text string) (bool, error) {
	args := []string{
		"--batch",
		"--verify", sig, file,
	}
	cmd := exec.Command(gpg2Tool.Path, args...)
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf("failed to verify signature: %s", out)
		return false, err
	}

	for _, line := range bytes.Split(out, []byte("\n")) {
		m := rgxGoodSignatureFrom.FindSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		trust := m[2]
		_ = trust
		if bytes.Contains(name, []byte(text)) {
			return true, nil
		}
	}

	return false, nil
}

func gpgSign(out io.Writer, path string) error {
	args := []string{
		"--batch",
		"--detach-sign", "--armor",
		"--output", "-",
		path,
	}
	cmd := exec.Command(gpg2Tool.Path, args...)
	cmd.Stdin = nil
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
