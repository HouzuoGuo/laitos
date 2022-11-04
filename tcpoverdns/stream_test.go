package tcpoverdns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestTransmissionControl_InboundSegments_ReadNothing(t *testing.T) {
	_, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:           true,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
		InitialTiming: TimingConfig{
			ReadTimeout: 3 * time.Second,
		},
		state: StateEstablished,
	}
	tc.Start(context.Background())
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != os.ErrDeadlineExceeded {
		t.Fatalf("read n: %+v, err: %+v", n, err)
	}
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_Closed(t *testing.T) {
	_, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		state:           StateEstablished,
		Debug:           true,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
		MaxLifetime:     1 * time.Second,
	}
	tc.Start(context.Background())
	// Wait for TC to close.
	CheckTC(t, tc, 2, StateClosed, 0, 0, 0, nil, nil)
	n, err := tc.Read(nil)
	if n != 0 || err != io.EOF {
		t.Fatalf("read n: %+v, err: %+v", n, err)
	}
	n, err = tc.Write(nil)
	if n != 0 || err != io.EOF {
		t.Fatalf("read n: %+v, err: %+v", n, err)
	}
	CheckTC(t, tc, 5, StateClosed, 0, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_CloseAfterDrained(t *testing.T) {
	_, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:           true,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
		state:           StateEstablished,
	}
	tc.Start(context.Background())
	tc.CloseAfterDrained()
	// Wait for TC to close.
	CheckTC(t, tc, 5, StateClosed, 0, 0, 0, nil, nil)
}

