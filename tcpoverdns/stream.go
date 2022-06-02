package tcpoverdns

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	// ReadStarvationRetryInterval specifies a duration to wait when a reader
	// is starved of data.
	ReadStarvationRetryInterval = 100 * time.Millisecond
	// SegmentDataTimeout specifies the time limit in between the arrival of a
	// segment header and segment data.
	SegmentDataTimeout = 5 * time.Second
	//MaxSegmentDataLen is the maximum permissible segment length.
	MaxSegmentDataLen = 8192
)

var (
	ErrTimeout = errors.New("transmission control IO timeout")
)

// TransmissionControl provides TCP-like features for duplex transportation of
// data between an initiator and a responder, with flow and congestion control,
// customisable segment size, and guaranteed in-order delivery.
// The behaviour is inspired by but not a replica of the Internet standard TCP.
type TransmissionControl struct {
	io.ReadCloser
	io.WriteCloser

	// ID is a unique identifier of the transport control, it is primarily used
	// for logging.
	ID string
	// Debug enables/disables verbose logging for IO activities.
	Debug bool
	// Logger is used to log IO activities when verbose logging is enabled.
	Logger lalog.Logger

	// Initiator determines whether this transmission control will initiate
	// the handshake sequence with the peer.
	// Otherwise, this transmission control remains passive at the start.
	Initiator bool
	// State is the current transmission control connection state.
	state State

	// lastOutputSyn is the timestamp of the latest outbound segment with with syn
	// flag (used for handhsake).
	lastOutputSyn time.Time

	// MaxSegmentLenExclHeader is the maximum length of a single segment in both
	// directions, excluding the headers.
	MaxSegmentLenExclHeader int
	// InputTransport transports inbound segments.
	InputTransport io.Reader
	// OutputTransport transports outbound segments.
	OutputTransport io.Writer

	// ReadTimeout specifies a time limit for the Read function.
	ReadTimeout time.Duration
	// WriteTimeout specifies a time limit for the Write function.
	WriteTimeout time.Duration

	context   context.Context
	cancelFun func()

	// Buffer input and output for callers.
	inputBuf  []byte
	outputBuf []byte

	// CongestionWindow is the maximum length of outbound data that can be
	// transported whilst waiting for acknowledgement.
	CongestionWindow uint32
	// CongestionWaitDuration is a short duration to wait during congestion
	// before retrying.
	CongestionWaitDuration time.Duration
	// RetransmissionInterval is a short duration to wait before re-transmitting
	// the unacknowledged outbound segments (if any).
	RetransmissionInterval time.Duration
	// MaxRetransmissions is the maximum number of retransmissions that can be
	// made for handshake and data segments before the transmission control is
	// irreversably closed.
	MaxRetransmissions int
	// KeepAliveInterval is a short duration to wait before transmitting an
	// outbound ack segment in the absence of outbound data.
	// This internal must be longer than the peer's retransmission interval.
	KeepAliveInterval time.Duration
	// MaxTransportErrors is the maximum number of consecutive errors to
	// tolerate from input and output transports before closing down the
	// transmission control.
	MaxTransportErrors int

	// ongoingRetransmissions is the number of retransmissions being made for a
	// handshake or data segment.
	ongoingRetransmissions int

	// inputTransportErrors is the number of consecutive IO errors that have
	// occurred in the input transport so far.
	inputTransportErrors int
	// outputTransportErrors is the number of consecutive IO errors that have
	// occurred in the output transport so far.
	outputTransportErrors int

	// inputSeq is the latest sequence number read from inbound segments.
	inputSeq uint32
	// inputAck is the latest sequence number acknowledged by inbound segments.
	inputAck uint32
	// lastInputAck is the timestamp of the latest inbound segment.
	lastInputAck time.Time
	// outputSeq is the latest sequence number written for outbound segments.
	outputSeq uint32
	// lastOutput is the timestamo of the latest write operation done to the
	// output transport.
	lastOutput time.Time

	mutex *sync.Mutex
}

