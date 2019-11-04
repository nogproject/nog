package main

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/nogproject/nog/backend/pkg/flock"
)

func cmdLock(args map[string]interface{}) int {
	lockWait := args["--lock-wait"].(time.Duration)

	repo, err := OpenRepo(".")
	if err != nil {
		lg.Fatalw("Failed to open repo.", "err", err)
	}
	defer repo.Close()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()

	// This should be refacored to a `Repo` method if we need it elsewhere.
	lock, err := flock.Open(".")
	if err != nil {
		lg.Fatalw("Failed to open repo flock.", "err", err)
	}
	defer lock.Close()
	if err := lock.TryLock(ctx, 500*time.Millisecond); err != nil {
		lg.Fatalw("Failed to lock repo.", "err", err)
	}
	defer lock.Unlock()

	for _, n := range repo.StoreNames() {
		store, err := repo.OpenStore(n)
		if err != nil {
			lg.Fatalw(
				"Failed to open store.",
				"store", n, "err", err,
			)
		}
		defer store.Close()
		if err := store.TryLock(ctx); err != nil {
			lg.Fatalw(
				"Failed to lock store.",
				"store", n, "err", err,
			)
		}
		defer store.Unlock()
	}

	cmd := args["<cmd>"].([]string)
	err = runCmd(cmd[0], cmd[1:])
	if err != nil {
		exitError, ok := err.(*exec.ExitError)
		if !ok {
			lg.Fatalw("Failed to get <cmd> exit code.", "err", err)
		}
		return exitError.Sys().(syscall.WaitStatus).ExitStatus()
	}
	return 0
}

func runCmd(program string, args []string) error {
	cmd := exec.Command(program, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
