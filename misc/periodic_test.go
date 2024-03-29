package misc

import (
	"context"
	"io"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestPeriodic_Start(t *testing.T) {
	p := &Periodic{Func: func(context.Context, int, int) error {
		return nil
	}}
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("must not start when Interval is 0")
	}
	p.Interval = 1 * time.Second
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("must not start when MaxInt is 0")
	}
	p.MaxInt = 1
	p.Start(context.Background())
	p.Stop()
	if err := p.WaitForErr(); err != context.Canceled {
		t.Fatalf("unexpected error return: %+v", err)
	}
	p.Stop()
	p.Stop()
}

func TestPeriodic_Restart(t *testing.T) {
	gotRoundNums := make(chan int, 10)
	gotInts := make(chan int, 10)
	p := &Periodic{
		MaxInt:   1,
		Interval: 100 * time.Millisecond,
		Func: func(_ context.Context, round, i int) error {
			gotRoundNums <- round
			gotInts <- i
			return nil
		},
	}
	for startups := 0; startups < 3; startups++ {
		p.Start(context.Background())
		for i := 0; i < 3; i++ {
			if round := <-gotRoundNums; round != i {
				t.Fatalf("unexpected round number: %v", round)
			}
			if anInt := <-gotInts; anInt != 0 {
				t.Fatalf("unexpected int : %v", anInt)
			}
		}
		p.Stop()
		if err := p.WaitForErr(); err != context.Canceled {
			t.Fatalf("unexpected error return: %+v", err)
		}
	}
}

func TestPeriodic_RegularOrder(t *testing.T) {
	funcDone := make(chan struct{}, 1)
	gotRoundNums := make([]int, 0)
	gotInts := make([]int, 0)
	p := &Periodic{
		Interval: 1 * time.Millisecond,
		MaxInt:   3,
		Func: func(_ context.Context, round, i int) error {
			gotRoundNums = append(gotRoundNums, round)
			gotInts = append(gotInts, i)
			if len(gotInts) >= 5 {
				funcDone <- struct{}{}
				return nil
			}
			return nil
		},
	}
	p.Start(context.Background())
	<-funcDone
	p.Stop()
	if err := p.WaitForErr(); err != context.Canceled {
		t.Fatalf("unexpected error return: %+v", err)
	}
	if !reflect.DeepEqual(gotRoundNums, []int{0, 0, 0, 1, 1}) || !reflect.DeepEqual(gotInts, []int{0, 1, 2, 0, 1}) {
		t.Fatalf("Incorrect parameters received: %+v, %+v", gotRoundNums, gotInts)
	}
}

func TestPeriodic_RandomOrder(t *testing.T) {
	rand.Seed(0)
	funcDone := make(chan struct{}, 1)
	gotInts := make([]int, 0)
	gotRoundNums := make([]int, 0)
	p := &Periodic{
		Interval: 1 * time.Millisecond,
		MaxInt:   3,
		Func: func(_ context.Context, round, i int) error {
			gotRoundNums = append(gotRoundNums, round)
			gotInts = append(gotInts, i)
			if len(gotInts) >= 5 {
				funcDone <- struct{}{}
				return nil
			}
			return nil
		},
		RandomOrder: true,
	}
	p.Start(context.Background())
	<-funcDone
	p.Stop()
	if err := p.WaitForErr(); err != context.Canceled {
		t.Fatalf("unexpected error return: %+v", err)
	}
	// This is the expected result for the default PRNG seed.
	if !reflect.DeepEqual(gotRoundNums, []int{0, 0, 0, 1, 1}) || !reflect.DeepEqual(gotInts, []int{1, 0, 2, 1, 0}) {
		t.Fatalf("Incorrect parameters received: %+v, %+v", gotRoundNums, gotInts)
	}
}

