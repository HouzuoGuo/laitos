package tcpoverdns

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	// BusyWaitInterval specifies a short duration in between consecutive
	// busy-wait operations.
	BusyWaitInterval = 100 * time.Millisecond
	// SegmentDataTimeout specifies the timeout between the arrival of a segment
	// header and the segment data.
	SegmentDataTimeout = 10 * time.Second
	//MaxSegmentDataLen is the maximum permissible segment length.
	MaxSegmentDataLen = 8192
)

// TransmissionControl provides TCP-like features for duplex transportation of
// data between an initiator and a responder, with flow sliding window control,
// customisable segment size, and guaranteed in-order delivery.
// The behaviour is inspired by but not a replica of the Internet standard TCP.
type TransmissionControl struct {
	io.ReadCloser
	io.WriteCloser

	// ID is a file descriptor-like number that identifies all outgoing segments
	// as well as used for logging.
	ID uint16
	// Debug enables verbose logging for IO activities.
	Debug bool
	// Logger is used to log IO activities when verbose logging is enabled.
	Logger lalog.Logger

	// Initiator determines whether this transmission control will initiate
	// the handshake sequence with the peer.
	// Otherwise, this transmission control remains passive at the start.
	Initiator bool
	// InitiatorSegmentData is an optional byte array carried by initiator's
	// handshake (SYN) segment. It must be shorter than MaxSegmentLenExclHeader.
	InitiatorSegmentData []byte

	// lastOutputSyn is the timestamp of the latest outbound segment with with syn
	// flag (used for handhsake).
	lastOutputSyn time.Time

	// MaxSegmentLenExclHeader is the maximum length of the data portion in an
	// outgoing segment, the length excludes the headers.
	MaxSegmentLenExclHeader int
	// InputTransport transports inbound segments.
	InputTransport io.Reader
	// OutputTransport transports outbound segments.
	OutputTransport io.Writer
	// OutputSegmentCallback (optional) is invoked for each outbound segment as
	// they are written to output transport.
	OutputSegmentCallback func(Segment)

	// ReadTimeout specifies a time limit for the Read function.
	ReadTimeout time.Duration
	// WriteTimeout specifies a time limit for the Write function.
	WriteTimeout time.Duration

	context   context.Context
	cancelFun func()

	// State is the current transmission control connection state.
	state State
	// closeAfterDrained signals the transmission control to close as soon as
	// the output buffer is drained.
	closeAfterDrained bool
	// Buffer input and output for callers.
	inputBuf  []byte
	outputBuf []byte

	// MaxSlidingWindow is the maximum length of data buffered in the outgoing
	// direction without receiving acknowledge from the peer.
	// This number is comparable to the TCP flow control sliding window.
	MaxSlidingWindow uint32
	// SlidingWindowWaitDuration is a short duration to wait the peer's
	// acknowledgement before this transmission control sends more data to the
	// output transport.
	SlidingWindowWaitDuration time.Duration
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
	// MaxLifetime is the maximum lifetime of the transmission control. After
	// the lifetime elapses, the transmission control will be unconditionally
	// closed/terminated.
	// This is used as a safeguard against transmission control going stale
	// without being properly closed/terminated.
	MaxLifetime time.Duration

	// AckDelay is a short delay between receiving the latest segment and
	// sending an outbound acknowledgement-only segment.
	// It should shorter than the retransmission interval by a magnitude.
	AckDelay time.Duration
	// lastAckOnlySeg is the timestamp of the latest acknowledgment-only segment
	// (i.e. delayed ack or keep-alive) sent to the output transport.
	lastAckOnlySeg time.Time

	// ongoingRetransmissions is the number of retransmissions being made for a
	// handshake or data segment.
	ongoingRetransmissions int

	// inputTransportErrors is the number of IO errors that have occurred when
	// reading a segment from the input transport.
	// The number is reset to 0 when a valid segment is read successfully from
	// the transport.
	inputTransportErrors int
	// outputTransportErrors is the number of IO errors that have occurred when
	// writing a segment from the input transport.
	// The number is reset to 0 when a valid segment is written succesfully to
	// the transport.
	outputTransportErrors int

	// inputSeq is the sequence number of the latest byte read from the inbound
	// segment (i.e. seg.SeqNum + len(seg.Data)).
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
	// startTime is the timestamp of the moment Start is called.
	startTime time.Time

	mutex *sync.Mutex
}