func TestTransmissionControl_InboundSegments_ReadEach(t *testing.T) {
	testIn, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		MaxSlidingWindow:        10,
		InitialTiming: TimingConfig{
			ReadTimeout: 3 * time.Second,
		},
		state: StateEstablished,
	}
	tc.Start(context.Background())
	for i := byte(0); i < 10; i++ {
		t.Log("i", i)
		seg := Segment{
			SeqNum: uint32(i) * 3,
			AckNum: 0,
			Data:   []byte{i, i, i},
		}
		nWritten, err := testIn.Write(seg.Packet())
		if nWritten != SegmentHeaderLen+3 || err != nil {
			t.Fatalf("write n: %v, err: %+#v", nWritten, err)
		}
		readBuf := make([]byte, 10)
		nRead, err := tc.Read(readBuf)
		if nRead != 3 || err != nil || !reflect.DeepEqual(readBuf[:nRead], []byte{i, i, i}) {
			t.Fatalf("read n: %v, err: %+#v, buf: %+v", nRead, err, readBuf)
		}
		if tc.inputSeq != uint32(i+1)*3 || time.Since(tc.lastInputAck) > tc.LiveTiming.ReadTimeout*2 {
			t.Fatalf("input seq: %v, last input ack: %+v", tc.inputSeq, tc.lastInputAck)
		}
	}
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != os.ErrDeadlineExceeded {
		t.Fatalf("should not have read data n: %+v, err: %+v", n, err)
	}
	CheckTC(t, tc, 5, StateEstablished, 10*3, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_InboundSegments_ReadAll(t *testing.T) {
	testIn, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		InitialTiming: TimingConfig{
			ReadTimeout: 2 * time.Second,
		},
		state: StateEstablished,
	}
	tc.Start(context.Background())
	var wantData []byte
	for i := byte(0); i < 10; i++ {
		t.Log("i", i)
		wantData = append(wantData, i, i, i)
		seg := Segment{
			SeqNum: uint32(i) * 3,
			AckNum: 0,
			Data:   []byte{i, i, i},
		}
		nWritten, err := testIn.Write(seg.Packet())
		if nWritten != SegmentHeaderLen+3 || err != nil {
			t.Fatalf("write n: %v, err: %+#v", nWritten, err)
		}
	}
	// Read all at once.
	gotData, err := readInput(context.Background(), tc, 30)
	if err != nil || !reflect.DeepEqual(gotData, wantData) {
		t.Fatalf("read err: %+#v, got: %+v", err, gotData)
	}
	if tc.inputSeq != 30 || time.Since(tc.lastInputAck) > tc.LiveTiming.ReadTimeout {
		t.Fatalf("input seq: %v, last input ack: %+v", tc.inputSeq, tc.lastInputAck)
	}
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != os.ErrDeadlineExceeded {
		t.Fatalf("should not have read data n: %+v, err: %+v", n, err)
	}

	CheckTC(t, tc, 5, StateEstablished, 10*3, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_OutboundSegments_WriteNothing(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		ID:                      1111,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		// This test is not concerned with keep-alive.
		InitialTiming: TimingConfig{
			KeepAliveInterval: 999 * time.Second,
		},
		state: StateEstablished,
	}
	tc.Start(context.Background())
	n, err := tc.Write([]byte{})
	if n != 0 || err != nil {
		t.Fatalf("write: n %v, %+v", n, err)
	}

	// There should not be anything coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", n, err)
	}
	if tc.outputSeq != 0 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("output seq: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.outputSeq, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}

	CheckTC(t, tc, 5, StateEstablished, 0, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_OutboundSegments_Callback(t *testing.T) {
	_, inTransport := net.Pipe()
	callBackSegments := make(chan Segment, 10)
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		state:                   StateEstablished,
		MaxSegmentLenExclHeader: 10,
		InputTransport:          inTransport,
		OutputTransport:         ioutil.Discard,
		OutputSegmentCallback: func(seg Segment) {
			callBackSegments <- seg
		},
		// Leave retransmission, keep-alive, and delayed ack out of this test.
		InitialTiming: TimingConfig{
			KeepAliveInterval:      999 * time.Second,
			RetransmissionInterval: 999 * time.Second,
			AckDelay:               999 * time.Second,
		},
	}
	tc.Start(context.Background())
	n, err := tc.Write([]byte{0, 1, 2, 3, 4, 5})
	if n != 6 || err != nil {
		t.Fatalf("write: n %v, %+v", n, err)
	}
	CheckTC(t, tc, 3, StateEstablished, 0, 0, 0, nil, []byte{0, 1, 2, 3, 4, 5})
	got := <-callBackSegments
	want := Segment{
		ID:   1111,
		Data: []byte{0, 1, 2, 3, 4, 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+#v, want %+#v", got, want)
	}
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 6, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_OutboundSegments_WriteEach(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		ID:                      1111,
		MaxSegmentLenExclHeader: 5,
		// The sliding window should be large enough for all of the bytes written by the test.
		MaxSlidingWindow: 30,
		InputTransport:   inTransport,
		OutputTransport:  outTransport,
		state:            StateEstablished,
	}
	tc.Start(context.Background())
	var wantOutputBuf []byte
	for i := byte(0); i < 10; i++ {
		t.Log("i", i)
		n, err := tc.Write([]byte{i, i, i})
		if n != 3 || err != nil {
			t.Fatalf("write: n %v, %+v", n, err)
		}
		// Read the segment coming out of TC.
		segData, err := readInput(context.Background(), testOut, SegmentHeaderLen+3)
		if err != nil {
			t.Fatalf("read err: %+v", err)
		}
		gotSeg := SegmentFromPacket(segData)
		wantOutputBuf = append(wantOutputBuf, []byte{i, i, i}...)
		wantSeg := Segment{
			ID:     1111,
			SeqNum: uint32(i) * 3,
			AckNum: 0,
			Data:   []byte{i, i, i},
		}
		if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
			t.Fatalf("got seg data: %+v, got seg: %+v want: %+v", segData, gotSeg, wantSeg)
		}
		// Wait for output sequence number to catch up and verify output buffer.
		// In the absence of ack, the data remains in the buffer.
		CheckTC(t, tc, 1, StateEstablished, 0, 0, int(i+1)*3, nil, wantOutputBuf)
	}

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 10*3, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_OutboundSegments_WriteAll(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		MaxSegmentLenExclHeader: 10,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		state:                   StateEstablished,
	}
	tc.Start(context.Background())
	for i := byte(0); i < 5; i++ {
		t.Log("i", i)
		n, err := tc.Write([]byte{i, i})
		if n != 2 || err != nil {
			t.Fatalf("write: n %v, %+v", n, err)
		}
	}
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 0, nil, []byte{0, 0, 1, 1, 2, 2, 3, 3, 4, 4})
	// Read all output segments and verify the data.
	var gotData []byte
	for len(gotData) < 10 {
		seg := ReadSegmentHeaderData(t, context.Background(), testOut)
		gotData = append(gotData, seg.Data...)
	}
	// Wait for output sequence number to catch up and verify output buffer.
	// In the absence of ack, the data remains in the buffer.
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 10, nil, []byte{0, 0, 1, 1, 2, 2, 3, 3, 4, 4})

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}

	CheckTC(t, tc, 5, StateEstablished, 0, 0, 5*2, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_OutboundSegments_WriteWithRetransmission(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		MaxSegmentLenExclHeader: 3,
		MaxSlidingWindow:        64 * 3,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		InitialTiming: TimingConfig{
			// Leave keep-alive and retransmission out of this test.
			KeepAliveInterval:         999 * time.Second,
			AckDelay:                  999 * time.Second,
			RetransmissionInterval:    2 * time.Second,
			SlidingWindowWaitDuration: 1 * time.Second,
		},
		MaxRetransmissions: 5,
		state:              StateEstablished,
	}
	tc.Start(context.Background())
	n, err := tc.Write([]byte{0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3})
	if n != 12 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}
	// Discard all 4 segments without acknowledging to them.
	for i := 0; i < 4; i++ {
		got := readSegment(t, testOut, 3)
		if err != nil {
			t.Fatalf("err: %+v", err)
		}
		want := Segment{
			ID:     1111,
			SeqNum: uint32(i * 3),
			AckNum: 0,
			Data:   []byte{byte(i), byte(i), byte(i)},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got: %+v, want: %+v", got, want)
		}
	}
	// Wait for re-transmission.
	CheckTCError(t, tc, 3, 1, 0, 0)
	// Discard all 4 retransmitted segments.
	for i := 0; i < 4; i++ {
		got := readSegment(t, testOut, 3)
		if err != nil {
			t.Fatalf("err: %+v", err)
		}
		want := Segment{
			ID:     1111,
			SeqNum: uint32(i * 3),
			AckNum: 0,
			Data:   []byte{byte(i), byte(i), byte(i)},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got: %+v, want: %+v", got, want)
		}
	}
	// Gradually acknowledge segments.
	for i := 1; i < 4; i++ {
		ackSeg := Segment{
			SeqNum: 0,
			AckNum: uint32(i * 3),
			Data:   []byte{},
		}
		if _, err := testIn.Write(ackSeg.Packet()); err != nil {
			t.Fatalf("write ack: %+v", err)
		}
		// Wait for retransmission.
		CheckTCError(t, tc, 3, i+1, 0, 0)
		for j := i; j < 4; j++ {
			got := readSegment(t, testOut, 3)
			if err != nil {
				t.Fatalf("err: %+v", err)
			}
			want := Segment{
				ID:     1111,
				SeqNum: uint32(j * 3),
				AckNum: 0,
				Data:   []byte{byte(j), byte(j), byte(j)},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got: %+v, want: %+v", got, want)
			}
		}
	}
	// The TC is closed after exhausting all retransmission attempts.
	CheckTC(t, tc, 3, StateClosed, 0, 9, 12, nil, []byte{3, 3, 3})
	CheckTCError(t, tc, 3, 5, 0, 0)
}

