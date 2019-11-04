// Package `regexpx` contains functions that complement the standard package
// `regexp`.
package regexpx

import (
	"strings"
	"unicode"
)

/*

`Verbose(verboseRegex)` returns a normal regex that can be compiled with the
`regexp` package function.  Example:

    regexp.MustCompile(regexpx.Verbose(`
        ^
	( [^<] + )
	\s
	< ( [^>]+ ) >
	$
    `)

*/
func Verbose(s string) string {
	return removeWhitespace(s)
}

func removeWhitespace(s string) string {
	dropSpace := func(c rune) rune {
		if unicode.IsSpace(c) {
			return -1
		}
		return c
	}
	return strings.Map(dropSpace, s)
}