// Start initialises the internal state of the transmission control.
// Start may not be called after the transmission control is stopped.
func (tc *TransmissionControl) Start(ctx context.Context) {
	if tc.state == StateClosed {
		panic("caller may not restart an already stopped transmission control")
	}
	// Give parameters a default value.
	if tc.MaxSegmentLenExclHeader == 0 {
		tc.MaxSegmentLenExclHeader = 256
	}
	if tc.ReadTimeout == 0 {
		tc.ReadTimeout = 20 * time.Second
	}
	if tc.WriteTimeout == 0 {
		tc.WriteTimeout = 20 * time.Second
	}
	if tc.MaxSlidingWindow == 0 {
		tc.MaxSlidingWindow = 256
	}
	if tc.RetransmissionInterval == 0 {
		// 10 seconds
		tc.RetransmissionInterval = tc.ReadTimeout / 2
	}
	if tc.KeepAliveInterval == 0 {
		// 5 seconds
		tc.KeepAliveInterval = tc.RetransmissionInterval / 2
	}
	if tc.SlidingWindowWaitDuration == 0 {
		// 3 seconds
		tc.SlidingWindowWaitDuration = tc.RetransmissionInterval / 3
	}
	if tc.AckDelay == 0 {
		// 1 second
		tc.AckDelay = tc.SlidingWindowWaitDuration / 3
	}
	if tc.MaxRetransmissions == 0 {
		tc.MaxRetransmissions = 3
	}
	if tc.MaxTransportErrors == 0 {
		tc.MaxTransportErrors = 10
	}
	if tc.MaxLifetime == 0 {
		tc.MaxLifetime = 10 * time.Minute
	}

	tc.context, tc.cancelFun = context.WithCancel(ctx)
	tc.lastInputAck = time.Now()
	tc.lastOutput = time.Now()
	tc.lastAckOnlySeg = time.Now()
	tc.startTime = time.Now()
	tc.mutex = new(sync.Mutex)
	tc.Logger = lalog.Logger{
		ComponentName: "TC",
		ComponentID:   []lalog.LoggerIDField{{Key: "ID", Value: tc.ID}},
	}
	go tc.drainInputFromTransport()
	go tc.drainOutputToTransport()
}

