/*

Package `tarquote` converts quoted tar member names to UTF-8 strings.

The package only supports the default quoting style "escape".  See GNU tar
manual section "Quoting Member Names",
<https://www.gnu.org/software/tar/manual/html_section/tar_50.html>

*/
package tarquote

import (
	"errors"
	"strings"
)

var ErrSyntax = errors.New("invalid quoted string")

// `UnquoteEscape()` unquotes the GNU tar quoting style "escape".  It is based
// on the `strconv.Unquote()` source.
func UnquoteEscape(s string) (string, error) {
	// If it's trivial, avoid allocation.
	if strings.IndexByte(s, '\\') < 0 {
		return s, nil
	}

	buf := make([]byte, 0, len(s)) // Avoid further allocations.
	for len(s) > 0 {
		c, ss, err := UnquoteEscapeChar(s)
		if err != nil {
			return "", err
		}
		s = ss
		buf = append(buf, c)
	}
	return string(buf), nil
}

// `UnquoteEscapeChar()` unquotes the leading character of a string that is
// quoted in GNU tar quoting style "escape".  It is based on the
// `strconv.UnquoteChar()` source.
func UnquoteEscapeChar(s string) (head byte, tail string, err error) {
	if len(s) == 0 {
		err = ErrSyntax
		return
	}

	c := s[0]
	if c != '\\' {
		head = c
		tail = s[1:]
		return
	}

	if len(s) <= 1 {
		err = ErrSyntax
		return
	}
	c = s[1]
	s = s[2:]

	switch c {
	case 'a':
		head = '\a'
	case 'b':
		head = '\b'
	case 'f':
		head = '\f'
	case 'n':
		head = '\n'
	case 'r':
		head = '\r'
	case 't':
		head = '\t'
	case 'v':
		head = '\v'
	case '0', '1', '2', '3', '4', '5', '6', '7':
		v := c - '0'
		if len(s) < 2 {
			err = ErrSyntax
			return
		}
		for j := 0; j < 2; j++ { // one digit already; two more
			x := s[j] - '0'
			if x < 0 || x > 7 {
				err = ErrSyntax
				return
			}
			v = (v << 3) | x
		}
		s = s[2:]
		if v > 255 {
			err = ErrSyntax
			return
		}
		head = v
	case '\\':
		head = '\\'
	default:
		err = ErrSyntax
		return
	}
	tail = s
	return
}
