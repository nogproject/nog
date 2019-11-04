package parse

import (
	"fmt"
	"regexp"

	"github.com/nogproject/nog/backend/pkg/regexpx"
)

// Example:
// `A U Thor <author@example.com>` -> (`A U Thor`, `author@example.com`).
var rgxUser = regexp.MustCompile(regexpx.Verbose(`
	^
	( [^<]+ )
	\s
	< ( [^>]+ ) >
	$
`))

func User(user string) (name, email string, err error) {
	m := rgxUser.FindStringSubmatch(user)
	if m == nil {
		err := fmt.Errorf("does not match `%s`", rgxUser)
		return "", "", err
	}
	return m[1], m[2], nil
}