func (tc *TransmissionControl) Write(buf []byte) (int, error) {
	if tc.Debug {
		tc.Logger.Info("Write", "", nil, "(sliding window full? %v state? %v) writing buf %v", tc.slidingWindowFull(), tc.State(), lalog.ByteArrayLogString(buf))
	}
	if tc.State() == StateClosed {
		return 0, io.EOF
	}
	start := time.Now()
	for tc.slidingWindowFull() {
		// Wait for peer to acknowledge before sending more.
		if tc.State() == StateClosed {
			return 0, io.EOF
		} else if time.Since(start) < tc.WriteTimeout {
			<-time.After(BusyWaitInterval)
			continue
		} else {
			if tc.Debug {
				tc.Logger.Info("Write", "", nil, "timed out writing buf %v", lalog.ByteArrayLogString(buf))
			}
			return 0, os.ErrDeadlineExceeded
		}
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.outputBuf = append(tc.outputBuf, buf...)
	// There is no need to wait for the output sequence number to catch up.
	return len(buf), nil
}

// slidingWindowFull returns true if the output sliding input is saturated, in
// which case the transmission control will wait before sending more data to
// the output transport.
func (tc *TransmissionControl) slidingWindowFull() bool {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	return tc.state < StateEstablished || len(tc.outputBuf) >= int(tc.MaxSlidingWindow) ||
		tc.inputAck < tc.outputSeq && tc.outputSeq-tc.inputAck >= tc.MaxSlidingWindow
}

// State returns the current state of the transmission control.
func (tc *TransmissionControl) State() State {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	s := tc.state
	return s
}

// WaitState blocks the caller until the transmission control reaches the
// specified state, or the context is cancelled.
// It returns true only if the state has been reached while the context is not
// cancelled.
func (tc *TransmissionControl) WaitState(ctx context.Context, want State) bool {
	for {
		if tc.State() == want {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(BusyWaitInterval):
			continue
		}
	}
}

func (tc *TransmissionControl) drainOutputToTransport() {
	if tc.Debug {
		tc.Logger.Info("drainOutputToTransport", "", nil, "starting now")
	}
	defer func() {
		if tc.Debug {
			tc.Logger.Info("drainOutputToTransport", "", nil, "returning and closing")
		}
		_ = tc.Close()
	}()
	for tc.State() < StateClosed {
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()

		if time.Since(instant.startTime) > instant.MaxLifetime {
			tc.Logger.Warning("drainOutputToTransport", "", nil, "closing due to exceeding max lifetime (start: %v, max: %v)", instant.startTime, instant.MaxLifetime)
			_ = tc.Close()
		} else if instant.state < StateEstablished {
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
					seg := Segment{
						ID:    tc.ID,
						Flags: FlagHandshakeSyn,
						Data:  instant.InitiatorSegmentData,
					}
					if tc.Debug {
						tc.Logger.Info("drainOutputToTransport", "", nil, "sending handshake, state: %v, seg: %+v", instant.state, seg)
					}
					_ = tc.writeToOutputTransport(seg)
					tc.mutex.Lock()
					tc.lastOutputSyn = time.Now()
					tc.ongoingRetransmissions++
					tc.mutex.Unlock()
				case StatePeerAck:
					// Got ack, send SYN + ACK.
					seg := Segment{ID: tc.ID, Flags: FlagHandshakeSyn | FlagHandshakeAck}
					if tc.Debug {
						tc.Logger.Info("drainOutputToTransport", "", nil, "handshake completed, sending syn+ack: %+v", seg)
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
						tc.Logger.Warning("drainOutputToTransport", "", nil, "handshake ack has got no response after multiple attempts, closing.")
						return
					}
					seg := Segment{ID: tc.ID, Flags: FlagHandshakeAck}
					if tc.Debug {
						tc.Logger.Info("drainOutputToTransport", "", nil, "sending handshake ack, state: %v, seg: %+v", instant.state, seg)
					}
					_ = tc.writeToOutputTransport(seg)
					tc.mutex.Lock()
					tc.lastOutputSyn = time.Now()
					tc.ongoingRetransmissions++
					tc.mutex.Unlock()
				}
			}
		} else if instant.state == StateEstablished && time.Since(instant.lastInputAck) > instant.RetransmissionInterval && instant.inputAck < instant.outputSeq {
			// Re-transmit the segments since the latest acknowledgement.
			tc.mutex.Lock()
			tc.ongoingRetransmissions++
			tc.mutex.Unlock()
			if tc.Debug {
				tc.Logger.Warning("drainOutputToTransport", "", nil,
					"re-transmitting, last input ack time: %+v, input ack: %+v, output seq: %+v, ongoing retransmissions: %v",
					instant.lastInputAck, instant.inputAck, instant.outputSeq, tc.ongoingRetransmissions)
			}
			if tc.ongoingRetransmissions >= tc.MaxRetransmissions {
				if tc.Debug {
					tc.Logger.Info("drainOutputToTransport", "", nil, "reached max retransmissions")
				}
				return
			}
			_ = tc.writeSegments(instant.inputSeq, instant.inputAck, instant.outputBuf[:instant.outputSeq-instant.inputAck], false)
			// Wait a short duration before the next transmission.
			select {
			case <-time.After(tc.SlidingWindowWaitDuration):
				continue
			case <-tc.context.Done():
				return
			}
		} else if time.Since(instant.lastInputAck) > tc.AckDelay && instant.lastAckOnlySeg.Before(instant.lastInputAck) && instant.inputSeq > 0 {
			// Send a delayed ack segment.
			emptySeg := Segment{
				ID:     tc.ID,
				SeqNum: instant.outputSeq,
				AckNum: instant.inputSeq,
				Data:   []byte{},
				Flags:  FlagAckOnly,
			}
			if tc.Debug {
				tc.Logger.Info("drainOutputToTransport", "", nil, "sending delayed ack: %+v", emptySeg)
			}
			_ = tc.writeToOutputTransport(emptySeg)
			tc.mutex.Lock()
			tc.lastAckOnlySeg = time.Now()
			tc.mutex.Unlock()
		} else if time.Since(instant.lastAckOnlySeg) > tc.KeepAliveInterval {
			// Send an empty segment for keep-alive.
			emptySeg := Segment{
				ID:     tc.ID,
				SeqNum: instant.outputSeq,
				AckNum: instant.inputSeq,
				Data:   []byte{},
				Flags:  FlagKeepAlive,
			}
			if tc.Debug {
				tc.Logger.Info("drainOutputToTransport", "", nil, "sending keep-alive: %+v", emptySeg)
			}
			_ = tc.writeToOutputTransport(emptySeg)
			tc.mutex.Lock()
			tc.lastAckOnlySeg = time.Now()
			tc.mutex.Unlock()
		} else if instant.state == StateEstablished && instant.inputAck < instant.outputSeq && instant.outputSeq-instant.inputAck >= instant.MaxSlidingWindow {
			// Wait for a short duration and retry when sliding window is full.
			if tc.Debug {
				tc.Logger.Info("drainOutputToTransport", "", nil,
					"wait due to saturated sliding window, output seq: %+v, input ack: %+v, max sliding window: %+v",
					instant.outputSeq, instant.inputAck, tc.MaxSlidingWindow)
			}
			select {
			case <-time.After(tc.SlidingWindowWaitDuration):
				continue
			case <-tc.context.Done():
				return
			}
		} else if instant.state == StateEstablished {
			// Send output segments starting with the latest sequence number.
			var toSend []byte
			if next := instant.outputSeq - instant.inputAck; int(next) < len(instant.outputBuf) {
				toSend = instant.outputBuf[next:]
			}
			if len(toSend) > int(tc.MaxSlidingWindow) {
				toSend = toSend[:int(tc.MaxSlidingWindow)]
			}
			if len(toSend) == 0 {
				// The output buffer has already been drained, there is no more
				// data to send for the time being.
				if instant.closeAfterDrained {
					return
				}
				select {
				case <-time.After(BusyWaitInterval):
					continue
				case <-tc.context.Done():
					return
				}
			} else {
				// The segment data can be empty. An empty segment is for
				// keep-alive.
				_ = tc.writeSegments(instant.inputSeq, instant.outputSeq, toSend, true)
				// Always clear the retransmission counter after a regular
				// transmission.
				tc.mutex.Lock()
				tc.ongoingRetransmissions = 0
				tc.mutex.Unlock()
			}
		}
	}
}

