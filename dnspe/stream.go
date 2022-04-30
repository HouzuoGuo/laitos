package dnspe

import (
	"context"
	"io"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	StarvationRetryInterval = 1 * time.Second
)

// TransmissionControl provides TCP-like features for duplex transportation of
// data between an initiator and a responder, with flow and congestion control,
// customisable segment size, and guaranteed in-order delivery.
type TransmissionControl struct {
	io.ReadCloser
	io.WriteCloser

	// Debug enables/disables verbose logging for IO activities.
	Debug bool
	// Logger is used to log IO activities when verbose logging is enabled.
	Logger lalog.Logger

	// MaxSegmentLenExclHeader is the maximum length of a single segment in both
	// directions, excluding the headers.
	MaxSegmentLenExclHeader int
	// InputTransport transports inbound segments.
	InputTransport io.Reader
	// OutputTransport transports outbound segments.
	OutputTransport io.Writer

	context   context.Context
	cancelFun func()

	// Buffer input and output for callers.
	inputBuf  []byte
	inputErr  chan error
	outputBuf []byte
	outputErr chan error

	// CongestionWindow is the maximum length of outbound data that can be
	// transported whilst waiting for acknowledgement.
	CongestionWindow int
	// CongestionWaitDuration is a short duration to wait during congestion
	// before retrying.
	CongestionWaitDuration time.Duration
	// RetransmissionInterval is a short duration to wait before re-transmitting
	// the unacknowledged outbound segments (if any).
	RetransmissionInterval time.Duration

	// inputSeq is the latest sequence number read from inbound segments.
	inputSeq int
	// inputAck is the latest sequence number acknowledged by inbound segments.
	inputAck int
	// lastInputAck is the timestamp of the latest inbound segment.
	lastInputAck time.Time

	// outputSeq is the latest sequence number written for outbound segments.
	outputSeq int
}

func (tc *TransmissionControl) Start(ctx context.Context) {
	tc.outputErr = make(chan error, 1)
	tc.inputErr = make(chan error, 1)
	tc.context, tc.cancelFun = context.WithCancel(ctx)
	go tc.DrainInputFromTransport()
	go tc.DrainOutputToTransport()
}

func (tc *TransmissionControl) Write(buf []byte) (int, error) {
	// FIXME: block caller while waiting for congestion to clear.
	// Drain input into the internal send buffer.
	tc.outputBuf = append(tc.outputBuf, buf...)
	return len(buf), nil
}

func (tc *TransmissionControl) DrainOutputToTransport() {
	if tc.Debug {
		tc.Logger.Info("DrainOutputToTransport", "", nil, "starting now")
	}
	for {
		// Wait for outgoing data.
		if len(tc.outputBuf) == 0 {
			select {
			case <-time.After(StarvationRetryInterval):
				continue
			case <-tc.context.Done():
				return
			}
		}
		// Decide whether to re-transmit unacknowledged data, wait for ongoing
		// congestion to clear, or transmit incoming data in a segment.
		if time.Since(tc.lastInputAck) > tc.RetransmissionInterval && tc.inputAck < tc.outputSeq {
			// Re-transmit the segments since the latest acknowledgement.
			if tc.Debug {
				tc.Logger.Info("DrainOutputToTransport", "", nil, "re-transmitting, last input ack time: %+v, input ack: %+v, output seq: %+v", tc.lastInputAck, tc.inputAck, tc.outputSeq)
			}
			tc.writeSegments(tc.inputAck, tc.outputBuf[:tc.inputSeq-tc.outputSeq])
		} else if tc.outputSeq-tc.inputAck > tc.CongestionWindow {
			// Wait for a short duration and retry.
			if tc.Debug {
				tc.Logger.Info("DrainOutputToTransport", "", nil, "wait due to congestion, output seq: %+v, input ack: %+v, congestion window: %+v", tc.outputSeq, tc.inputAck, tc.CongestionWindow)
			}
			select {
			case <-time.After(tc.CongestionWaitDuration):
				continue
			case <-tc.context.Done():
				return
			}
		} else {
			// Send output segments starting with the latest sequence number.
			toSend := tc.outputBuf[tc.outputSeq:]
			if len(toSend) > tc.CongestionWindow {
				toSend = toSend[:tc.CongestionWindow]
			}
			if tc.Debug {
				tc.Logger.Info("DrainOutputToTransport", "", nil, "sending segments totalling %d bytes: %+v", len(toSend), toSend)
			}
		}
	}
}

func (tc *TransmissionControl) writeSegments(seqNum int, buf []byte) {
	for i := 0; i < len(buf); i += tc.MaxSegmentLenExclHeader {
		// Split the buffer into individual segments maximum
		// MaxSegmentLenExclHeader bytes each.
		thisSeg := buf[i:]
		if len(thisSeg) > tc.MaxSegmentLenExclHeader {
			thisSeg = thisSeg[:tc.MaxSegmentLenExclHeader]
		}
		seg := Segment{
			SeqNum: seqNum + i,
			AckNum: tc.inputSeq,
			Data:   thisSeg,
		}
		if tc.Debug {
			tc.Logger.Info("writeSegments", "", nil, "segment seq: %+v, segment ack: %+v, len segment: %d, data: %+v", seg.SeqNum, seg.AckNum, len(seg.Data), seg.Data)
		}
		_, err := tc.OutputTransport.Write(seg.Packet())
		if err != nil {
			// FIXME: maybe log and retry? How to detect permanent failure?
			tc.outputErr <- err
			return
		}
	}
}

func (tc *TransmissionControl) Read(buf []byte) (int, error) {
	// FIXME: block caller while waiting for data.
	// Drain received data buffer to caller.
	readLen := copy(buf, tc.inputBuf)
	// Remove received portion from the internal buffer.
	tc.inputBuf = tc.inputBuf[readLen:]
	return readLen, nil
}

func (tc *TransmissionControl) DrainInputFromTransport() {
	// Continuously read the bytes of inputBuf using the underlying transit.
	for {
		recvBuf := make([]byte, tc.MaxSegmentLenExclHeader)
		recvLen := make(chan int, 1)
		recvErr := make(chan error, 1)
		go func() {
			// FIXME: apply a short IO timeout slightly longer than the starvation retry interval.
			n, err := tc.InputTransport.Read(recvBuf)
			// FIXME: maybe log and retry? How to detect permanent failure?
			recvLen <- n
			recvErr <- err
		}()
		select {
		case <-tc.context.Done():
			return
		case <-time.After(StarvationRetryInterval):
			continue
		case n := <-recvLen:
			// Break down into segments.
			if tc.Debug {
				tc.Logger.Info("DrainInputFromTransport", "", nil, "received len: %+v, data: %+v", recvLen, recvBuf)
			}
			for i := 0; i < n; {
				// FIXME: make the function return err on parser failure.
				seg := SegmentFromPacket(recvBuf[i:])
				i += len(seg.Data)
				tc.inputBuf = append(tc.inputBuf, seg.Data...)
				if tc.Debug {
					tc.Logger.Info("DrainInputFromTransport", "", nil, "got segment %+#v", seg)
				}
			}
		}
	}
}

func (tc *TransmissionControl) Close() error {
	tc.cancelFun()
	return nil
}
