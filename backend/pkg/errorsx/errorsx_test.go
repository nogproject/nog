package errorsx_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/nogproject/nog/backend/pkg/errorsx"
)

var ErrX = errors.New("X")

type error1 struct {
	err error
}

func (err *error1) Error() string {
	return fmt.Sprintf("error 1: %v", err.err)
}

func (err *error1) Unwrap() error {
	return err.err
}

func isError1(err error) bool {
	_, ok := err.(*error1)
	return ok
}

type error2 struct {
	err error
}

func (err *error2) Error() string {
	return fmt.Sprintf("error 2: %v", err.err)
}

func (err *error2) Unwrap() error {
	return err.err
}

func isError2(err error) bool {
	_, ok := err.(*error2)
	return ok
}

type ErrorIface interface {
	ErrorIface()
}

func (err *error1) ErrorIface() {}

func IsErrorIface(err error) bool {
	_, ok := err.(ErrorIface)
	return ok
}

func TestIs(t *testing.T) {
	if !errorsx.Is(nil, nil) {
		t.Errorf("Is(nil, nil) failed.")
	}

	err := io.EOF
	if errorsx.Is(err, nil) {
		t.Errorf("Is(err, nil) failed.")
	}
	if errorsx.Is(err, ErrX) {
		t.Errorf("Is(err, ErrX) failed.")
	}
	if !errorsx.Is(err, io.EOF) {
		t.Errorf("Is(err, io.EOF) failed.")
	}

	err1 := &error1{err: err}
	if !errorsx.Is(err1, io.EOF) {
		t.Errorf("Is(err1, io.EOF) failed.")
	}

	err2 := &error2{err: err1}
	if !errorsx.Is(err2, io.EOF) {
		t.Errorf("Is(err2, io.EOF) failed.")
	}
}

func TestAsEOF(t *testing.T) {
	errx, ok := errorsx.AsPred(io.EOF, func(err error) bool {
		return err == io.EOF
	})
	if !ok || errx != io.EOF {
		t.Errorf("AsPred(io.EOF, ...) failed.")
	}

	errx, ok = errorsx.AsPred(io.EOF, IsErrorIface)
	if ok {
		t.Errorf("AsPred(io.EOF, IsErrorIface) did not return false.")
	}
}

func TestAsUnwrap(t *testing.T) {
	err := io.EOF
	err1 := &error1{err: err}
	err2 := &error2{err: err1}

	errx, ok := errorsx.AsPred(err2, isError2)
	if !ok || errx.(*error2) != err2 {
		t.Errorf("AsPred(..., isError2) failed.")
	}

	errx, ok = errorsx.AsPred(err2, isError1)
	if !ok || errx.(*error1) != err1 {
		t.Errorf("AsPred(..., isError2) failed.")
	}

	errx, ok = errorsx.AsPred(err2, func(err error) bool {
		return err == io.EOF
	})
	if !ok || errx != io.EOF {
		t.Errorf("AsPred(..., err == io.EOF) failed.")
	}

	errx, ok = errorsx.AsPred(err2, IsErrorIface)
	if !ok {
		t.Errorf("AsPred(..., IsErrorIface) failed.")
	}

	errx, ok = errorsx.AsPred(err2, func(err error) bool {
		return false
	})
	if ok {
		t.Errorf("AsPred(wrapped, false) did not return false.")
	}
}