func (tc *TransmissionControl) writeSegments(ackInputSeq, seqNum uint32, buf []byte, increaseOutputSeq bool) uint32 {
	for i := 0; i < len(buf); i += tc.MaxSegmentLenExclHeader {
		// Split the buffer into individual segments maximum
		// MaxSegmentLenExclHeader bytes each.
		thisSeg := buf[i:]
		if len(thisSeg) > tc.MaxSegmentLenExclHeader {
			thisSeg = thisSeg[:tc.MaxSegmentLenExclHeader]
		}
		seg := Segment{
			ID:     tc.ID,
			SeqNum: seqNum + uint32(i),
			AckNum: ackInputSeq,
			Data:   thisSeg,
		}
		err := tc.writeToOutputTransport(seg)
		if err == nil {
			if increaseOutputSeq {
				// Increase the output sequence number with each successfully
				// written segment.
				tc.mutex.Lock()
				tc.outputSeq += uint32(len(thisSeg))
				tc.lastOutput = time.Now()
				tc.mutex.Unlock()
			}
		} else {
			return uint32(i)
		}
	}
	return uint32(len(buf))
}

func (tc *TransmissionControl) Read(buf []byte) (int, error) {
	if tc.State() == StateClosed {
		return 0, io.EOF
	}
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
			if tc.Debug {
				tc.Logger.Info("Read", "", nil, "returning to caller %d bytes %v", readLen, lalog.ByteArrayLogString(buf[:readLen]))
			}
			// Caller has got some data.
			return readLen, nil
		} else if tc.State() == StateClosed {
			return readLen, io.EOF
		} else if time.Since(start) < tc.ReadTimeout {
			// Wait for more input data to arrive and then retry.
			<-time.After(BusyWaitInterval)
		} else {
			if tc.Debug {
				tc.Logger.Info("Read", "", nil, "time out, want %d, got %d, got data: %v", len(buf), readLen, lalog.ByteArrayLogString(buf[:readLen]))
			}
			return readLen, os.ErrDeadlineExceeded
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
		_ = tc.Close()
	}()
	// Continuously read the bytes of inputBuf using the underlying transit.
	for tc.State() < StateClosed {
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
			tc.Logger.Warning("drainInputFromTransport", "", nil, "failed to decode the segment, header: %v, data: %v", segHeader, lalog.ByteArrayLogString(segData))
			segDataCtxCancel()
			continue
		}
		tc.mutex.Lock()
		instant := *tc
		tc.mutex.Unlock()
		if time.Since(instant.startTime) > instant.MaxLifetime {
			tc.Logger.Warning("drainInputFromTransport", "", nil, "closing due to exceeding max lifetime")
			_ = tc.Close()
		} else if instant.state < StateEstablished {
			if tc.Debug {
				tc.Logger.Info("drainInputFromTransport", "", nil, "handshake ongoing, received: %+v", seg)
			}
			if instant.Initiator {
				if instant.state == StateEmpty {
					// SYN was sent, expect ACK.
					if seg.Flags == FlagHandshakeAck {
						if tc.Debug {
							tc.Logger.Info("drainInputFromTransport", "", nil, "transition to StatePeerAck")
						}
						tc.mutex.Lock()
						tc.state = StatePeerAck
						tc.mutex.Unlock()
					} else {
						tc.Logger.Warning("drainInputFromTransport", "", nil, "expecting ack, got: %+v", seg)
						tc.mutex.Lock()
						tc.inputTransportErrors++
						tc.mutex.Unlock()
					}
				}
			} else {
				switch instant.state {
				case StateEmpty:
					// Expect SYN.
					if seg.Flags == FlagHandshakeSyn {
						if tc.Debug {
							tc.Logger.Info("drainInputFromTransport", "", nil, "transition to StateSynReceived")
						}
						tc.mutex.Lock()
						tc.state = StateSynReceived
						tc.mutex.Unlock()
					} else {
						tc.Logger.Warning("drainInputFromTransport", "", nil, "expecting syn, got: %+v", seg)
						tc.mutex.Lock()
						tc.inputTransportErrors++
						tc.mutex.Unlock()
					}
				default:
					// Expect SYN+ACK.
					if seg.Flags == FlagHandshakeSyn|FlagHandshakeAck {
						if tc.Debug {
							tc.Logger.Info("drainInputFromTransport", "", nil, "transition to StateEstablished")
						}
						tc.mutex.Lock()
						tc.ongoingRetransmissions = 0
						tc.state = StateEstablished
						tc.mutex.Unlock()
					} else {
						tc.Logger.Warning("drainInputFromTransport", "", nil, "expecting syn+ack, got: %+v", seg)
						tc.mutex.Lock()
						tc.inputTransportErrors++
						tc.mutex.Unlock()
					}
				}
			}
		} else {
			if seg.Flags.Has(FlagReset) {
				if tc.Debug {
					tc.Logger.Info("drainInputFromTransport", "", nil, "received a reset segment %+v", seg)
				}
				tc.mutex.Lock()
				// There should not be data in this segment though.
				tc.inputSeq = seg.SeqNum + uint32(len(seg.Data))
				tc.inputAck = seg.AckNum
				tc.mutex.Unlock()
				_ = tc.Close()
			} else if seg.Flags.Has(FlagHandshakeSyn) || seg.Flags.Has(FlagHandshakeAck) {
				if tc.Debug {
					tc.Logger.Info("drainInputFromTransport", "", nil, "ignored a handshake segments %+v after handshake is already over", seg)
				}
				tc.mutex.Lock()
				tc.inputTransportErrors++
				tc.mutex.Unlock()
			} else if tc.inputSeq == 0 || seg.SeqNum == tc.inputSeq {
				// Ensure the new segment is consecutive to the ones already
				// received. There is no selective acknowledgement going on here.
				tc.mutex.Lock()
				if seg.AckNum > tc.outputSeq || seg.AckNum < tc.inputAck {
					// This will be (hopefully) resolved by a retransmission.
					tc.Logger.Warning("drainInputFromTransport", "", nil, "received segment %+v with an out-of-range ack numbers, my output seq: %d", seg, tc.outputSeq)
					tc.inputTransportErrors++
				} else {
					if tc.Debug {
						tc.Logger.Info("drainInputFromTransport", "", nil, "received a good segment %+v", seg)
					}
					tc.inputSeq = seg.SeqNum + uint32(len(seg.Data))
					// Pop the acknowledged bytes from the output buffer.
					tc.outputBuf = tc.outputBuf[seg.AckNum-tc.inputAck:]
					tc.inputAck = seg.AckNum
					tc.lastInputAck = time.Now()
					tc.inputBuf = append(tc.inputBuf, seg.Data...)
					tc.inputTransportErrors = 0
				}
				tc.mutex.Unlock()
			} else {
				tc.mutex.Lock()
				// This will be (hopefully) resolved by a retransmission.
				tc.Logger.Warning("drainInputFromTransport", "", nil, "received out-of-sequence segment %+v, my input seq: %d", seg, tc.inputSeq)
				tc.inputTransportErrors++
				// In a special case, if the other TC is out of sync with the
				// segment sequence number but still comes with a valid
				// ack number, then make use of the ack number.
				// This also helps to bring two TCs back in sync when they
				// disagree on each other's ack number and seq number
				// simultaneously.
				if seg.AckNum <= tc.outputSeq && seg.AckNum > tc.inputAck {
					tc.Logger.Warning("drainInputFromTransport", "", nil, "advancing input ack from %d to %d of the out-of-sequence segment", tc.inputAck, seg.AckNum)
					tc.outputBuf = tc.outputBuf[seg.AckNum-tc.inputAck:]
					tc.inputAck = seg.AckNum
					tc.lastInputAck = time.Now()
				}
				tc.mutex.Unlock()
			}
		}
		segDataCtxCancel()
	}
}

