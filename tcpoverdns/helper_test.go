package tcpoverdns

import (
	"context"
	"io"
	"os"
	"os/signal"
	"reflect"
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

func checkTC(t *testing.T, tc *TransmissionControl, timeoutSec int, wantState State, wantInputSeq, wantInputAck, wantOutputSeq int, wantInputBuf, wantOutputBuf []byte) {
	t.Helper()
	for i := 0; i < timeoutSec*10; i++ {
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()
		if !reflect.DeepEqual(instant.state, wantState) {
			goto retry
		} else if int(instant.inputSeq) != wantInputSeq {
			goto retry
		} else if int(instant.inputAck) != wantInputAck {
			goto retry
		} else if int(instant.outputSeq) != wantOutputSeq {
			goto retry
		} else if (instant.inputBuf != nil && wantInputBuf != nil) && !reflect.DeepEqual(instant.inputBuf, wantInputBuf) {
			goto retry
		} else if (instant.outputBuf != nil && wantOutputBuf != nil) && !reflect.DeepEqual(instant.outputBuf, wantOutputBuf) {
			goto retry
		} else {
			return
		}
	retry:
		time.Sleep(100 * time.Millisecond)
	}
	tc.DumpState()
	t.Fatalf("want state: %d, input seq: %d, input ack: %d, output seq: %d, input buf: %v, output buf: %v",
		wantState, wantInputSeq, wantInputAck, wantOutputSeq, wantInputBuf, wantOutputBuf)
}

func checkTCError(t *testing.T, tc *TransmissionControl, timeoutSec int, wantOngoingTransmission, wantInputTransportErrors, wantOutputTransportErrors int) {
	t.Helper()
	for i := 0; i < timeoutSec*10; i++ {
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()
		if instant.ongoingRetransmissions != wantOngoingTransmission {
			goto retry
		} else if instant.inputTransportErrors != wantInputTransportErrors {
			goto retry
		} else if instant.outputTransportErrors != wantOutputTransportErrors {
			goto retry
		} else {
			return
		}
	retry:
		time.Sleep(100 * time.Millisecond)
	}
	tc.DumpState()
	t.Fatalf("want ongiong retrans: %d, input transport errs: %d, output transport errs: %d",
		wantOngoingTransmission, wantInputTransportErrors, wantOutputTransportErrors)
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
