package main

import (
	"context"
	"os"
	slashpath "path"
	"path/filepath"
	"time"
)

type GcOptions struct {
	DryRun   bool
	LockWait time.Duration
}

func cmdGc(args map[string]interface{}) {
	opts := GcOptions{
		DryRun:   args["--dry-run"].(bool),
		LockWait: args["--lock-wait"].(time.Duration),
	}

	repo, err := OpenRepo(".")
	if err != nil {
		lg.Fatalw("Failed to open repo.", "err", err)
	}
	defer repo.Close()

	for _, n := range repo.StoreNames() {
		gcStore(repo, n, opts)
	}
}

func gcStore(repo *Repo, storeName string, opts GcOptions) {
	store, err := repo.OpenStore(storeName)
	if err != nil {
		lg.Fatalw("Failed to open store.", "err", err)
	}
	defer store.Close()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, opts.LockWait)
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

	gcStale(store, opts)
	gcGood(store, opts)
}

func gcStale(store *Store, opts GcOptions) {
	now := time.Now().UTC()
	// XXX The lifetime of stale tars would better be configurable.
	lifetimeStale, err := ParseDateTimeDuration("5 days")
	if err != nil {
		panic(err)
	}

	gch := store.GcHandler()
	gcIfStale := func(inf TreeInfo) error {
		t := inf.Node.(*TimeTree)
		// Consider gc only for incomplete tars.
		switch t.TarType {
		case TarFullInProgress:
		case TarFullError:
		case TarPatchInProgress:
		case TarPatchError:
		default:
			return nil
		}

		tar := t.TarType.Path()
		tmax := t.MaxTime()
		if lifetimeStale.AddTime(tmax).Before(now) {
			if opts.DryRun {
				lg.Warnw(
					"Would remove stale tar.",
					"maxTime", tmax.Format(time.RFC3339),
					"path", storePath(store, inf.Path),
					"tar", tar,
				)
			} else {
				if err := gch.RemoveAll(inf.Path); err != nil {
					return err
				}

				abspath := store.AbsPath(inf.Path)
				if err := os.RemoveAll(abspath); err != nil {
					return err
				}
				lg.Infow(
					"Removed stale tar.",
					"maxTime", tmax.Format(time.RFC3339),
					"path", storePath(store, inf.Path),
					"tar", tar,
				)
			}
			return SkipTree
		} else {
			lg.Infow(
				"Kept recent incomplete tar.",
				"maxTime", tmax.Format(time.RFC3339),
				"path", storePath(store, inf.Path),
				"tar", tar,
			)
		}
		return nil
	}

	// List all nodes and select incomplete tars in `gcIfStale()`.
	tree, err := store.LsTreeSelect(TarTypesAll)
	if err != nil {
		lg.Fatalw("Failed to list stale tars.", "err", err)
	}
	if err := store.WalkTree(tree, func(inf TreeInfo) error {
		if inf.Path == "" {
			return nil // Enter root.
		}
		switch inf.Node.(type) {
		case *LevelTree:
			return nil
		case *TimeTree:
			return gcIfStale(inf)
		default:
			panic("invalid tree node")
		}
	}); err != nil {
		lg.Fatalw("gc failed.", "err", err)
	}
}

func gcGood(store *Store, opts GcOptions) {
	tree, err := store.LsTree()
	if err != nil {
		lg.Fatalw("Failed to list tree.", "err", err)
	}

	now := time.Now().UTC()
	latest := tree.MaxTime()
	lg.Infow(
		"Time of the latest archive, which will be kept.",
		"store", store.Name,
		"latest", latest.Format(time.RFC3339),
	)

	gch := store.GcHandler()
	gcIfExpired := func(inf TreeInfo) error {
		t := inf.Node
		tmax := t.MaxTime()
		if t.Level().lifetime.AddTime(tmax).Before(now) {
			if opts.DryRun {
				lg.Warnw(
					"Would remove level.",
					"maxTime", tmax.Format(time.RFC3339),
					"path", storePath(store, inf.Path),
				)
			} else {
				if err := gch.RemoveAll(inf.Path); err != nil {
					return err
				}

				abspath := store.AbsPath(inf.Path)
				if err := os.RemoveAll(abspath); err != nil {
					return err
				}
				lg.Infow(
					"Removed level.",
					"maxTime", tmax.Format(time.RFC3339),
					"path", storePath(store, inf.Path),
				)
			}
			return SkipTree
		}
		return nil
	}

	gcLevel := func(inf TreeInfo) error {
		if inf.Node.MaxTime() == latest {
			lg.Infow(
				"Kept level that contains latest.",
				"path", storePath(store, inf.Path),
			)
			return nil
		}

		if err := gcIfExpired(inf); err != nil {
			return err
		}

		return nil
	}

	cleanFrozenArchive := func(inf TreeInfo) error {
		dir := filepath.Join(
			filepath.FromSlash(inf.Path),
			inf.Node.(*TimeTree).TarType.Path(),
		)
		for _, file := range []string{
			"origin.snar",
		} {
			relpath := filepath.Join(dir, file)
			abspath := store.AbsPath(relpath)
			if !exists(abspath) {
				continue
			}
			if opts.DryRun {
				lg.Warnw(
					"Would remove detail from frozen archive.",
					"path", relpath,
				)
			} else {
				if err := os.Remove(abspath); err != nil {
					return err
				}
				lg.Infow(
					"Removed detail from frozen archive.",
					"path", relpath,
				)
			}
		}
		return nil
	}

	gcTime := func(inf TreeInfo) error {
		t := inf.Node
		if store.IsRootLevel(t.Level()) {
			if t.MaxTime() == latest {
				lg.Infow(
					"Kept full archive that contains latest.",
					"path", storePath(store, inf.Path),
				)
			} else if err := gcIfExpired(inf); err != nil {
				return err
			}
		}

		if inf.LifeCycle == LcFrozen {
			if err := cleanFrozenArchive(inf); err != nil {
				return err
			}
		}

		return nil
	}

	if err := store.WalkTree(tree, func(inf TreeInfo) error {
		if inf.Path == "" {
			return nil // Enter root.
		}
		switch inf.Node.(type) {
		case *LevelTree:
			return gcLevel(inf)
		case *TimeTree:
			return gcTime(inf)
		default:
			panic("invalid tree node")
		}
	}); err != nil {
		lg.Fatalw("gc failed.", "err", err)
	}
}

func storePath(store *Store, p string) string {
	return slashpath.Join(store.Name, p)
}