func (tc *TransmissionControl) DumpState() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.Logger.Warning("DumpState", "", nil, "\n"+
		"state: %d\tlast output syn: %v\n"+
		"input seq: %v\tinput ack: %v\tlast input ack: %v\tinput buf: %v\n"+
		"output seq: %v\tlast output: %v\tlast ack-only seg: %v\toutput buf: %v\n"+
		"ongoing retrans: %d\tinput transport errs: %d\toutput transport errs: %d\n",
		tc.state, tc.lastOutputSyn,
		tc.inputSeq, tc.inputAck, tc.lastInputAck, lalog.ByteArrayLogString(tc.inputBuf),
		tc.outputSeq, tc.lastOutput, tc.lastAckOnlySeg, lalog.ByteArrayLogString(tc.outputBuf),
		tc.ongoingRetransmissions, tc.inputTransportErrors, tc.outputTransportErrors,
	)
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
// If the output transport is experience an exceeding exceeding number of IO
// errors then the transmission controll will be stopped.
func (tc *TransmissionControl) writeToOutputTransport(seg Segment) error {
	if tc.Debug {
		tc.Logger.Info("", "", nil, "writing to output transport %+v", seg)
	}
	_, err := tc.OutputTransport.Write(seg.Packet())
	if tc.OutputSegmentCallback != nil {
		tc.OutputSegmentCallback(seg)
	}
	tc.mutex.Lock()
	if err == nil {
		tc.outputTransportErrors = 0
		tc.mutex.Unlock()
		return nil
	} else if err == io.EOF {
		tc.mutex.Unlock()
		_ = tc.Close()
		return nil
	} else {
		tc.outputTransportErrors++
		gotErrs := tc.outputTransportErrors
		tc.mutex.Unlock()
		if gotErrs >= tc.MaxTransportErrors {
			tc.Logger.Warning("writeToOutputTransport", "", nil, "closing due to exceedingly many transport errors")
			_ = tc.Close()
		}
		return err
	}
}