func TestTransmissionControl_OutboundSegments_SaturateSlidingWindowWithoutAck(t *testing.T) {
	_, inTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         ioutil.Discard,
		// Leave sliding window sufficient small for this test.
		MaxSlidingWindow: 5,
		InitialTiming: TimingConfig{
			SlidingWindowWaitDuration: 1 * time.Second,
			RetransmissionInterval:    30 * time.Second,
			WriteTimeout:              5 * time.Second,
		},
		state: StateEstablished,
	}
	tc.Start(context.Background())

	// The first write operation fully saturates the sliding window.
	start := time.Now()
	n, err := tc.Write([]byte{0, 1, 2, 3, 4})
	if n != 5 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}
	if time.Since(start) > tc.LiveTiming.WriteTimeout/3 {
		t.Fatalf("write took unusually long to complete")
	}
	// The second write operation times out and does nothing.
	n, err = tc.Write([]byte{5, 6, 7, 8, 9})
	if n != 0 || err != os.ErrDeadlineExceeded {
		t.Fatalf("write n %v, %+v", n, err)
	}
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 5, nil, []byte{0, 1, 2, 3, 4})
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_OutboundSegments_SaturateSlidingWindowWithAck(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		// Leave sliding window sufficient small for this test.
		MaxSlidingWindow: 5,
		InitialTiming: TimingConfig{
			// Keep the sliding window wait duration short to shorten the test case
			// execution.
			SlidingWindowWaitDuration: 1 * time.Second,
			// Leave retransmission, keep-alive, and delayed ack out of this test.
			KeepAliveInterval:      999 * time.Second,
			RetransmissionInterval: 999 * time.Second,
			AckDelay:               999 * time.Second,
			ReadTimeout:            5 * time.Second,
			WriteTimeout:           5 * time.Second,
		},
		state: StateEstablished,
	}
	tc.Start(context.Background())

	// TC will absorb the data buffer in full though it exceeds the max sliding
	// window without timing out.
	// Consequently, the sliding window becomes saturated.
	n, err := tc.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
	if n != 20 || err != nil {
		log.Panicf("write n %v, %+v", n, err)
	}

	// Write another stream of data, which is going to fail due to saturated
	// sliding window.
	n, err = tc.Write([]byte{20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30})
	if n != 0 || err != os.ErrDeadlineExceeded {
		t.Fatalf("write n: %+v, err: %+v", n, err)
	}

	// Clear the sliding window saturation by reading all 20 bytes.
	for i := byte(0); i < 20; i += 5 {
		t.Log("read at", i)
		gotSeg := readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
		wantSeg := Segment{
			ID:     1111,
			SeqNum: uint32(i),
			AckNum: 0,
			Data:   []byte{i, i + 1, i + 2, i + 3, i + 4},
		}
		if !reflect.DeepEqual(gotSeg, wantSeg) {
			t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
		}
		t.Log("ack at ", i)
		waitForOutputSeq(t, tc, 1, int(i+5))
		ackSeg := Segment{
			SeqNum: 0,
			AckNum: uint32(i + 5),
			Data:   []byte{},
		}
		if _, err := testIn.Write(ackSeg.Packet()); err != nil {
			t.Fatalf("write ack: %+v", err)
		}
		waitForInputAck(t, tc, 1, int(i)+5)
	}

	// Sliding window saturation is now cleared, this write should succeed.
	n, err = tc.Write([]byte{31, 32, 33, 34, 35})
	if n != 5 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}
	// Read the same segment.
	gotSeg := readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
	wantSeg := Segment{
		ID:     1111,
		SeqNum: 20, // length of the first invocation of write
		AckNum: 0,
		Data:   []byte{31, 32, 33, 34, 35},
	}
	if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
		t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
	}

	CheckTC(t, tc, 5, StateEstablished, 0, 20, 20+5, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_DelayedAckAndKeepAlive(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		MaxSlidingWindow:        5,
		state:                   StateEstablished,
	}
	tc.Start(context.Background())

	// The first empty segment is for keep-alive.
	start := time.Now()
	lalog.DefaultLogger.Info("", nil, "tests expects the first keep-alive")
	gotSeg := readSegment(t, testOut, 0)
	lalog.DefaultLogger.Info("", nil, "test got the first keep-alive")
	wantSeg := Segment{
		ID:     1111,
		SeqNum: 0,
		AckNum: 0,
		Data:   []byte{},
		Flags:  FlagKeepAlive,
	}
	if !reflect.DeepEqual(gotSeg, wantSeg) {
		t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
	}
	if time.Since(start) < tc.LiveTiming.KeepAliveInterval {
		t.Fatalf("keep alive came too early")
	}

	// Write a segment to TC's input transport and expect a delayed ACK.
	seg := Segment{
		SeqNum: 0,
		AckNum: 0,
		Data:   []byte{255},
	}
	if _, err := testIn.Write(seg.Packet()); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	lalog.DefaultLogger.Info("", nil, "test expects a delayed ack")
	gotSeg = readSegment(t, testOut, 0)
	wantSeg = Segment{
		ID:     1111,
		SeqNum: 0,
		AckNum: 1,
		Data:   []byte{},
		Flags:  FlagAckOnly,
	}
	if !reflect.DeepEqual(gotSeg, wantSeg) {
		tc.DumpState()
		t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
	}
	if time.Since(start) < tc.LiveTiming.AckDelay || time.Since(start) > tc.LiveTiming.KeepAliveInterval {
		tc.DumpState()
		t.Fatalf("delayed ack came too early or too late")
	}

	// All further segments are for keep-alive only.
	for i := 0; i < 3; i++ {
		start = time.Now()
		lalog.DefaultLogger.Info("", nil, "test expects a keep-alive")
		gotSeg := readSegment(t, testOut, 0)
		wantSeg := Segment{
			ID:     1111,
			SeqNum: 0,
			AckNum: 1,
			Data:   []byte{},
			Flags:  FlagKeepAlive,
		}
		if !reflect.DeepEqual(gotSeg, wantSeg) {
			tc.DumpState()
			t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
		}
		if time.Since(start) < tc.LiveTiming.KeepAliveInterval {
			tc.DumpState()
			t.Fatalf("keep alive came too early")
		}
	}
	CheckTC(t, tc, 5, StateEstablished, 1, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_InitiatorHandshake(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	conf := InitiatorConfig{
		SetConfig:               true,
		MaxSegmentLenExclHeader: 111,
		Timing: TimingConfig{
			ReadTimeout:       100 * time.Second,
			WriteTimeout:      100 * time.Second,
			KeepAliveInterval: 333 * time.Second,
		},
	}
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		InitialTiming: TimingConfig{
			ReadTimeout:            2 * time.Second,
			WriteTimeout:           2 * time.Second,
			RetransmissionInterval: 5 * time.Second,
		},
		Initiator:            true,
		InitiatorConfig:      conf,
		InitiatorSegmentData: []byte{3, 2, 1},
	}
	tc.Start(context.Background())

	// Expect SYN with retransmissions.
	synData := conf.Bytes()
	synData = append(synData, 3, 2, 1)
	for i := 0; i < 3; i++ {
		lalog.DefaultLogger.Info("", nil, "test expects syn")
		syn := readSegment(t, testOut, InitiatorConfigLen+3)
		if !reflect.DeepEqual(syn, Segment{ID: 1111, Flags: FlagHandshakeSyn, Data: synData}) {
			t.Fatalf("incorrect syn seg: %+v", syn)
		}
	}

	// Send ACK and expect state transition.
	ack := Segment{Flags: FlagHandshakeAck}
	lalog.DefaultLogger.Info("", nil, "test writes ack")
	_, err := testIn.Write(ack.Packet())
	if err != nil {
		t.Fatalf("write err: %+v", err)
	}
	waitForState(t, tc, 10, StatePeerAck)

	// Expect SYN+ACK and expect state transition.
	lalog.DefaultLogger.Info("", nil, "test expects syn+ack")
	synAck := readSegment(t, testOut, 0)
	if !reflect.DeepEqual(synAck, Segment{ID: 1111, Flags: FlagHandshakeSyn | FlagHandshakeAck, Data: []byte{}}) {
		t.Fatalf("incorrect syn seg: %+v", synAck)
	}
	waitForState(t, tc, 10, StateEstablished)

	CheckTC(t, tc, 5, StateEstablished, 0, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)

	// Further SYN or SYN+ACK segments will not alter TC's state.
	unexpectedSeg := Segment{Flags: FlagHandshakeSyn | FlagHandshakeAck}
	_, err = testIn.Write(unexpectedSeg.Packet())
	if err != nil {
		t.Fatalf("write err: %+v", err)
	}
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 1, 0) // the error is for the unexpected segment.
}

