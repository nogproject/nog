// Package `ratecounter` is the subset of `paulbellamy/ratecounter` that we
// use.  See <https://godoc.org/github.com/paulbellamy/ratecounter>.
package ratecounter

import (
	"time"

	"github.com/paulbellamy/ratecounter"
)

type RateCounter = ratecounter.RateCounter

func NewRateCounter(interval time.Duration) *RateCounter {
	return ratecounter.NewRateCounter(interval)
}