func (tc *TransmissionControl) Start(ctx context.Context) {
	// Give parameters a default value.
	if tc.MaxSegmentLenExclHeader == 0 {
		tc.MaxSegmentLenExclHeader = 256
	}
	if tc.ReadTimeout == 0 {
		tc.ReadTimeout = 15 * time.Second
	}
	if tc.WriteTimeout == 0 {
		tc.WriteTimeout = 15 * time.Second
	}
	if tc.CongestionWindow == 0 {
		tc.CongestionWindow = 256
	}
	if tc.CongestionWaitDuration == 0 {
		tc.CongestionWaitDuration = 1 * time.Second
	}
	if tc.RetransmissionInterval == 0 {
		tc.RetransmissionInterval = 10 * time.Second
	}
	if tc.KeepAliveInterval == 0 {
		tc.KeepAliveInterval = 5 * time.Second
	}
	if tc.MaxRetransmissions == 0 {
		tc.MaxRetransmissions = 3
	}
	if tc.MaxTransportErrors == 0 {
		tc.MaxTransportErrors = 10
	}

	tc.context, tc.cancelFun = context.WithCancel(ctx)
	tc.lastInputAck = time.Now()
	tc.lastOutput = time.Now()
	tc.mutex = new(sync.Mutex)
	tc.Logger.ComponentID = append(tc.Logger.ComponentID, lalog.LoggerIDField{Key: "TCID", Value: tc.ID})
	go tc.drainInputFromTransport()
	go tc.drainOutputToTransport()
}

