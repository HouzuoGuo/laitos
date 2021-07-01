package misc

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

// Periodic invokes a function continuously with a regular interval in between.
type Periodic struct {
	// LogActorName is a string used in log messages. These messages are rare.
	LogActorName string
	// Interval between each invocation.
	Interval time.Duration
	// MaxInt determines the upper bound range of the integer value given
	// to the invoked function.
	// If the function does not use this integer, then set this field to 1.
	MaxInt int
	// Func is the function to be invoked at regular interval. With each
	// invocation, the function will receive:
	// - A context, which may be cancelled.
	// - Current round number starting with 0. A round is completed after the
	//   function has been invoked with all of [0,MaxInt).
	// - An integer in the range of [0,MaxInt).
	// If the function returns a non-nil error, then the periodic invocation
	// will stop entirely. The function's error can be retrieved by calling
	// WaitForErr function.
	Func func(context.Context, int, int) error
	// RandomInt determines whether each invocation of the function will receive
	// a randomly chosen integer within the range of [0,MaxInt).
	// Otherwise, each consecutive invocation will receive the next integer in
	// sequence, which wraps around after reaching MaxInt-1.
	// Caller of Start function should seed the default PRNG.
	RandomOrder bool
	// StableInterval lines up the start of each invocation of the function to
	// the interval.
	// Otherwise, the interval between consecutive invocations of the function
	// will always be the fixed duration.
	StableInterval bool
	// RapidFirstRound causes the first round of invocations (that is Func
	// invoked with all of [0,MaxInt)) to be invoked in rapid succession, without
	// having to wait for the interval in between.
	RapidFirstRound bool

	cancelFunc  func()
	funcErrChan chan error
	funcErr     error
}

// Start invoking the periodic function continuously at regular interval.
// The function does not block caller.
// Optionally, use WaitForErr to block-wait for the periodic function to return
// an error, in which case the periodic invocations will stop entirely.
func (p *Periodic) Start(ctx context.Context) error {
	if p.Interval == 0 {
		return errors.New("Interval must be greater than 0")
	}
	if p.MaxInt < 1 {
		return errors.New("MaxInt must be greater than 0")
	}
	funcInputInts := make([]int, p.MaxInt)
	for i := 0; i < p.MaxInt; i++ {
		funcInputInts[i] = i
	}
	if p.RandomOrder {
		rand.Shuffle(len(funcInputInts), func(a, b int) {
			funcInputInts[a], funcInputInts[b] = funcInputInts[b], funcInputInts[a]
		})
	}
	ctx, cancelFunc := context.WithCancel(ctx)
	p.cancelFunc = cancelFunc
	p.funcErrChan = make(chan error, 1)
	p.funcErr = nil
	go func() {
		for roundNum := 0; ; roundNum++ {
			for _, anInt := range funcInputInts {
				if EmergencyLockDown {
					lalog.DefaultLogger.Warning("Periodic.Start", p.LogActorName, ErrEmergencyLockDown, "stop immediately")
				}
				startTime := time.Now()
				if err := p.Func(ctx, roundNum, anInt); err != nil {
					p.funcErrChan <- err
					return
				}
				runDuration := time.Now().Sub(startTime)
				// Calculate the interval to wait
				waitInterval := p.Interval
				if p.StableInterval {
					// Handle overrun (the previous invocation took longer than
					// the regular interval) by waiting for the next interval
					// to start.
					if runDuration > waitInterval {
						waitInterval -= runDuration % waitInterval
					} else {
						waitInterval -= runDuration
					}
				}
				if p.RapidFirstRound && roundNum == 0 {
					waitInterval = 0
				}
				select {
				case <-time.After(waitInterval):
				case <-ctx.Done():
					p.funcErrChan <- ctx.Err()
					return
				}
			}
		}
	}()
	return nil
}

// Wait for the periodically invoked function or its context to return an error,
// and then return the error to the caller. The function blocks caller.
func (p *Periodic) WaitForErr() error {
	if p.funcErr == nil {
		err := <-p.funcErrChan
		p.funcErr = err
	}
	return p.funcErr
}

// Stop the periodic invocation of the function. The result of the final
// invocation can be discovered from the return value of Wait function.
func (p *Periodic) Stop() {
	p.cancelFunc()
}