func TestTransmissionControl_ResponderHandshake(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                      1111,
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		InitialTiming: TimingConfig{
			ReadTimeout:            2 * time.Second,
			WriteTimeout:           2 * time.Second,
			RetransmissionInterval: 5 * time.Second,
		},
	}
	tc.Start(context.Background())

	// Send SYN.
	conf := InitiatorConfig{}
	syn := Segment{Flags: FlagHandshakeSyn, Data: conf.Bytes()}
	lalog.DefaultLogger.Info("", nil, "test writes syn")
	_, err := testIn.Write(syn.Packet())
	if err != nil {
		t.Fatalf("write err: %+v", err)
	}
	waitForState(t, tc, 10, StateSynReceived)

	// Expect ACK with retransmissions.
	for i := 0; i < 3; i++ {
		lalog.DefaultLogger.Info("", nil, "test expects ack")
		ack := readSegment(t, testOut, 0)
		if !reflect.DeepEqual(ack, Segment{ID: 1111, Flags: FlagHandshakeAck, Data: []byte{}}) {
			t.Fatalf("incorrect ack seg: %+v", ack)
		}
	}

	// Send SYN+ACK.
	synAck := Segment{Flags: FlagHandshakeSyn | FlagHandshakeAck}
	lalog.DefaultLogger.Info("", nil, "test writes syn+ack")
	_, err = testIn.Write(synAck.Packet())
	if err != nil {
		t.Fatalf("write err: %+v", err)
	}
	waitForState(t, tc, 10, StateEstablished)
	CheckTC(t, tc, 5, StateEstablished, 0, 0, 0, nil, nil)
	// The input and output handler race against each other and that leads to
	// occasional negligible increment of the retransmission counter.
	if tc.ongoingRetransmissions > 1 {
		t.Fatalf("unexpected retransmission count: %v", tc.ongoingRetransmissions)
	}
	tc.mutex.Lock()
	tc.ongoingRetransmissions = 0
	tc.mutex.Unlock()
	// Check for other IO errors.
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_PeerHandshake(t *testing.T) {
	leftIn, leftInTransport := net.Pipe()
	rightIn, rightInTransport := net.Pipe()
	start := time.Now()

	wantTiming := TimingConfig{
		SlidingWindowWaitDuration: 1000 * time.Millisecond,
		RetransmissionInterval:    1234 * time.Millisecond,
		AckDelay:                  3456 * time.Millisecond,
		KeepAliveInterval:         4567 * time.Millisecond,
		ReadTimeout:               5678 * time.Millisecond,
		WriteTimeout:              7890 * time.Millisecond,
	}

	conf := InitiatorConfig{
		SetConfig:               true,
		MaxSegmentLenExclHeader: 111,
		Debug:                   true,
		Timing:                  wantTiming,
	}
	leftTC := &TransmissionControl{
		Debug:                false,
		ID:                   1111,
		InputTransport:       leftInTransport,
		OutputTransport:      rightIn,
		Initiator:            true,
		InitiatorConfig:      conf,
		InitiatorSegmentData: []byte{1, 2, 3},
	}
	leftTC.Start(context.Background())
	if !leftTC.slidingWindowFull() {
		t.Fatalf("should have blocked sending")
	}

	rightTC := &TransmissionControl{
		Debug:           false,
		ID:              2222,
		InputTransport:  rightInTransport,
		OutputTransport: leftIn,
	}
	rightTC.Start(context.Background())
	if !rightTC.slidingWindowFull() {
		t.Fatalf("should have blocked sending")
	}

	waitForState(t, leftTC, 10, StateEstablished)
	waitForState(t, rightTC, 10, StateEstablished)
	if time.Since(start) > leftTC.LiveTiming.RetransmissionInterval/2 {
		t.Fatalf("the handshake took unusually long")
	}
	if leftTC.slidingWindowFull() {
		t.Fatalf("should have unblocked sending")
	}
	if rightTC.slidingWindowFull() {
		t.Fatalf("should have unblocked sending")
	}

	CheckTC(t, leftTC, 5, StateEstablished, 0, 0, 0, nil, nil)
	CheckTCError(t, leftTC, 1, 0, 0, 0)
	CheckTC(t, rightTC, 5, StateEstablished, 0, 0, 0, nil, nil)
	CheckTCError(t, rightTC, 1, 0, 0, 0)

	// Check TC configuration.
	if !reflect.DeepEqual(rightTC.InitialTiming, wantTiming) {
		t.Fatalf("got: %+v, want: %+v", rightTC.InitialTiming, wantTiming)
	}
	if !reflect.DeepEqual(rightTC.LiveTiming, wantTiming) {
		t.Fatalf("got: %+v, want: %+v", rightTC.LiveTiming, wantTiming)
	}
	if !rightTC.Debug {
		t.Fatalf("did not set debug")
	}
}

