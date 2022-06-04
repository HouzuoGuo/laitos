package tcpoverdns

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"reflect"
	"testing"
	"time"

	runtimePprof "runtime/pprof"

	"github.com/HouzuoGuo/laitos/lalog"
)

func DumpGoroutinesOnInterrupt() {
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

func waitForState(t *testing.T, tc *TransmissionControl, timeoutSec int, wantState State) {
	t.Helper()
	for i := 0; i < timeoutSec*10; i++ {
		tc.mutex.Lock()
		gotState := tc.state
		tc.mutex.Unlock()
		if gotState == wantState {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("got tc state %d, want state %v", tc.state, wantState)
}

func TestTransmissionControl_InboundSegments_ReadNothing(t *testing.T) {
	_, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:           true,
		InputTransport:  inTransport,
		OutputTransport: outTransport,
		ReadTimeout:     3 * time.Second,
		state:           StateEstablished,
	}
	tc.Start(context.Background())
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != ErrTimeout {
		t.Fatalf("read n: %+v, err: %+v", n, err)
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.inputAck != 0 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("input ack: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.inputAck, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
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
		ReadTimeout:             3 * time.Second,
		state:                   StateEstablished,
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
		if tc.inputSeq != uint32(i)*3 || time.Since(tc.lastInputAck) > tc.ReadTimeout*2 {
			t.Fatalf("ack number: got %v, want %v, last input ack: %+v", tc.inputAck, (i+1)*3, tc.lastInputAck)
		}
	}
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != ErrTimeout {
		t.Fatalf("should not have read data n: %+v, err: %+v", n, err)
	}
	if tc.inputAck != 0 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("input ack: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.inputAck, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}

func TestTransmissionControl_InboundSegments_ReadAll(t *testing.T) {
	testIn, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		ReadTimeout:             2 * time.Second,
		state:                   StateEstablished,
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
	if tc.inputSeq != 30-3 || time.Since(tc.lastInputAck) > tc.ReadTimeout {
		t.Fatalf("ack number: %+v, last input ack: %+v", tc.inputAck, tc.lastInputAck)
	}
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != ErrTimeout {
		t.Fatalf("should not have read data n: %+v, err: %+v", n, err)
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.inputAck != 0 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("input ack: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.inputAck, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}

func TestTransmissionControl_OutboundSegments_WriteNothing(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		ReadTimeout:             3 * time.Second,
		state:                   StateEstablished,
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
}

func TestTransmissionControl_OutboundSegments_WriteEach(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		state:                   StateEstablished,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
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
			SeqNum: uint32(i) * 3,
			AckNum: 0,
			Data:   []byte{i, i, i},
		}
		if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
			t.Fatalf("got seg data: %+v, got seg: %+v want: %+v", segData, gotSeg, wantSeg)
		}
		// Verify output sequence number.
		waitForOutputSeq(t, tc, 3, int((i+1)*3))
		// In the absence of ack, the data remains in the buffer.
		if !reflect.DeepEqual(tc.outputBuf, wantOutputBuf) {
			t.Fatalf("wrong data in output buf, got %v, want %v", tc.outputBuf, wantOutputBuf)
		}
	}

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.outputSeq != 10*3 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("output seq: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.outputSeq, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}

func TestTransmissionControl_OutboundSegments_WriteAll(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
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
	// Read all at once, TC should have combined 5 bursts of data into a single
	// segment after a short while.
	time.Sleep(100 * time.Millisecond)
	segData, err := readInput(context.Background(), testOut, SegmentHeaderLen+10)
	if err != nil {
		t.Fatalf("read err: %+v", err)
	}
	gotSeg := SegmentFromPacket(segData)
	wantSeg := Segment{
		SeqNum: 0,
		AckNum: 0,
		Data:   []byte{0, 0, 1, 1, 2, 2, 3, 3, 4, 4},
	}
	if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
		t.Fatalf("got seg data: %+v, got seg: %+v want: %+v", segData, gotSeg, wantSeg)
	}
	// Verify output sequence number.
	waitForOutputSeq(t, tc, 3, 10)
	// In the absence of ack, the data remains in the buffer.
	if !reflect.DeepEqual(tc.outputBuf, []byte{0, 0, 1, 1, 2, 2, 3, 3, 4, 4}) {
		t.Fatalf("wrong data in output buf, got %v", tc.outputBuf)
	}

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.outputSeq != 5*2 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("output seq: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.outputSeq, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}

func TestTransmissionControl_OutboundSegments_WriteWithRetransmission(t *testing.T) {
	// TODO FIXME: fix me first
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		// Leave the retransmission interval short to shorten the test case
		// execution.
		RetransmissionInterval: 1 * time.Second,
		MaxRetransmissions:     3,
		ReadTimeout:            2 * time.Second,
		state:                  StateEstablished,
	}
	tc.Start(context.Background())
	// Write a segment without acknowledging it.
	n, err := tc.Write([]byte{1, 1, 1})
	if n != 3 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}
	// Look one retransmission and then acknowledge it.
	segData, err := readInput(context.Background(), testOut, SegmentHeaderLen+3)
	if err != nil {
		t.Fatalf("err: %+v", err)
	}
	gotSeg := SegmentFromPacket(segData)
	wantSeg := Segment{
		SeqNum: 0,
		AckNum: 0,
		Data:   []byte{1, 1, 1},
	}
	if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
		t.Fatalf("retrans first seg data: %+v, got seg: %+v want: %+v", segData, gotSeg, wantSeg)
	}
	// Acknowledge the first transmission.
	ackSeg := Segment{
		SeqNum: 0,
		AckNum: 3,
		Data:   []byte{},
	}
	if _, err := testIn.Write(ackSeg.Packet()); err != nil {
		t.Fatalf("write ack: %+v", err)
	}

	// Write a second segment without acknowledging it.
	n, err = tc.Write([]byte{2, 2, 2})
	if n != 3 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}
	// Look for three retransmissions.
	for i := 0; i < 3; i++ {
		t.Log("i", i)
		segData, err := readInput(context.Background(), testOut, SegmentHeaderLen+3)
		if err != nil {
			t.Fatalf("err: %+v", err)
		}
		t.Logf("seg data: %+v", segData)
		gotSeg := SegmentFromPacket(segData)
		wantSeg := Segment{
			SeqNum: 3,
			AckNum: 0,
			Data:   []byte{2, 2, 2},
		}
		if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
			t.Fatalf("retrans second seg data: %+v, got seg: %+v want: %+v", segData, gotSeg, wantSeg)
		}
	}

	time.Sleep(tc.SlidingWindowWaitDuration * 2)
	if tc.inputAck != 3 {
		t.Fatalf("wrong input ack: %+v", tc.inputAck)
	}

	// There should not be anything else coming out of the transport.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}

	// The connection is broken and closed.
	// The single error is "DrainInputFromTransport: failed to read segment header: context canceled".
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.outputSeq != 3+3 || tc.ongoingRetransmissions != 3 || tc.state != StateClosed || tc.inputTransportErrors != 1 || tc.outputTransportErrors != 0 {
		t.Fatalf("output seq: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.outputSeq, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}
func TestTransmissionControl_OutboundSegments_SaturateSlidingWindowWithoutAck(t *testing.T) {
	_, inTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         io.Discard,
		// Leave sliding window sufficient small for this test.
		MaxSlidingWindow:          5,
		SlidingWindowWaitDuration: 1 * time.Second,
		RetransmissionInterval:    30 * time.Second,
		WriteTimeout:              5 * time.Second,
		state:                     StateEstablished,
	}
	tc.Start(context.Background())

	// The first write operation saturates the sliding window.
	start := time.Now()
	n, err := tc.Write([]byte{0, 1, 2, 3, 4})
	if n != 5 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}
	if time.Since(start) > tc.WriteTimeout/3 {
		t.Fatalf("write took unusually long to complete")
	}
	// The second write operation is blocked due to sliding window saturation.
	n, err = tc.Write([]byte{5, 6, 7, 8, 9})
	if n != 0 || err != ErrTimeout {
		t.Fatalf("write n %v, %+v", n, err)
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.outputSeq != 5 || tc.ongoingRetransmissions != 0 || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("output seq: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.outputSeq, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}

func TestTransmissionControl_OutboundSegments_SaturateSlidingWindowWithAck(t *testing.T) {
	// TODO FIXME: fix me second
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		// Leave sliding window sufficient small for this test.
		MaxSlidingWindow: 5,
		// Keep the sliding window wait duration short to shorten the test case
		// execution.
		SlidingWindowWaitDuration: 1 * time.Second,
		RetransmissionInterval:    30 * time.Second,
		ReadTimeout:               5 * time.Second,
		WriteTimeout:              5 * time.Second,
		state:                     StateEstablished,
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
	// sliding window .
	n, err = tc.Write([]byte{20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30})
	if n != 0 || err != ErrTimeout {
		t.Fatalf("write n: %+v, err: %+v", n, err)
	}

	// Clear the sliding window saturation by reading all 20 bytes.
	for i := byte(0); i < 20; i += 5 {
		t.Log("read at", i)
		gotSeg := readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
		wantSeg := Segment{
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
	}
	tc.DumpState()

	// Write again after clearing sliding window saturation.
	n, err = tc.Write([]byte{31, 32, 33, 34, 35})
	if n != 5 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}

	// Read the latest segment after having cleared the satured sliding window.
	gotSeg := readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
	wantSeg := Segment{
		SeqNum: 20, // length of the first invocation of write
		AckNum: 0,
		Data:   []byte{31, 32, 33, 34, 35},
	}
	if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
		t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
	}

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.outputSeq != 20+5 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("output seq: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.outputSeq, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}

func TestTransmissionControl_KeepAlive(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                     true,
		MaxSegmentLenExclHeader:   5,
		InputTransport:            inTransport,
		OutputTransport:           outTransport,
		MaxSlidingWindow:          5,
		SlidingWindowWaitDuration: 1 * time.Second,
		RetransmissionInterval:    3 * time.Second,
		MaxRetransmissions:        3,
		ReadTimeout:               2 * time.Second,
		WriteTimeout:              2 * time.Second,
		// Keep the keep alive interval short and below read timeout.
		KeepAliveInterval: 1 * time.Second,
		state:             StateEstablished,
	}
	tc.Start(context.Background())

	//  Wait for the keep-alive segment to arrive.
	for i := 0; i < 3; i++ {
		gotSeg := readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
		wantSeg := Segment{
			SeqNum: 0,
			AckNum: 0,
			Data:   []byte{},
		}
		if !reflect.DeepEqual(gotSeg, wantSeg) {
			t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
		}
	}
	if tc.inputSeq != 0 || tc.outputSeq != 0 || tc.ongoingRetransmissions != 0 || tc.state != StateEstablished || tc.inputTransportErrors != 0 || tc.outputTransportErrors != 0 {
		t.Fatalf("input seq: %v, output seq: %v, retrans: %v, state: %v, in err: %d, out err: %d", tc.inputSeq, tc.outputSeq, tc.ongoingRetransmissions, tc.state, tc.inputTransportErrors, tc.outputTransportErrors)
	}
}

func TestTransmissionControl_InitiatorHandshake(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            2 * time.Second,
		Initiator:               true,
		RetransmissionInterval:  5 * time.Second,
	}
	tc.Start(context.Background())

	// Expect SYN with retransmissions.
	for i := 0; i < 3; i++ {
		lalog.DefaultLogger.Info("", "", nil, "test expects syn")
		syn := readSegment(t, testOut, 0)
		if !reflect.DeepEqual(syn, Segment{Flags: FlagSyn, Data: []byte{}}) {
			t.Fatalf("incorrect syn seg: %+v", syn)
		}
	}

	// Send ACK and expect state transition.
	ack := Segment{Flags: FlagAck}
	lalog.DefaultLogger.Info("", "", nil, "test writes ack")
	_, err := testIn.Write(ack.Packet())
	if err != nil {
		t.Fatalf("write err: %+v", err)
	}
	waitForState(t, tc, 10, StatePeerAck)

	// Expect SYN+ACK and expect state transition.
	lalog.DefaultLogger.Info("", "", nil, "test expects syn+ack")
	synAck := readSegment(t, testOut, 0)
	if !reflect.DeepEqual(synAck, Segment{Flags: FlagSyn | FlagAck, Data: []byte{}}) {
		t.Fatalf("incorrect syn seg: %+v", synAck)
	}
	waitForState(t, tc, 10, StateEstablished)
}

func TestTransmissionControl_ResponderHandshake(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            2 * time.Second,
		RetransmissionInterval:  5 * time.Second,
	}
	tc.Start(context.Background())

	// Send SYN.
	syn := Segment{Flags: FlagSyn}
	lalog.DefaultLogger.Info("", "", nil, "test writes syn")
	_, err := testIn.Write(syn.Packet())
	if err != nil {
		t.Fatalf("write err: %+v", err)
	}
	waitForState(t, tc, 10, StateSynReceived)

	// Expect ACK with retransmissions.
	for i := 0; i < 3; i++ {
		lalog.DefaultLogger.Info("", "", nil, "test expects ack")
		ack := readSegment(t, testOut, 0)
		if !reflect.DeepEqual(ack, Segment{Flags: FlagAck, Data: []byte{}}) {
			t.Fatalf("incorrect ack seg: %+v", ack)
		}
	}

	// Send SYN+ACK.
	synAck := Segment{Flags: FlagSyn | FlagAck}
	lalog.DefaultLogger.Info("", "", nil, "test writes syn+ack")
	_, err = testIn.Write(synAck.Packet())
	if err != nil {
		t.Fatalf("write err: %+v", err)
	}
	waitForState(t, tc, 10, StateEstablished)
}

func TestTransmissionControl_PeerHandshake(t *testing.T) {
	leftIn, leftInTransport := net.Pipe()
	rightIn, rightInTransport := net.Pipe()
	start := time.Now()

	leftTC := &TransmissionControl{
		Debug:                   true,
		ID:                      "LEFT",
		MaxSegmentLenExclHeader: 5,
		InputTransport:          leftInTransport,
		OutputTransport:         rightIn,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            2 * time.Second,
		RetransmissionInterval:  5 * time.Second,
		Initiator:               true,
	}
	leftTC.Start(context.Background())
	if !leftTC.slidingWindowFull() {
		t.Fatalf("should have blocked sending")
	}

	rightTC := &TransmissionControl{
		Debug:                   true,
		ID:                      "RIGHT",
		MaxSegmentLenExclHeader: 5,
		InputTransport:          rightInTransport,
		OutputTransport:         leftIn,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            2 * time.Second,
		RetransmissionInterval:  5 * time.Second,
	}
	rightTC.Start(context.Background())
	if !rightTC.slidingWindowFull() {
		t.Fatalf("should have blocked sending")
	}

	waitForState(t, leftTC, 10, StateEstablished)
	waitForState(t, rightTC, 10, StateEstablished)
	if time.Since(start) > leftTC.RetransmissionInterval/2 {
		t.Fatalf("the handshake took unusually long")
	}
	if leftTC.slidingWindowFull() {
		t.Fatalf("should have unblocked sending")
	}
	if rightTC.slidingWindowFull() {
		t.Fatalf("should have unblocked sending")
	}
}

func TestTransmissionControl_PeerDuplexIO(t *testing.T) {
	leftIn, leftInTransport := net.Pipe()
	rightIn, rightInTransport := net.Pipe()

	leftTC := &TransmissionControl{
		Debug:                   true,
		ID:                      "LEFT",
		MaxSegmentLenExclHeader: 5,
		InputTransport:          leftInTransport,
		OutputTransport:         rightIn,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            2 * time.Second,
		RetransmissionInterval:  5 * time.Second,
		Initiator:               true,
	}
	leftTC.Start(context.Background())

	rightTC := &TransmissionControl{
		Debug:                   true,
		ID:                      "RIGHT",
		MaxSegmentLenExclHeader: 5,
		InputTransport:          rightInTransport,
		OutputTransport:         leftIn,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            2 * time.Second,
		RetransmissionInterval:  5 * time.Second,
	}
	rightTC.Start(context.Background())
	t.Skip("TODO FIXME: fix sliding window saturation detection")

	for i := 0; i < 100; i++ {
		n, err := leftTC.Write(bytes.Repeat([]byte{1, 2, 3}, 20))
		if n != 60 {
			t.Fatalf("left tc write n: %v, err: %v", n, err)
		}
	}
}