// readFromInputTransport reads the desired number of bytes from the input
// transport and returns the IO error (if any).
// The function briefly locks the transmission control mutex, therefore the
// caller must not hold the mutex.
// If the input transport is experience an exceeding exceeding number of IO
// errors then the transmission controll will be stopped.
func (tc *TransmissionControl) readFromInputTransport(ctx context.Context, totalLen int) ([]byte, error) {
	data, err := readInput(ctx, tc.InputTransport, totalLen)
	if err == nil {
		return data, err
	} else if err == io.EOF {
		_ = tc.Close()
		return data, nil
	} else if err == context.Canceled {
		// The caller (TC's internal function) cancelled the context, this is
		// not a transport error.
		return data, err
	} else {
		tc.mutex.Lock()
		tc.inputTransportErrors++
		gotErrs := tc.inputTransportErrors
		tc.mutex.Unlock()
		if gotErrs >= tc.MaxTransportErrors {
			tc.Logger.Warning("readFromInputTransport", "", nil, "closing due to exceedingly many transport errors")
			_ = tc.Close()
		}
		return data, err
	}
}

// closeAfterDrained irreversibly sets an internal flag to signal the
// transmission control to terminate/close after completely draining the output
// buffer to its transport.
func (tc *TransmissionControl) CloseAfterDrained() {
	if tc.Debug {
		tc.Logger.Info("CloseAfterDrained", "", nil, "will close the TC after emptying output buffer")
	}
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.closeAfterDrained = true
}

