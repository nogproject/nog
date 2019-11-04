package tarquote_test

import (
	"testing"

	"github.com/nogproject/nog/backend/pkg/tarquote"
)

func TestUnquoteChar(t *testing.T) {
	for _, spec := range []struct {
		s    string
		head byte
		tail string
		err  error
	}{
		{"abc", 'a', "bc", nil},
		{"\\abc", '\a', "bc", nil},
		{"\\bbc", '\b', "bc", nil},
		{"\\fbc", '\f', "bc", nil},
		{"\\nbc", '\n', "bc", nil},
		{"\\rbc", '\r', "bc", nil},
		{"\\tbc", '\t', "bc", nil},
		{"\\vbc", '\v', "bc", nil},
		{"\\\\bc", '\\', "bc", nil},
		{"\\", 0, "", tarquote.ErrSyntax},
		{"\\xbc", 0, "", tarquote.ErrSyntax},
		// ä in UTF-8 = two bytes octal 0303 0244.
		{"\\303\\244bc", '\303', "\\244bc", nil},
	} {
		head, tail, err := tarquote.UnquoteEscapeChar(spec.s)
		if head != spec.head {
			t.Errorf(
				"Case '%s': wrong unquoted head: "+
					"expected '%s', got '%s'.",
				spec.s,
				string(spec.head), string(head),
			)
		}
		if tail != spec.tail {
			t.Errorf(
				"Case '%s': wrong tail: "+
					"expected '%s', got '%s'.",
				spec.s,
				string(spec.tail), string(tail),
			)
		}
		if err != spec.err {
			t.Errorf(
				"Case '%s': unexpected error: "+
					"expected '%v', got '%v'.",
				spec.s,
				spec.err, err,
			)
		}
	}
}

func TestUnquote(t *testing.T) {
	for _, spec := range []struct {
		s   string
		un  string
		err error
	}{
		{"abc", "abc", nil},
		{"\\abc", "\abc", nil},
		{"\\bbc", "\bbc", nil},
		{"\\fbc", "\fbc", nil},
		{"\\nbc", "\nbc", nil},
		{"\\rbc", "\rbc", nil},
		{"\\tbc", "\tbc", nil},
		{"\\vbc", "\vbc", nil},
		{"\\\\bc", "\\bc", nil},
		{"\\", "", tarquote.ErrSyntax},
		{"\\xbc", "", tarquote.ErrSyntax},
		// ä in UTF-8 = two bytes octal 0303 0244.
		{"\\303\\244bc", "äbc", nil},
	} {
		un, err := tarquote.UnquoteEscape(spec.s)
		if un != spec.un {
			t.Errorf(
				"Case '%s': wrong unquoted string: "+
					"expected '%s', got '%s'.",
				spec.s,
				string(spec.un), string(un),
			)
		}
		if err != spec.err {
			t.Errorf(
				"Case '%s': unexpected error: "+
					"expected '%v', got '%v'.",
				spec.s,
				spec.err, err,
			)
		}
	}
}
