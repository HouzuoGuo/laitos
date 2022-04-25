package dnspe

import (
	"context"
	"io"
	"time"

	"golang.org/x/time/rate"
)

const (
	StarvationRetryInterval = 1 * time.Second
	MaxBufLen               = 65536
	MaxIOErrorRetry         = 5
)

type TransmissionControl struct {
	io.ReadCloser
	io.WriteCloser

	MaxInTransitLen int
	MaxSegmentLen   int
	InputTransport  io.Writer
	OutputTransport io.Reader

	context   context.Context
	cancelFun func()

	inTransitLimiter *rate.Limiter
	inputBuf         []byte
	inputErr         chan error
	outputBuf        []byte
	outputErr        chan error
}

func (tc *TransmissionControl) Start(ctx context.Context) {
	tc.inTransitLimiter = rate.NewLimiter(rate.Inf, tc.MaxInTransitLen)
	tc.inputErr = make(chan error, 1)
	tc.outputErr = make(chan error, 1)
	tc.context, tc.cancelFun = context.WithCancel(ctx)
	go tc.DrainInput()
	go tc.DrainOutput()
}

func (tc *TransmissionControl) Read(buf []byte) (int, error) {
	// Drain received data buffer to caller.
	readLen := copy(buf, tc.outputBuf)
	if len(tc.outputBuf) > 0 {
		tc.inTransitLimiter.AllowN(time.Now(), readLen)
	}
	// Remove received portion from the internal buffer.
	tc.outputBuf = tc.outputBuf[readLen:]
	return readLen, nil
}

func (tc *TransmissionControl) Write(buf []byte) (int, error) {
	// Wait for in-transit buffer to replenish.
	if err := tc.inTransitLimiter.WaitN(context.Background(), len(buf)); err != nil {
		return 0, err
	}
	// Drain input into the internal send buffer.
	tc.inputBuf = append(tc.inputBuf, buf...)
	return len(buf), nil
}

func (tc *TransmissionControl) DrainInput() {
	// Continuously send the bytes of inputBuf using the underlying transit.
	for {
		toSend := tc.inputBuf
		if len(toSend) == 0 {
			select {
			case <-time.After(StarvationRetryInterval):
				continue
			case <-tc.context.Done():
				return
			}
		}
		if len(tc.inputBuf) >= tc.MaxSegmentLen {
			toSend = toSend[:tc.MaxSegmentLen]
		}
		_, err := tc.InputTransport.Write(toSend)
		// Remove sent portion from the internal buffer.
		tc.inputBuf = tc.inputBuf[len(toSend):]
		if err != nil {
			// TODO: maybe retry?
			tc.inputErr <- err
			return
		}
	}
}

func (tc *TransmissionControl) DrainOutput() {
	// Continuously read the bytes of outputBuf using the underlying transit.
	for {
		recvBuf := make([]byte, tc.MaxSegmentLen)
		// TODO: maybe retry?
		recvN := make(chan int, 1)
		recvErr := make(chan error, 1)
		go func() {
			// TODO: make sure the read timeout is consistent with starvation retry interval, which may be too short at 1sec.
			n, err := tc.OutputTransport.Read(recvBuf)
			recvN <- n
			recvErr <- err
		}()
		select {
		case <-tc.context.Done():
			return
		case <-time.After(StarvationRetryInterval):
			continue
		case n := <-recvN:
			tc.outputBuf = append(tc.outputBuf, recvBuf[:n]...)
		}
	}
}

func (tc *TransmissionControl) Close() error {
	tc.cancelFun()
	return nil
}
