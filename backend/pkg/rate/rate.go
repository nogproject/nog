package rate

import (
	"context"
	"time"

	"github.com/nogproject/nog/backend/pkg/ratecounter"
	"golang.org/x/time/rate"
)

const (
	StartRateFactor   = 3
	IncreaseSteps     = 100
	DecreaseFactor    = 0.75
	IncreaseThreshold = 0.6
	DecreaseThreshold = 0.1
	RegulateEveryNTau = 3
)

type Config struct {
	Name    string
	MinRate rate.Limit
	MaxRate rate.Limit
	Burst   int
	Tau     time.Duration
}

type Logger interface {
	Infow(msg string, kv ...interface{})
}

type Limiter struct {
	lg   Logger
	name string

	L   *rate.Limiter
	min rate.Limit
	max rate.Limit

	tau          time.Duration
	successCount *ratecounter.RateCounter
	excessCount  *ratecounter.RateCounter
}

func NewLimiter(lg Logger, cfg Config) *Limiter {
	startRate := StartRateFactor * cfg.MinRate
	return &Limiter{
		lg:           lg,
		name:         cfg.Name,
		L:            rate.NewLimiter(startRate, cfg.Burst),
		min:          cfg.MinRate,
		max:          cfg.MaxRate,
		tau:          cfg.Tau,
		successCount: ratecounter.NewRateCounter(cfg.Tau),
		excessCount:  ratecounter.NewRateCounter(cfg.Tau),
	}
}

func (lim *Limiter) Regulate(ctx context.Context) error {
	regulate := func() {
		suc := lim.SuccessRate()
		ex := lim.ExcessRate()
		if ex > DecreaseThreshold*suc {
			r := lim.L.Limit()
			r *= DecreaseFactor
			if r < lim.min {
				r = lim.min
			}
			lim.L.SetLimit(r)
			lim.lg.Infow(
				"Decreased rate limit.",
				"rateLimiter", lim.name,
				"successRate", suc,
				"excessRate", ex,
				"newLimit", r,
			)
		} else if suc > float64(lim.L.Limit())*IncreaseThreshold {
			r := lim.L.Limit()
			r += (lim.max - lim.min) / IncreaseSteps
			if r > lim.max {
				r = lim.max
			}
			lim.L.SetLimit(r)
			lim.lg.Infow(
				"Increased rate limit.",
				"rateLimiter", lim.name,
				"successRate", suc,
				"excessRate", ex,
				"newLimit", r,
			)
		}
	}

	quit := make(chan struct{})
	go func() {
		ticker := time.NewTicker(RegulateEveryNTau * lim.tau)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				close(quit)
				return
			case <-ticker.C:
				regulate()
			}
		}
	}()

	<-ctx.Done()
	<-quit
	return ctx.Err()
}

func (lim *Limiter) Success() {
	lim.successCount.Incr(1)
}

func (lim *Limiter) Excess() {
	lim.excessCount.Incr(1)
}

func (lim *Limiter) SuccessRate() float64 {
	secs := float64(lim.tau) / float64(time.Second)
	return float64(lim.successCount.Rate()) / secs
}

func (lim *Limiter) ExcessRate() float64 {
	secs := float64(lim.tau) / float64(time.Second)
	return float64(lim.excessCount.Rate()) / secs
}