func TestTransmissionControl_Reset(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:              1111,
		Debug:           true,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
		state:           StateEstablished,
	}
	tc.Start(context.Background())
	// Send a segment to reset the TC.
	resetSeg := Segment{
		Flags:  FlagReset,
		SeqNum: 0,
		AckNum: 0,
		Data:   []byte{},
	}
	nWritten, err := testIn.Write(resetSeg.Packet())
	if nWritten != SegmentHeaderLen || err != nil {
		t.Fatalf("write n: %v, err: %+#v", nWritten, err)
	}
	// Expect the TC to transmit an outbound reset segment before closing.
	segData, err := readInput(context.Background(), testOut, SegmentHeaderLen)
	if err != nil {
		t.Fatalf("read err: %+v", err)
	}
	resetSeg.ID = 1111
	gotSeg := SegmentFromPacket(segData)
	if !reflect.DeepEqual(gotSeg, resetSeg) {
		t.Fatalf("did not get a reset in return: %+v", gotSeg)
	}
	CheckTC(t, tc, 5, StateClosed, 0, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_MaxLifetime(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:              1111,
		Debug:           true,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
		MaxLifetime:     1 * time.Second,
		state:           StateEstablished,
	}
	tc.Start(context.Background())
	waitForState(t, tc, 3, StateClosed)
	// Expect the TC to transmit an outbound reset segment before closing.
	segData, err := readInput(context.Background(), testOut, SegmentHeaderLen)
	if err != nil {
		t.Fatalf("read err: %+v", err)
	}
	resetSeg := Segment{
		ID:     1111,
		Flags:  FlagReset,
		SeqNum: 0,
		AckNum: 0,
		Data:   []byte{},
	}
	gotSeg := SegmentFromPacket(segData)
	if !reflect.DeepEqual(gotSeg, resetSeg) {
		t.Fatalf("did not get a reset in return: %+v", gotSeg)
	}
	CheckTC(t, tc, 5, StateClosed, 0, 0, 0, nil, nil)
	CheckTCError(t, tc, 5, 0, 0, 0)
}

