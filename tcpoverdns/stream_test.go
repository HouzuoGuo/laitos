package tcpoverdns

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

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

func TestTransmissionControl_InboundSegments_ReadNothing(t *testing.T) {
	_, inTransport := net.Pipe()
	_, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		CongestionWindow:        10,
		CongestionWaitDuration:  1 * time.Second,
		RetransmissionInterval:  1 * time.Second,
		ReadTimeout:             2 * time.Second,
	}
	tc.Start(context.Background())
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != ErrTimeout {
		t.Fatalf("read n: %+v, err: %+v", n, err)
	}
	if tc.inputAck != 0 {
		t.Fatalf("ack number: ack %v, last input ack: %+v", tc.inputAck, tc.lastInputAck)
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
		CongestionWindow:        10,
		CongestionWaitDuration:  1 * time.Second,
		RetransmissionInterval:  1 * time.Second,
		ReadTimeout:             2 * time.Second,
	}
	tc.Start(context.Background())
	for i := byte(0); i < 10; i++ {
		t.Log("i", i)
		seg := Segment{SeqNum: int(i) * 3, Data: []byte{i, i, i}}
		nWritten, err := testIn.Write(seg.Packet())
		if nWritten != SegmentHeaderLen+3 || err != nil {
			t.Fatalf("write n: %v, err: %+#v", nWritten, err)
		}
		readBuf := make([]byte, 10)
		nRead, err := tc.Read(readBuf)
		if nRead != 3 || err != nil || !reflect.DeepEqual(readBuf[:nRead], []byte{i, i, i}) {
			t.Fatalf("read n: %v, err: %+#v, buf: %+v", nRead, err, readBuf)
		}
		if tc.inputSeq != int(i)*3 || time.Since(tc.lastInputAck) > tc.ReadTimeout*2 {
			t.Fatalf("ack number: got %v, want %v, last input ack: %+v", tc.inputAck, int(i+1)*3, tc.lastInputAck)
		}
	}
	// The next read times out due to lack of further input segments.
	n, err := tc.Read(nil)
	if n != 0 || err != ErrTimeout {
		t.Fatalf("should not have read data n: %+v, err: %+v", n, err)
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
		CongestionWindow:        10,
		CongestionWaitDuration:  1 * time.Second,
		RetransmissionInterval:  1 * time.Second,
		ReadTimeout:             2 * time.Second,
	}
	tc.Start(context.Background())
	var wantData []byte
	for i := byte(0); i < 10; i++ {
		t.Log("i", i)
		wantData = append(wantData, i, i, i)
		seg := Segment{SeqNum: int(i) * 3, Data: []byte{i, i, i}}
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
}

func TestTransmissionControl_OutboundSegments_WriteNothing(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		CongestionWindow:        10,
		CongestionWaitDuration:  1 * time.Second,
		RetransmissionInterval:  1 * time.Second,
		ReadTimeout:             2 * time.Second,
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
	if tc.outputSeq != 0 {
		t.Fatalf("unexpected seq number: %v", tc.outputSeq)
	}
}

func TestTransmissionControl_OutboundSegments_WriteEach(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		// Leave congestion window sufficient large for all outgoing segments.
		// This test case does not focus on congestion.
		CongestionWindow:       100,
		CongestionWaitDuration: 1 * time.Second,
		// Leave retransmission interval much longer than the typical execution
		// time of this test case. This test case does not intend to involve
		// retransmission.
		RetransmissionInterval: 10 * time.Second,
	}
	tc.Start(context.Background())
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
		wantSeg := Segment{
			SeqNum: int(i) * 3,
			AckNum: 0,
			Data:   []byte{i, i, i},
		}
		if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
			t.Fatalf("got seg data: %+v, got seg: %+v want: %+v", segData, gotSeg, wantSeg)
		}
	}

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
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
		// Leave congestion window sufficient large for all outgoing segments.
		// This test case does not focus on congestion.
		CongestionWindow:       100,
		CongestionWaitDuration: 1 * time.Second,
		// Leave retransmission interval much longer than the typical execution
		// time of this test case. This test case does not intend to involve
		// retransmission.
		RetransmissionInterval: 10 * time.Second,
		ReadTimeout:            2 * time.Second,
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
	// segment.
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

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}
}