// Close immediately terminates/closes this transmission control, and writes a
// single output segment to instruct the peer to terminate itself as well.
func (tc *TransmissionControl) Close() error {
	tc.mutex.Lock()
	if tc.state == StateClosed {
		tc.mutex.Unlock()
		return nil
	}
	if tc.Debug {
		tc.Logger.Info("Close", "", nil, "terminating now")
	}
	tc.state = StateClosed
	tc.mutex.Unlock()
	tc.cancelFun()
	// Both input and output loops have quit at this point.
	// Send an RST segment to the peer.
	if err := tc.writeToOutputTransport(Segment{
		ID:     tc.ID,
		Flags:  FlagReset,
		SeqNum: tc.outputSeq,
		AckNum: tc.inputSeq,
		Data:   []byte{},
	}); err != nil {
		tc.Logger.Info("Close", "", err, "failed to write reset segment")
		// There's nothing more for this TC to do.
	}
	return nil
}

// LocalAddr always returns nil.
func (tc *TransmissionControl) LocalAddr() net.Addr { return nil }

// RemoteAddr always returns nil.
func (tc *TransmissionControl) RemoteAddr() net.Addr { return nil }

// SetDeadline always returns nil.
func (tc *TransmissionControl) SetDeadline(t time.Time) error { return nil }

// SetReadDeadline always returns nil.
func (tc *TransmissionControl) SetReadDeadline(t time.Time) error { return nil }

// SetWriteDeadline always returns nil.
func (tc *TransmissionControl) SetWriteDeadline(t time.Time) error { return nil }

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