func TestTransmissionControl_PeerSimplexIO(t *testing.T) {
	leftIn, leftInTransport := net.Pipe()
	rightIn, rightInTransport := net.Pipe()

	leftTC := &TransmissionControl{
		Debug:                   true,
		ID:                      1111,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          leftInTransport,
		OutputTransport:         rightIn,
		Initiator:               true,
	}
	leftTC.Start(context.Background())

	rightTC := &TransmissionControl{
		Debug:                   true,
		ID:                      2222,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          rightInTransport,
		OutputTransport:         leftIn,
	}
	rightTC.Start(context.Background())

	waitForState(t, leftTC, 5, StateEstablished)
	waitForState(t, rightTC, 5, StateEstablished)

	go func() {
		for i := byte(0); i < 50; i++ {
			n, err := leftTC.Write([]byte{i, i, i})
			if n != 3 {
				log.Panicf("left tc write, i: %v, n: %v, err: %v", i, n, err)
			}
		}
	}()

	for i := byte(0); i < 50; i++ {
		got, err := readInput(context.Background(), rightTC, 3)
		if err != nil || !reflect.DeepEqual(got, []byte{i, i, i}) {
			t.Fatalf("right tc read, i: %d, err: %v, got: %v", i, err, got)
		}
	}

	// Switch to the other direction.
	go func() {
		for i := byte(0); i < 50; i++ {
			n, err := rightTC.Write([]byte{i, i, i})
			if n != 3 {
				log.Panicf("right tc write, i: %v, n: %v, err: %v", i, n, err)
			}
		}
	}()
	for i := byte(0); i < 50; i++ {
		got, err := readInput(context.Background(), leftTC, 3)
		if err != nil || !reflect.DeepEqual(got, []byte{i, i, i}) {
			t.Fatalf("left tc read, i: %d, err: %v, got: %v", i, err, got)
		}
	}

	CheckTC(t, leftTC, 5, StateEstablished, 3*50, 3*50, 3*50, nil, nil)
	CheckTCError(t, leftTC, 2, 0, 0, 0)
	CheckTC(t, rightTC, 5, StateEstablished, 3*50, 3*50, 3*50, nil, nil)
	CheckTCError(t, rightTC, 2, 0, 0, 0)

	// Close one peer and the other will close too.
	rightTC.Close()
	CheckTC(t, leftTC, 5, StateClosed, 3*50, 3*50, 3*50, nil, nil)
	CheckTCError(t, leftTC, 2, 0, 0, 0)
	CheckTC(t, rightTC, 5, StateClosed, 3*50, 3*50, 3*50, nil, nil)
	CheckTCError(t, rightTC, 2, 0, 0, 0)
}