func (tc *TransmissionControl) Write(buf []byte) (int, error) {
	// Drain input into the internal send buffer.
	tc.mutex.Lock()
	initialSeq := tc.outputSeq
	tc.mutex.Unlock()
	start := time.Now()
	if tc.Debug {
		tc.Logger.Info("Write", "", nil, "writing buf %v, has congestion? %v", buf, tc.hasCongestion())
	}
	for tc.hasCongestion() {
		// Wait for congestion to clear.
		if time.Since(start) < tc.WriteTimeout {
			<-time.After(ReadStarvationRetryInterval)
			continue
		} else {
			if tc.Debug {
				tc.Logger.Info("Write", fmt.Sprintf("%v", buf), nil, "abort write due to timeout ")
			}
			return 0, ErrTimeout
		}
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.outputSeq < initialSeq {
		// Retransmission happened while sender was waiting, don't let this
		// buffer through or it will be out of sequence.
		if tc.Debug {
			tc.Logger.Info("Write", fmt.Sprintf("%v", buf), nil, "abort write due to unexpected output sequence number %d which should have been %d", tc.outputSeq, initialSeq)
		}
		return 0, ErrTimeout
	}
	tc.outputBuf = append(tc.outputBuf, buf...)
	// There is no need to wait for the output sequence number to catch up.
	return len(buf), nil
}

// hasCongestion returns true if the output transport's backlog exceeds the
// congestion threshold or the transmission control is not established yet.
func (tc *TransmissionControl) hasCongestion() bool {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	return tc.state < StateEstablished || tc.inputAck < tc.outputSeq && tc.outputSeq-tc.inputAck >= tc.CongestionWindow
}

func (tc *TransmissionControl) drainOutputToTransport() {
	if tc.Debug {
		tc.Logger.Info("drainOutputToTransport", "", nil, "starting now")
	}
	defer func() {
		if tc.Debug {
			tc.Logger.Info("drainOutputToTransport", "", nil, "returning and closing")
		}
		tc.Close()
	}()
	for tc.state < StateClosed {
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()

		if instant.state < StateEstablished {
			if instant.Initiator {
				// The transmission control carries on with the handshake
				// sequence as the initiator.
				switch instant.state {
				case StateEmpty:
					// Got nothing yet, send SYN.
					if time.Since(instant.lastOutputSyn) < tc.RetransmissionInterval {
						// Avoid flooding the peer with repeated SYN.
						continue
					}
					if instant.ongoingRetransmissions >= tc.MaxRetransmissions {
						tc.Logger.Info("drainOutputToTransport", "", nil, "handshake syn has got no response after multiple attempts, closing.")
						return
					}
					seg := Segment{Flags: FlagSyn}
					if tc.Debug {
						tc.Logger.Info("drainOutputToTransport", "", nil, "sending handshake, state: %v, seg: %+#v", instant.state, seg)
					}
					_ = tc.writeToOutputTransport(seg)
					tc.mutex.Lock()
					tc.lastOutputSyn = time.Now()
					tc.ongoingRetransmissions++
					tc.mutex.Unlock()
				case StatePeerAck:
					// Got ack, send SYN + ACK.
					seg := Segment{Flags: FlagSyn | FlagAck}
					if tc.Debug {
						tc.Logger.Info("drainOutputToTransport", "", nil, "handshake completed, sending syn+ack: %+#v", seg)
					}
					_ = tc.writeToOutputTransport(seg)
					tc.mutex.Lock()
					tc.state = StateEstablished
					tc.lastOutputSyn = time.Now()
					tc.ongoingRetransmissions = 0
					tc.mutex.Unlock()
				default:
					tc.Logger.Warning("drainOutputToTransport", "", nil, "logical state error: %v", tc.state)
				}
			} else {
				// The transmission control carries on with the handshake
				// sequence as the responder.
				switch instant.state {
				case StateSynReceived:
					// Got SYN, send ACK.
					if time.Since(instant.lastOutputSyn) < tc.RetransmissionInterval {
						// Avoid flooding the peer with repeated ACK.
						continue
					}
					if instant.ongoingRetransmissions >= tc.MaxRetransmissions {
						tc.Logger.Info("drainOutputToTransport", "", nil, "handshake ack has got no response after multiple attempts, closing.")
						return
					}
					seg := Segment{Flags: FlagAck}
					if tc.Debug {
						tc.Logger.Info("drainOutputToTransport", "", nil, "sending handshake ack, state: %v, seg: %+#v", instant.state, seg)
					}
					_ = tc.writeToOutputTransport(seg)
					tc.mutex.Lock()
					tc.lastOutputSyn = time.Now()
					tc.ongoingRetransmissions++
					tc.mutex.Unlock()
				}
			}
		} else if tc.state == StateEstablished && time.Since(instant.lastInputAck) > tc.RetransmissionInterval && instant.inputAck < instant.outputSeq {
			// Re-transmit the segments since the latest acknowledgement.
			tc.ongoingRetransmissions++
			if tc.Debug {
				tc.Logger.Info("drainOutputToTransport", "", nil, "re-transmitting, last input ack time: %+v, input ack: %+v, output seq: %+v, ongoing retransmissions: %v", instant.lastInputAck, instant.inputAck, instant.outputSeq, tc.ongoingRetransmissions)
			}
			if tc.ongoingRetransmissions >= tc.MaxRetransmissions {
				if tc.Debug {
					tc.Logger.Info("drainOutputToTransport", "", nil, "reached max retransmissions")
				}
				return
			}
			tc.writeSegments(instant.inputAck, instant.outputBuf[instant.inputAck:instant.outputSeq])
			// Wait a short duration before the next transmission.
			select {
			case <-time.After(tc.CongestionWaitDuration):
				continue
			case <-tc.context.Done():
				return
			}
		} else if tc.state == StateEstablished && instant.hasCongestion() {
			// Wait for a short duration and retry.
			if tc.Debug {
				tc.Logger.Info("drainOutputToTransport", "", nil, "wait due to congestion, output seq: %+v, input ack: %+v, congestion window: %+v", instant.outputSeq, instant.inputAck, tc.CongestionWindow)
			}
			select {
			case <-time.After(tc.CongestionWaitDuration):
				continue
			case <-tc.context.Done():
				return
			}
		} else if tc.state == StateEstablished {
			// Send output segments starting with the latest sequence number.
			toSend := instant.outputBuf[instant.outputSeq:]
			if len(toSend) > int(tc.CongestionWindow) {
				toSend = toSend[:int(tc.CongestionWindow)]
			}
			if len(toSend) == 0 {
				if time.Since(instant.lastOutput) < tc.KeepAliveInterval {
					// Wait for write to be called with more data.
					select {
					case <-time.After(ReadStarvationRetryInterval):
						continue
					case <-tc.context.Done():
						return
					}
				} else {
					// Send an empty segment for keep-alive.
					emptySeg := Segment{
						SeqNum: instant.outputSeq,
						AckNum: instant.inputSeq,
						Data:   []byte{},
					}
					if tc.Debug {
						tc.Logger.Info("drainOutputToTransport", "", nil, "sending keep-alive: %+#v", emptySeg)
					}
					_ = tc.writeToOutputTransport(emptySeg)
				}
			} else {
				// The segment data can be empty. An empty segment is for
				// keep-alive.
				if tc.Debug {
					tc.Logger.Info("drainOutputToTransport", "", nil, "sending segments totalling %d bytes: %+v", len(toSend), toSend)
				}
				written := tc.writeSegments(instant.outputSeq, toSend)
				tc.mutex.Lock()
				// Clear the retransmission counter if retransmission happened.
				tc.ongoingRetransmissions = 0
				tc.outputSeq += written
				tc.mutex.Unlock()
			}
		}
	}
}

func (tc *TransmissionControl) writeSegments(seqNum uint32, buf []byte) uint32 {
	for i := 0; i < len(buf); i += tc.MaxSegmentLenExclHeader {
		// Split the buffer into individual segments maximum
		// MaxSegmentLenExclHeader bytes each.
		thisSeg := buf[i:]
		if len(thisSeg) > tc.MaxSegmentLenExclHeader {
			thisSeg = thisSeg[:tc.MaxSegmentLenExclHeader]
		}
		seg := Segment{
			SeqNum: seqNum + uint32(i),
			AckNum: tc.inputSeq,
			Data:   thisSeg,
		}
		if tc.Debug {
			tc.Logger.Info("writeSegments", "", nil, "writing to output transport: %+#v", seg)
		}
		err := tc.writeToOutputTransport(seg)
		if err != nil {
			return uint32(i)
		}
	}
	return uint32(len(buf))
}

func (tc *TransmissionControl) Read(buf []byte) (int, error) {
	start := time.Now()
	var readLen int
	for {
		tc.mutex.Lock()
		// Drain from received data buffer to caller's buffer.
		readLen = copy(buf[readLen:], tc.inputBuf)
		// Remove drained portion from the internal buffer.
		tc.inputBuf = tc.inputBuf[readLen:]
		tc.mutex.Unlock()
		if readLen > 0 {
			// Caller has got some data.
			return readLen, nil
		} else if time.Since(start) < tc.ReadTimeout {
			// Wait for more input data to arrive and then retry.
			<-time.After(ReadStarvationRetryInterval)
		} else {
			if tc.Debug {
				tc.Logger.Info("Read", "", nil, "time out, want %d, got %d, got data: %v", len(buf), readLen, buf[:readLen])
			}
			return readLen, ErrTimeout
		}
	}
}

func (tc *TransmissionControl) drainInputFromTransport() {
	if tc.Debug {
		tc.Logger.Info("drainInputFromTransport", "", nil, "starting now")
	}
	defer func() {
		if tc.Debug {
			tc.Logger.Info("drainInputFromTransport", "", nil, "returning and closing")
		}
		tc.Close()
	}()
	// Continuously read the bytes of inputBuf using the underlying transit.
	for tc.state < StateClosed {
		// Read the segment header first.
		segHeader, err := tc.readFromInputTransport(tc.context, SegmentHeaderLen)
		if err != nil {
			if tc.Debug {
				tc.Logger.Info("drainInputFromTransport", "", nil, "failed to read segment header: %+v", err)
			}
			continue
		}
		// Read the segment data.
		segDataLen := int(binary.BigEndian.Uint16(segHeader[SegmentHeaderLen-2 : SegmentHeaderLen]))
		if segDataLen > MaxSegmentDataLen {
			tc.Logger.Warning("drainInputFromTransport", "", nil, "seg data len (%d) must be less than %d", segDataLen, MaxSegmentDataLen)
			continue
		}
		segDataCtx, segDataCtxCancel := context.WithTimeout(tc.context, SegmentDataTimeout)
		segData, err := tc.readFromInputTransport(segDataCtx, segDataLen)
		if err != nil {
			segDataCtxCancel()
			if tc.Debug {
				tc.Logger.Info("drainInputFromTransport", "", nil, "failed to read segment data: %+v", err)
			}
			continue
		}
		seg := SegmentFromPacket(append(segHeader, segData...))
		if seg.Flags.Has(FlagMalformed) {
			tc.Logger.Warning("drainInputFromTransport", "", nil, "failed to decode the segment, header: %v, data: %v", segHeader, segData)
			segDataCtxCancel()
			continue
		}
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()
		if instant.state < StateEstablished {
			if tc.Debug {
				tc.Logger.Info("drainInputFromTransport", "", nil, "handshake ongoing, received: %+#v", seg)
			}
			if instant.Initiator {
				if instant.state == StateEmpty {
					// SYN was sent, expect ACK.
					if seg.Flags == FlagAck {
						if tc.Debug {
							tc.Logger.Info("drainInputFromTransport", "", nil, "transition to StatePeerAck")
						}
						tc.state = StatePeerAck
					} else {
						tc.Logger.Warning("drainInputFromTransport", "", nil, "expecting ack, got: %+#v", seg)
					}
				}
			} else {
				switch instant.state {
				case StateEmpty:
					// Expect SYN.
					if seg.Flags == FlagSyn {
						if tc.Debug {
							tc.Logger.Info("drainInputFromTransport", "", nil, "transition to StateSynReceived")
						}
						tc.state = StateSynReceived
					} else {
						tc.Logger.Warning("drainInputFromTransport", "", nil, "expecting syn, got: %+#v", seg)
					}
				default:
					// Expect SYN+ACK.
					if seg.Flags == FlagSyn|FlagAck {
						if tc.Debug {
							tc.Logger.Info("drainInputFromTransport", "", nil, "transition to StateEstablished")
						}
						tc.state = StateEstablished
					} else {
						tc.Logger.Warning("drainInputFromTransport", "", nil, "expecting syn+ack, got: %+#v", seg)
					}
				}
			}
		} else {
			tc.mutex.Lock()
			if tc.inputSeq == 0 || tc.inputSeq+uint32(len(seg.Data)) == seg.SeqNum {
				if tc.Debug {
					tc.Logger.Info("drainInputFromTransport", "", nil, "received %+#v", seg)
				}
				// Ensure the new segment is consecutive to the ones already
				// received. There is no selective acknowledgement going on here.
				tc.inputSeq = seg.SeqNum
				tc.inputAck = seg.AckNum
				tc.lastInputAck = time.Now()
				tc.inputBuf = append(tc.inputBuf, seg.Data...)
			} else {
				if tc.Debug {
					tc.Logger.Info("drainInputFromTransport", "", nil, "received out of sequence segment: %+#v, input seq: %v", seg, tc.inputSeq)
				}
				// Do nothing, wait for retransmission.
			}
			tc.mutex.Unlock()
		}
		segDataCtxCancel()
	}
}

// State returns the stream state.
func (tc *TransmissionControl) State() State {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	return tc.state
}

// OutputSeq returns the latest output sequence number.
func (tc *TransmissionControl) OutputSeq() uint32 {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	return tc.outputSeq
}

// writeToOutputTransport writes the segment to the output transport and returns
// the IO error (if any).
// The function briefly locks the transmission control mutex, therefore the
// caller must not hold the mutex.
// If there has been an exceeding number of IO errors from the output transport,
// then the transmission control will be closed.
func (tc *TransmissionControl) writeToOutputTransport(seg Segment) error {
	_, err := tc.OutputTransport.Write(seg.Packet())
	tc.mutex.Lock()
	tc.lastOutput = time.Now()
	if err == nil {
		tc.outputTransportErrors = 0
		tc.mutex.Unlock()
		return nil
	} else {
		tc.outputTransportErrors++
		if tc.outputTransportErrors >= tc.MaxTransportErrors {
			tc.Logger.Warning("writeToOutputTransport", "", nil, "closing due to exceedingly many transport errors")
			tc.mutex.Unlock()
			tc.Close()
		}
		return err
	}
}

// readFromInputTransport reads the desired number of bytes from the input
// transport and returns the IO error (if any).
// The function briefly locks the transmission control mutex, therefore the
// caller must not hold the mutex.
// If there has been an exceeding number of IO errors from the output transport,
// then the transmission control will be closed.
func (tc *TransmissionControl) readFromInputTransport(ctx context.Context, totalLen int) ([]byte, error) {
	n, err := readInput(ctx, tc.InputTransport, totalLen)
	tc.mutex.Lock()
	if err == nil {
		tc.inputTransportErrors = 0
		tc.mutex.Unlock()
		return n, err
	} else {
		tc.inputTransportErrors++
		if tc.inputTransportErrors >= tc.MaxTransportErrors {
			tc.Logger.Warning("readFromInputTransport", "", nil, "closing due to exceedingly many transport errors")
			tc.mutex.Unlock()
			tc.Close()
		}
		return n, err
	}
}

// Close terminates ongoing IO activities and terminates the stream.
func (tc *TransmissionControl) Close() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	if tc.state == StateClosed {
		return
	}
	if tc.Debug {
		tc.Logger.Info("Close", "", nil, "closing")
	}
	tc.state = StateClosed
	tc.cancelFun()
}

// readInput reads from transmission control's input transport for a total
// number of bytes specified in totalLen.
// The function always returns the desired number of bytes read or an error.
func readInput(ctx context.Context, in io.Reader, totalLen int) ([]byte, error) {
	buf := make([]byte, totalLen)
	for i := 0; i < totalLen; {
		recvLen := make(chan int, 1)
		recvErr := make(chan error, 1)
		go func() {
			n, err := in.Read(buf[i:totalLen])
			recvLen <- n
			recvErr <- err
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case n := <-recvLen:
			err := <-recvErr
			if err != nil {
				return nil, err
			}
			i += n
			continue
		}
	}
	return buf, nil
}
