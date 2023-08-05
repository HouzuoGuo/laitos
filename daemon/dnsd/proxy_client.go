package dnsd

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"github.com/miekg/dns"
)

// MaxUpstreamSegmentLength returns the maximum segment length appropriate for
// the upstream traffic direction for the DNS host name.
func MaxUpstreamSegmentLength(dnsHostName string) int {
	// The maximum DNS host name is 253 characters.
	// At present the encoding efficiency is ~68% according to TestCompression.
	// Though for some reason the actual efficiency seems lower.
	approxLen := float64(253-2-4-len(dnsHostName)) * 0.61
	ret := int(approxLen)
	if ret < 0 {
		return 0
	}
	return ret
}

// MaxDownstreamSegmentLengthTXT returns the maximum segment length appropriate
// for the downstream traffic direction for the DNS host name.
func MaxDownstreamSegmentLengthTXT(dnsHostName string) int {
	// Determined by trial and error.
	return 820 - MaxUpstreamSegmentLength(dnsHostName)
}

// ProxiedConnection handles an individual proxy connection to transport
// data between local transmission control and the one on the remote DNS proxy
// server.
type ProxiedConnection struct {
	dnsHostName       string
	dnsConfig         *dns.ClientConfig
	dropPercentage    int
	debug             bool
	enableTXTRequests bool

	in      net.Conn
	tc      *tcpoverdns.TransmissionControl
	buf     *tcpoverdns.SegmentBuffer
	context context.Context
	logger  *lalog.Logger
}

// Start configures and then starts the transmission control on local side, and
// spawns a background goroutine to transport segments back and forth using
// DNS queries.
// The function returns when the local transmission control transitions to the
// established state, or an error.
func (conn *ProxiedConnection) Start() error {
	conn.logger.Info("", nil, "start transporting data over DNS")
	conn.buf = tcpoverdns.NewSegmentBuffer(conn.logger, conn.debug, conn.tc.MaxSegmentLenExclHeader)
	// Absorb outgoing segments into the outgoing backlog.
	conn.tc.OutputSegmentCallback = conn.buf.Absorb
	conn.tc.Start(conn.context)
	// Start transporting segments back and forth.
	go conn.transportLoop()
	if !conn.tc.WaitState(conn.context, tcpoverdns.StateEstablished) {
		return fmt.Errorf("local transmission control failed to complete handshake")
	}
	return nil
}

func (conn *ProxiedConnection) sendDNSQuery(countHostNameLabels int, questionName string) (tcpoverdns.Segment, error) {
	malformedSegment := tcpoverdns.Segment{Flags: tcpoverdns.FlagMalformed}
	if len(questionName) < 3 {
		return malformedSegment, errors.New("the input query name is too short")
	}
	if questionName[len(questionName)-1] != '.' {
		questionName += "."
	}
	client := new(dns.Client)
	query := new(dns.Msg)
	query.RecursionDesired = true
	query.SetEdns0(EDNSBufferSize, false)

	if conn.enableTXTRequests {
		query.SetQuestion(questionName, dns.TypeTXT)
		response, _, err := client.Exchange(query, fmt.Sprintf("%s:%s", conn.dnsConfig.Servers[0], conn.dnsConfig.Port))
		if err != nil {
			return malformedSegment, err
		}
		if len(response.Answer) == 0 {
			return malformedSegment, errors.New("the DNS query did not receive a response")
		}
		if txtResponse, ok := response.Answer[0].(*dns.TXT); ok {
			if rand.Intn(100) < conn.dropPercentage {
				return malformedSegment, errors.New("dropped for testing")
			}
			return tcpoverdns.SegmentFromDNSText(txtResponse.Txt), nil
		} else {
			return malformedSegment, fmt.Errorf("the response answer %v is not a TXT", response.Answer[0])
		}
	} else {
		query.SetQuestion(questionName, dns.TypeA)
		response, _, err := client.Exchange(query, fmt.Sprintf("%s:%s", conn.dnsConfig.Servers[0], conn.dnsConfig.Port))
		if err != nil {
			return malformedSegment, err
		}
		if len(response.Answer) == 0 {
			return malformedSegment, errors.New("the DNS query did not receive a response")
		}
		if cnameResp, ok := response.Answer[0].(*dns.CNAME); ok {
			if rand.Intn(100) < conn.dropPercentage {
				return malformedSegment, errors.New("dropped for testing")
			}
			return tcpoverdns.SegmentFromDNSName(countHostNameLabels, cnameResp.Target), nil
		} else {
			return malformedSegment, fmt.Errorf("the response answer %v is not a CNAME", response.Answer[0])
		}
	}
}