func TestTransmissionControl_PeerDuplexIO(t *testing.T) {
	leftIn, leftInTransport := net.Pipe()
	rightIn, rightInTransport := net.Pipe()

	leftTC := &TransmissionControl{
		Debug:                   true,
		ID:                      1111,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          leftInTransport,
		OutputTransport:         rightIn,
		Initiator:               true,
	}
	leftTC.Start(context.Background())

	rightTC := &TransmissionControl{
		Debug:                   true,
		ID:                      2222,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          rightInTransport,
		OutputTransport:         leftIn,
	}
	rightTC.Start(context.Background())

	waitForState(t, leftTC, 5, StateEstablished)
	waitForState(t, rightTC, 5, StateEstablished)

	errs := make(chan error, 4)
	totalRounds := byte(255)
	go func() {
		for i := byte(0); i < totalRounds; i++ {
			n, err := leftTC.Write([]byte{i, i, i})
			if n != 3 {
				errs <- fmt.Errorf("left tc write, i: %v, n: %v, err: %v", i, n, err)
				return
			}
		}
		errs <- nil
	}()

	go func() {
		for i := byte(0); i < totalRounds; i++ {
			got, err := readInput(context.Background(), leftTC, 3)
			if err != nil || !reflect.DeepEqual(got, []byte{i, i, i}) {
				errs <- fmt.Errorf("left tc read, i: %d, err: %v, got: %v", i, err, got)
			}
		}
		errs <- nil
	}()

	go func() {
		for i := byte(0); i < totalRounds; i++ {
			n, err := rightTC.Write([]byte{i, i, i})
			if n != 3 {
				errs <- fmt.Errorf("right tc write, i:%v, n: %v, err: %v", i, n, err)
			}
		}
		errs <- nil
	}()

	go func() {
		for i := byte(0); i < totalRounds; i++ {
			got, err := readInput(context.Background(), rightTC, 3)
			if err != nil || !reflect.DeepEqual(got, []byte{i, i, i}) {
				errs <- fmt.Errorf("right tc read, i: %d, err: %v, got: %v", i, err, got)
			}
		}
		errs <- nil
	}()

	for i := 0; i < 4; i++ {
		err := <-errs
		if err != nil {
			t.Fatal(err)
		}
	}
	CheckTC(t, leftTC, 5, StateEstablished, 3*int(totalRounds), 3*int(totalRounds), 3*int(totalRounds), nil, nil)
	CheckTCError(t, leftTC, 5, 0, 0, 0)
	CheckTC(t, rightTC, 5, StateEstablished, 3*int(totalRounds), 3*int(totalRounds), 3*int(totalRounds), nil, nil)
	CheckTCError(t, rightTC, 5, 0, 0, 0)

	// Close one peer and the other will close too.
	leftTC.Close()
	CheckTC(t, leftTC, 5, StateClosed, 3*int(totalRounds), 3*int(totalRounds), 3*int(totalRounds), nil, nil)
	CheckTCError(t, leftTC, 2, 0, 0, 0)
	CheckTC(t, rightTC, 5, StateClosed, 3*int(totalRounds), 3*int(totalRounds), 3*int(totalRounds), nil, nil)
	CheckTCError(t, rightTC, 2, 0, 0, 0)
}

