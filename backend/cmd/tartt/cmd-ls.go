package main

import (
	"context"
	"fmt"
	"time"
)

type LsOptions struct {
	Lock bool
}

func cmdLs(args map[string]interface{}) {
	opts := LsOptions{
		Lock: !args["--no-lock"].(bool),
	}

	repo, err := OpenRepo(".")
	if err != nil {
		lg.Fatalw("Failed to open repo.", "err", err)
	}
	defer repo.Close()

	for _, n := range repo.StoreNames() {
		lsStore(repo, n, opts)
	}
}

func lsStore(repo *Repo, storeName string, opts LsOptions) {
	store, err := repo.OpenStore(storeName)
	if err != nil {
		lg.Fatalw("Failed to open store.", "err", err)
	}
	defer store.Close()

	// Listing without lock MUST be safe with concurrent operations that
	// only append to the store, like archive.  It MAY be unsafe with
	// concurrent operations that may delete content, like gc.  If in
	// doubt, document the behavior for individual operations.
	if opts.Lock {
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

	if err := store.WalkTree(tree, func(inf TreeInfo) error {
		fmt.Printf("%s", inf.LifeCycle.Letter())
		switch x := inf.Node.(type) {
		case *LevelTree:
			fmt.Printf(" %d", len(x.Times))
			fmt.Printf(" %5s", x.Level().Name())
		case *TimeTree:
			fmt.Printf(" %d", len(x.SubLevels))
			fmt.Printf(" %5s", x.TarType.Path())
		}
		fmt.Printf(
			"  %s %s",
			inf.Node.MinTime().Format(time.RFC3339),
			inf.Node.MaxTime().Format(time.RFC3339),
		)
		if inf.Path == "" {
			fmt.Printf("\t%s\n", storeName)
		} else {
			fmt.Printf("\t%s/%s\n", storeName, inf.Path)
		}
		return nil
	}); err != nil {
		lg.Fatalw("ls failed.", "err", err)
	}
}