func TestTransmissionControl_OutboundSegments_WriteWithRetransmission(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		// Leave congestion window sufficient large for all outgoing segments.
		// This test case does not focus on congestion.
		CongestionWindow:       100,
		CongestionWaitDuration: 1 * time.Second,
		// Leave the retransmission interval short to shorten the test case
		// execution.
		RetransmissionInterval: 1 * time.Second,
		MaxRetransmissions:     3,
		ReadTimeout:            2 * time.Second,
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

	time.Sleep(tc.CongestionWaitDuration * 2)
	if tc.inputAck != 3 {
		t.Fatalf("wrong input ack: %+v", tc.inputAck)
	}

	// The connection is now broken, there won't be further retransmissions.
	if tc.State != StateClosed {
		t.Fatalf("unexpected state: %+v", tc.State)
	}

	// There should not be anything else coming out.
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	data, err := readInput(timeout, testOut, 10)
	if len(data) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read n: %+v, err: %+v", len(data), err)
	}
}

func TestTransmissionControl_OutboundSegments_WriteWithCongestion(t *testing.T) {
	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		// Leave congestion window sufficient small for this test.
		CongestionWindow: 5,
		// Keep the congestion window wait duration short to shorten the test
		// case execution.
		CongestionWaitDuration: 1 * time.Second,
		RetransmissionInterval: 3 * time.Second,
		MaxRetransmissions:     3,
		ReadTimeout:            2 * time.Second,
		WriteTimeout:           2 * time.Second,
	}
	tc.Start(context.Background())

	// Write a long stream of data that must be broken into multiple segments.
	n, err := tc.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
	if n != 20 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}

	// Read the first segment without acknowledging.
	gotSeg := readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
	wantSeg := Segment{
		SeqNum: 0,
		AckNum: 0,
		Data:   []byte{0, 1, 2, 3, 4},
	}
	if !reflect.DeepEqual(gotSeg, wantSeg) {
		t.Fatalf("got seg: %+#v, want seg: %+#v", gotSeg, wantSeg)
	}

	// Wait for output sequence number to catch up.
	for {
		if tc.outputSeq >= 5 {
			t.Logf("%v %v %v", tc.outputSeq, tc.inputAck, tc.outputSeq-tc.inputAck >= tc.CongestionWindow)
			break
		} else {
			time.Sleep(100 * time.Millisecond)
			t.Logf("%v %v %v", tc.outputSeq, tc.inputAck, tc.outputSeq-tc.inputAck >= tc.CongestionWindow)
		}
	}

	// Write another stream of data, which is going to fail due to congestion.
	n, err = tc.Write([]byte{20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30})
	if n != 0 || err != ErrTimeout {
		t.Fatalf("write n: %+v, err: %+v", n, err)
	}

	// Acknowledge all 20 bytes and then write another stream of data.
	ackSeg := Segment{
		SeqNum: 0,
		AckNum: 20,
		Data:   []byte{},
	}
	if _, err := testIn.Write(ackSeg.Packet()); err != nil {
		t.Fatalf("write ack: %+v", err)
	}
	n, err = tc.Write([]byte{31, 32, 33, 34, 35})
	if n != 5 || err != nil {
		t.Fatalf("write n %v, %+v", n, err)
	}

	// Read all segments from the first invocation of write.
	for i := byte(1); i <= 3; i++ {
		t.Log("i", i)
		gotSeg := readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
		wantSeg := Segment{
			SeqNum: int(i) * 5,
			AckNum: 0,
			Data:   []byte{i*5 + 0, i*5 + 1, i*5 + 2, i*5 + 3, i*5 + 4},
		}
		if err != nil || !reflect.DeepEqual(gotSeg, wantSeg) {
			t.Fatalf("got seg: %+v want: %+v", gotSeg, wantSeg)
		}
	}

	// Read the segment from the third invocation of write.
	gotSeg = readSegment(t, testOut, tc.MaxSegmentLenExclHeader)
	wantSeg = Segment{
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
}

func TestTransmissionControl_KeepAlive(t *testing.T) {
	_, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		Debug:                   true,
		MaxSegmentLenExclHeader: 5,
		InputTransport:          inTransport,
		OutputTransport:         outTransport,
		CongestionWindow:        5,
		CongestionWaitDuration:  1 * time.Second,
		RetransmissionInterval:  3 * time.Second,
		MaxRetransmissions:      3,
		ReadTimeout:             2 * time.Second,
		WriteTimeout:            2 * time.Second,
		// Keep the keep alive interval short and below read timeout.
		KeepAliveInterval: 1 * time.Second,
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
}
