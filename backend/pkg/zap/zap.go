/*

Package `zap` wraps Zap logging.

Zap has been chosen after a quick review of the logging solutions listed on
Awesome Go.  Zap was among the top 5 on GitHub.  Its performance is impressive.
Its API is similar to the stdlib `log` package.  It also has a convenient
structured logging api of `Levelw(msg, kv ...)` functions, which we usually
use.

*/
package zap

import (
	"go.uber.org/zap"
)

// We use the convenience sugared logger `Levelw(msg, kv...)` functions.
type Logger = zap.SugaredLogger

func NewProduction() (*Logger, error) {
	l, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return l.Sugar(), nil
}

func NewDevelopment() (*Logger, error) {
	l, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return l.Sugar(), nil
}
