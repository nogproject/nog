package main

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type Processor interface {
	Process(ctx context.Context) error
}

func StartScans(
	lg Logger,
	wg *sync.WaitGroup,
	ctx context.Context,
	what string,
	procs []Processor,
	argStart, argEvery, argJitter interface{},
) {
	start, startYes := argStart.(time.Duration)
	if startYes && start == 0 {
		startYes = false
	}
	every, everyYes := argEvery.(time.Duration)
	if everyYes && every == 0 {
		everyYes = false
	}
	jitter, _ := argJitter.(time.Duration)
	switch {
	case startYes && everyYes:
		lg.Infow(
			"Enabled initial and regular scans.",
			"what", what,
			"start", start,
			"every", every,
			"jitter", jitter,
		)
	case startYes:
		lg.Infow(
			"Enabled initial scan.",
			"what", what,
			"start", start,
			"jitter", jitter,
		)
	case everyYes:
		lg.Infow(
			"Enabled regular scans.",
			"what", what,
			"every", every,
			"jitter", jitter,
		)
	default:
		lg.Infow(
			"Disabled scans.",
			"what", what,
		)
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		foreverScan(lg, ctx, what, procs, start, every, jitter)
	}()
}

func foreverScan(
	lg Logger,
	ctx context.Context,
	what string,
	procs []Processor,
	scanStart, scanEvery, jitter time.Duration,
) {
	procAll := func() error {
		for _, p := range procs {
			if err := p.Process(ctx); err != nil {
				return err
			}
		}
		return nil
	}

	jitterSleep := func() error {
		if jitter <= 0 {
			return nil
		}
		d := time.Duration(rand.Int63n(int64(jitter)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.NewTimer(d).C:
			return nil
		}
	}

	if scanStart != 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTimer(scanStart).C:
		}

		if err := jitterSleep(); err != nil {
			return
		}

		lg.Infow(
			"Started initial scan.",
			"what", what,
		)
		err := procAll()
		switch {
		case err == context.Canceled:
			return
		case err != nil:
			lg.Errorw(
				"Initial scan failed.",
				"what", what,
				"err", err,
			)
		default:
			lg.Infow(
				"Completed initial scan.",
				"what", what,
			)
		}
	}

	if scanEvery == 0 {
		return
	}

	ticker := time.NewTicker(scanEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := jitterSleep(); err != nil {
				return
			}

			lg.Infow(
				"Started regular scan.",
				"what", what,
			)
			err := procAll()
			switch {
			case err == context.Canceled:
				return
			case err != nil:
				lg.Errorw(
					"Regular scan failed.",
					"what", what,
					"err", err,
				)
			default:
				lg.Infow(
					"Completed regular scan.",
					"what", what,
				)
			}
		}
	}
}
