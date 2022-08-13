package tcpoverdns

import (
	"context"
	"io"
	"os"
	"os/signal"
	runtimePprof "runtime/pprof"
	"testing"
	"time"
)

func dumpGoroutinesOnInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			_ = runtimePprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
		}
	}()
}

func readSegment(t *testing.T, reader io.Reader, dataLen int) Segment {
	t.Helper()
	segData, err := readInput(context.Background(), reader, SegmentHeaderLen+dataLen)
	if err != nil {
		t.Fatalf("readSegment err: %+v", err)
	}
	if len(segData) != SegmentHeaderLen+dataLen {
		t.Fatalf("readSegment unexpected segData length: %v, want: %v", len(segData), dataLen)
	}
	return SegmentFromPacket(segData)
}

func waitForOutputSeq(t *testing.T, tc *TransmissionControl, timeoutSec int, want int) {
	t.Helper()
	for i := 0; i < timeoutSec*10; i++ {
		tc.mutex.Lock()
		got := tc.outputSeq
		tc.mutex.Unlock()
		if int(got) == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("got tc output seq %d, want seq %v", tc.outputSeq, want)
}

func waitForInputAck(t *testing.T, tc *TransmissionControl, timeoutSec int, want int) {
	t.Helper()
	for i := 0; i < timeoutSec*10; i++ {
		tc.mutex.Lock()
		got := tc.inputAck
		tc.mutex.Unlock()
		if int(got) == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("got tc input ack %d, want seq %v", tc.inputAck, want)
}

func waitForState(t *testing.T, tc *TransmissionControl, timeoutSec int, wantState State) {
	t.Helper()
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	if !tc.WaitState(timeoutCtx, wantState) {
		t.Fatalf("got tc state %d, want state %v", tc.state, wantState)
	}
}