func TestPeriodic_RapidFirstRound(t *testing.T) {
	funcDone := make(chan struct{}, 1)
	gotInts := make([]int, 0)
	gotRoundNums := make([]int, 0)
	p := &Periodic{
		Interval: 1 * time.Second,
		MaxInt:   2,
		Func: func(_ context.Context, round, i int) error {
			gotRoundNums = append(gotRoundNums, round)
			gotInts = append(gotInts, i)
			if len(gotInts) >= 4 {
				funcDone <- struct{}{}
				return nil
			}
			return nil
		},
		RapidFirstRound: true,
	}
	startTime := time.Now()
	p.Start(context.Background())
	<-funcDone
	p.Stop()
	if err := p.WaitForErr(); err != context.Canceled {
		t.Fatalf("unexpected error return: %+v", err)
	}
	duration := time.Now().Sub(startTime)
	// 0, zero interval, 1, zero interval, 0, one-sec interval, 1, stop.
	if !reflect.DeepEqual(gotRoundNums, []int{0, 0, 1, 1}) || !reflect.DeepEqual(gotInts, []int{0, 1, 0, 1}) {
		t.Fatalf("Incorrect parameters received: %+v, %+v", gotRoundNums, gotInts)
	}
	if duration < 1*time.Second || duration > 2*time.Second {
		t.Fatalf("Duration seems wrong: %+v", duration)
	}
}

func TestPeriodic_StableInterval(t *testing.T) {
	funcDone := make(chan struct{}, 1)
	gotInts := make([]int, 0)
	gotRoundNums := make([]int, 0)
	p := &Periodic{
		Interval: 1 * time.Second,
		MaxInt:   2,
		Func: func(_ context.Context, round, i int) error {
			gotRoundNums = append(gotRoundNums, round)
			gotInts = append(gotInts, i)
			if len(gotInts) >= 4 {
				funcDone <- struct{}{}
				return nil
			}
			time.Sleep(500 * time.Millisecond)
			return nil
		},
		StableInterval: true,
	}
	startTime := time.Now()
	p.Start(context.Background())
	<-funcDone
	p.Stop()
	if err := p.WaitForErr(); err != context.Canceled {
		t.Fatalf("unexpected error return: %+v", err)
	}
	duration := time.Now().Sub(startTime)
	// 0, one-sec interval, 1, one-sec interval, 0, one-sec interval, 1, stop.
	if !reflect.DeepEqual(gotRoundNums, []int{0, 0, 1, 1}) || !reflect.DeepEqual(gotInts, []int{0, 1, 0, 1}) {
		t.Fatalf("Incorrect parameters received: %+v, %+v", gotRoundNums, gotInts)
	}
	if duration < 3*time.Second || duration > 4*time.Second {
		t.Fatalf("Duration seems wrong: %+v", duration)
	}
}

func TestPeriodic_StableIntervalWithOverrun(t *testing.T) {
	funcDone := make(chan struct{}, 1)
	gotInts := make([]int, 0)
	gotRoundNums := make([]int, 0)
	p := &Periodic{
		Interval: 1 * time.Second,
		MaxInt:   2,
		Func: func(_ context.Context, round, i int) error {
			gotRoundNums = append(gotRoundNums, round)
			gotInts = append(gotInts, i)
			if len(gotInts) >= 4 {
				funcDone <- struct{}{}
				return nil
			}
			time.Sleep(1500 * time.Millisecond)
			return nil
		},
		StableInterval: true,
	}
	startTime := time.Now()
	p.Start(context.Background())
	<-funcDone
	p.Stop()
	if err := p.WaitForErr(); err != context.Canceled {
		t.Fatalf("unexpected error return: %+v", err)
	}
	duration := time.Now().Sub(startTime)
	// 0, half-sec interval, 1, half-sec interval, 0, half-sec interval, 1, stop.
	if !reflect.DeepEqual(gotRoundNums, []int{0, 0, 1, 1}) || !reflect.DeepEqual(gotInts, []int{0, 1, 0, 1}) {
		t.Fatalf("Incorrect parameters received: %+v, %+v", gotRoundNums, gotInts)
	}
	if duration < 6*time.Second || duration > 7*time.Second {
		t.Fatalf("Duration seems wrong: %+v", duration)
	}
}

func TestPeriodic_WaitForErr(t *testing.T) {
	p := &Periodic{
		Interval: 1 * time.Second,
		MaxInt:   2,
		Func: func(_ context.Context, round, i int) error {
			return io.EOF
		},
		StableInterval: true,
	}
	p.Start(context.Background())
	for i := 0; i < 2; i++ {
		if err := p.WaitForErr(); err != io.EOF {
			t.Fatalf("unexpected error return: %+v", err)
		}
	}
	p.Stop()
	for i := 0; i < 2; i++ {
		if err := p.WaitForErr(); err != io.EOF {
			t.Fatalf("unexpected error return: %+v", err)
		}
	}
}