func TestRuntimeIntervalConfig(t *testing.T) {
	_, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		state:           StateEstablished,
		Debug:           true,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
		MaxLifetime:     1 * time.Second,
		InitialTiming: TimingConfig{
			AckDelay:          2 * time.Second,
			KeepAliveInterval: 3 * time.Second,
		},
	}
	tc.Start(context.Background())
	if tc.LiveTiming.AckDelay != tc.InitialTiming.AckDelay {
		t.Fatalf("unexpected ack delay: %v", tc.LiveTiming.AckDelay)
	}
	if tc.LiveTiming.KeepAliveInterval != tc.InitialTiming.KeepAliveInterval {
		t.Fatalf("unexpected keepalive: %v", tc.LiveTiming.KeepAliveInterval)
	}

	tc.DecreaseTimingInterval()
	if tc.LiveTiming.AckDelay != tc.InitialTiming.AckDelay/2 {
		t.Fatalf("unexpected ack delay: %v", tc.LiveTiming.AckDelay)
	}
	if tc.LiveTiming.KeepAliveInterval != tc.InitialTiming.KeepAliveInterval/2 {
		t.Fatalf("unexpected keepalive: %v", tc.LiveTiming.KeepAliveInterval)
	}
	if tc.LiveTiming.SlidingWindowWaitDuration != tc.InitialTiming.SlidingWindowWaitDuration/2 {
		t.Fatalf("unexpected sliding window: %v", tc.LiveTiming.SlidingWindowWaitDuration)
	}
	for i := 0; i < 20; i++ {
		tc.DecreaseTimingInterval()
	}
	// It won't decrease any further.
	if tc.LiveTiming.AckDelay != tc.InitialTiming.AckDelay/256 {
		t.Fatalf("unexpected ack delay: %v", tc.LiveTiming.AckDelay)
	}
	if tc.LiveTiming.KeepAliveInterval != tc.InitialTiming.KeepAliveInterval/256 {
		t.Fatalf("unexpected keepalive: %v", tc.LiveTiming.KeepAliveInterval)
	}
	if tc.LiveTiming.SlidingWindowWaitDuration != tc.InitialTiming.SlidingWindowWaitDuration/256 {
		t.Fatalf("unexpected sliding window: %v", tc.LiveTiming.SlidingWindowWaitDuration)
	}
	// Increase by one notch.
	tc.IncreaseTimingInterval()
	if tc.LiveTiming.AckDelay != tc.InitialTiming.AckDelay/128 {
		t.Fatalf("unexpected ack delay: %v", tc.LiveTiming.AckDelay)
	}
	if tc.LiveTiming.KeepAliveInterval != tc.InitialTiming.KeepAliveInterval/128 {
		t.Fatalf("unexpected keepalive: %v", tc.LiveTiming.KeepAliveInterval)
	}
	if tc.LiveTiming.SlidingWindowWaitDuration != tc.InitialTiming.SlidingWindowWaitDuration/128 {
		t.Fatalf("unexpected sliding window: %v", tc.LiveTiming.SlidingWindowWaitDuration)
	}
	// It won't exceed the initial timing.
	for i := 0; i < 20; i++ {
		tc.IncreaseTimingInterval()
	}
	if tc.LiveTiming.AckDelay != tc.InitialTiming.AckDelay {
		t.Fatalf("unexpected ack delay: %v", tc.LiveTiming.AckDelay)
	}
	if tc.LiveTiming.KeepAliveInterval != tc.InitialTiming.KeepAliveInterval {
		t.Fatalf("unexpected keep-alive delay, got  %v, want %v", tc.LiveTiming.AckDelay, tc.InitialTiming.AckDelay)
	}
	if tc.LiveTiming.SlidingWindowWaitDuration != tc.InitialTiming.SlidingWindowWaitDuration {
		t.Fatalf("unexpected sliding window: %v", tc.LiveTiming.SlidingWindowWaitDuration)
	}
}
