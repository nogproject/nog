package rules

type Finder interface {
	Find(
		hostRoot string,
		known map[string]bool,
		fns FindHandlerFuncs,
	) error
}

type FindHandlerFuncs struct {
	CandidateFn func(relpath string) error
	IgnoreFn    func(relpath string) error
	KnownFn     func(relpath string) error
}
