// Package `tartt/drivers` contains the interfaces that allow customizing,
// through a store driver, how tartt stores data.
//
// There is little abstraction compared to the default driver, which stores all
// tar files locally in the tartt repo tree by running `tartt-store`.  Drivers
// can change the program and the program arguments that `tartt` executes to
// store data.
//
// If we wanted to support other storage options, like writing to S3, we should
// consider raising the level of abstraction.
package drivers

// `StoreDriver` is the entry point for `tartt.Repo.OpenStore()`.
type StoreDriver interface {
	// `Open()` is called during `tartt.Repo.OpenStore()`.  A driver can
	// return a handle object to manage stateful resources that are
	// released during `StoreHandle.Close()`.
	//
	// A driver that does not need to maintain state for each `Open()` can
	// return itself and implement the `StoreHandle` methods on itself,
	// including an empty `Close()` method.
	Open() (StoreHandle, error)
}

// `StoreHandle` contains the operations on an open store.  It may store state
// that should be released in `Close()`, which is called from
// `tartt.Store.Close()`.
type StoreHandle interface {
	ArchiveHandler
	GcHandler
	UntarHandler
	Close() error
}

// `ArchiveHandler` contains the operations used by `tartt tar`.
type ArchiveHandler interface {
	// `BeginArchive()` initiates a new archive and returns an `ArchiveTx`
	// handle to manage its completion.  `dst` is the relative path within
	// the store.  `tmp` is the local `.inprogress/` directory, which will
	// contain a complete local archive when `tartt` calls
	// `ArchiveTx.Commit()`.  `tmp` may be an absolute path.
	//
	// `tartt` either calls `ArchiveTx.Commit()` or `ArchiveTx.Abort()`,
	// unless it is terminated by a signal.  The driver must complete the
	// new archive at its final location in `ArchiveTx.Commit()` or discard
	// any allocated resources during `ArchiveTx.Abort()`.  If `Commit()`
	// returns an error, `tartt` will call `Abort()`.
	//
	// The naming convention must allow removing multiple archives that
	// have a common prefix.  See `GcHandler.RemoveAll()`.
	//
	// The driver should implement a naming convention that allows
	// detecting and removing stale inprogress archives, which may result
	// from terminating tartt early.  The naming convention must ensure
	// that inprogress archives are removed when removing a prefix.
	// Example: `GcHandler.RemoveAll("2018-06-21T123235Z")` removes:
	//
	// ```
	// 2018-06-21T123235Z/full.inprogress
	// ```
	//
	BeginArchive(dst, tmp string) (ArchiveTx, error)
}

// `GcHandler` contains the operations used by `tartt gc`.
type GcHandler interface {
	// `RemoveAll()` removes all archives below a common `prefix`, which
	// can be a level path, such as `2018-06-21T123238Z/s0`, or a time
	// path, such as `2018-06-21T123238Z/s0/2018-06-21T231122Z`, including
	// time paths to the first level, such as `2018-06-21T123238Z`.
	RemoveAll(prefix string) error
}

// `UntarHandler` contains the operations used by `tartt restore`.
type UntarHandler interface {
	// `LoadProgram()` and `LoadArgs()` are similar to
	// `ArchiveTx.SaveProgram()` and `ArchiveTx.SaveArgs()`.
	//
	// `LoadArgs()`, however, has an additional argument `arRel` that
	// contains the path to the archive relative to the store, which is the
	// same as the argument `dst` of `BeginArchive()` during save
	// operations.
	LoadProgram(tarttStore string) string
	LoadArgs(arRel string, origArgs []string) []string
}

// `ArchiveTx` represents an archive operation that has been started with
// `ArchiveHandler.BeginArchive()` and not yet completed.
type ArchiveTx interface {
	// `Commit()` completes the archive, usually by moving it to the final
	// location and maybe copying additional files.
	Commit() error

	// `Abort()` discards an incomplete archive.
	Abort() error

	// `SaveProgram()` returns the program that saves data streams.  It is
	// usually `tartt-store`, whose full path is passed to the method as
	// `tarttStore`.  The program is executed in the local inprogress
	// directory.
	//
	// `SaveArgs()` returns the command arguments for `SaveProgram()`.
	// `origArgs` contains the default `tartt-store` arguments.
	//
	// Example driver strategies:
	//
	//  - Change the arguments to store tar files immediately into remote
	//    storage without local duplicate.
	//  - Tell the save program to store small files locally and copy them
	//    to remote storage during `Commit()`, for example `README.md` and
	//    `manifest.shasums`.
	//  - Change the args to immediately store a local and remote copy of
	//    small files.
	//
	SaveProgram(tarttStore string) string
	SaveArgs(origArgs []string) []string
}