func (conn *ProxiedConnection) transportLoop() {
	countHostNameLabels := CountNameLabels(conn.dnsHostName)
	defer func() {
		// Linger briefly, then send the last segment. The brief waiting
		// time allows the TC to transition to the closed state.
		time.Sleep(5 * time.Second)
		final, exists := conn.buf.Latest()
		if exists && final.Flags != 0 {
			if _, err := conn.sendDNSQuery(countHostNameLabels, final.DNSName(fmt.Sprintf("%c", ProxyPrefix), conn.dnsHostName)); err != nil {
				conn.logger.Warning("", err, "failed to send the final segment")
			}
		}
		conn.logger.Info("", nil, "DNS data transport finished, the final segment was: %v", final)
	}()
	for {
		if conn.tc.State() == tcpoverdns.StateClosed {
			return
		}
		var replySeg, outgoingSeg, nextInBacklog tcpoverdns.Segment
		var exists bool
		var err error
		begin := time.Now()
		// Pop a segment.
		outgoingSeg, exists = conn.buf.Pop()
		if !exists {
			// Wait for a segment.
			goto busyWaitInterval
		}
		// Turn the segment into a DNS query and send the query out
		// (data.data.data.example.com).
		replySeg, err = conn.sendDNSQuery(countHostNameLabels, outgoingSeg.DNSName(fmt.Sprintf("%c", ProxyPrefix), conn.dnsHostName))
		conn.logger.Info(fmt.Sprint(conn.tc.ID), nil, "sent over DNS query in %dms: %+v", time.Since(begin).Milliseconds(), outgoingSeg)
		if err != nil {
			conn.logger.Warning(fmt.Sprint(conn.tc.ID), err, "failed to send output segment %v", outgoingSeg)
			conn.tc.IncreaseTimingInterval()
			goto busyWaitInterval
		}
		if conn.debug {
			conn.logger.Info(fmt.Sprint(conn.tc.ID), nil, "DNS query response segment: %v", replySeg)
		}
		if replySeg.Flags.Has(tcpoverdns.FlagMalformed) {
			// Slow down a notch in the presence of transmission error.
			conn.tc.IncreaseTimingInterval()
			goto busyWaitInterval
		} else if replySeg.Flags.Has(tcpoverdns.FlagKeepAlive) {
			// Slow down a notch in in the absence of data transmission.
			conn.tc.IncreaseTimingInterval()
		} else {
			if replySeg.SeqNum >= conn.tc.InputSeq() && len(replySeg.Data) > 0 {
				// Decrease the timing interval with each input segment that
				// carries data. This helps to temporarily increase the
				// throughput.
				conn.tc.DecreaseTimingInterval()
			}
		}
		if _, err := conn.in.Write(replySeg.Packet()); err != nil {
			conn.logger.Warning(fmt.Sprint(conn.tc.ID), err, "failed to receive input segment %v", replySeg)
			conn.tc.IncreaseTimingInterval()
			goto busyWaitInterval
		}
		// If data was transported in either direction, then do not wait for
		// the keep-alive interval to speed up the transmission.
		nextInBacklog, exists = conn.buf.First()
		if exists && len(nextInBacklog.Data) > 0 || !nextInBacklog.Flags.Has(tcpoverdns.FlagKeepAlive) {
			continue
		}
		if len(replySeg.Data) > 0 && !replySeg.Flags.Has(tcpoverdns.FlagKeepAlive) {
			continue
		}
		// Wait slightly longer than the keep-alive interval.
		select {
		case <-time.After(time.Duration(conn.tc.LiveTimingInterval().KeepAliveInterval * 8 / 7)):
			continue
		case <-conn.context.Done():
			return
		}
	busyWaitInterval:
		select {
		case <-time.After(tcpoverdns.BusyWaitInterval):
			continue
		case <-conn.context.Done():
			return
		}
	}
}
