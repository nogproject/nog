// Package `errorsx` implements error unwrapping and checking inspired by the
// Go 2 draft design document "Error Values — Problem Overview",
// <https://go.googlesource.com/proposal/+/master/design/go2draft-error-values-overview.md>.
//
// The main difference is that, without Go 2 generics, `AsPred()` uses a
// predicate function instead to test the error and returns an `error`, which
// needs to be cast if desired, like so:
//
//     if errx, ok := errorsx.AsPred(err, func (e error) bool {
//         _, ok := e.(*os.PathError)
//         return ok
//     }); ok {
//         errx.(*os.PathError)...
//     }
//
// The official Go experimental package `x/exp/errors`,
// <https://godoc.org/golang.org/x/exp/errors>, uses a different `As()`
// function signature, which takes a pointer to the target error type instead
// of a predicate function.
package errorsx

type Wrapper interface {
	Unwrap() error
}

func Unwrap(err error) error {
	w, ok := err.(Wrapper)
	if !ok {
		return nil
	}
	return w.Unwrap()
}

func Is(err, target error) bool {
	for {
		if err == target {
			return true
		}
		err = Unwrap(err)
		if err == nil {
			return false
		}
	}
}

type Predicate func(error) bool

func IsPred(err error, is Predicate) bool {
	_, ok := AsPred(err, is)
	return ok
}

func AsPred(err error, is Predicate) (error, bool) {
	for {
		if is(err) {
			return err, true
		}
		err = Unwrap(err)
		if err == nil {
			return nil, false
		}
	}
}
