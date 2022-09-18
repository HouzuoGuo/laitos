package dnsclient

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/tcpoverdns"
	"github.com/miekg/dns"
)

// OptimalSegLen returns the optimal segment length appropriate for the DNS host
// name.
func OptimalSegLen(dnsHostName string) int {
	// The maximum DNS host name is 253 characters.
	// At present the encoding efficiency is ~60% at the worst case scenario.
	approxLen := float64(250-len(dnsHostName)) * 0.60
	ret := int(approxLen)
	if ret < 0 {
		return 0
	}
	return ret
}

// ProxiedConnection handles an individual proxy connection to transport
// data between local transmission control and the one on the remote DNS proxy
// server.
type ProxiedConnection struct {
	dnsHostName    string
	dnsConfig      *dns.ClientConfig
	dropPercentage int
	debug          bool

	in      net.Conn
	tc      *tcpoverdns.TransmissionControl
	buf     *tcpoverdns.SegmentBuffer
	context context.Context
	logger  lalog.Logger
}

// Start configures and then starts the transmission control on local side, and
// spawns a background goroutine to transport segments back and forth using
// DNS queries.
// The function returns when the local transmission control transitions to the
// established state, or an error.
func (conn *ProxiedConnection) Start() error {
	conn.logger.Info("", nil, "start transporting data over DNS")
	conn.buf = tcpoverdns.NewSegmentBuffer(conn.logger, conn.tc.Debug, conn.tc.MaxSegmentLenExclHeader)
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

func (conn *ProxiedConnection) lookupCNAME(queryName string) (string, error) {
	if len(queryName) < 3 {
		return "", errors.New("the input query name is too short")
	}
	if queryName[len(queryName)-1] != '.' {
		queryName += "."
	}
	client := new(dns.Client)
	query := new(dns.Msg)
	query.RecursionDesired = true
	query.SetQuestion(queryName, dns.TypeA)
	query.SetEdns0(dnsd.EDNSBufferSize, false)
	response, _, err := client.Exchange(query, fmt.Sprintf("%s:%s", conn.dnsConfig.Servers[0], conn.dnsConfig.Port))
	if err != nil {
		return "", err
	}
	if len(response.Answer) == 0 {
		return "", errors.New("the DNS query did not receive a response")
	}
	if cname, ok := response.Answer[0].(*dns.CNAME); ok {
		if rand.Intn(100) < conn.dropPercentage {
			return "", errors.New("dropped for testing")
		}
		return cname.Target, nil
	} else {
		return "", fmt.Errorf("the response answer %v is not a CNAME", response.Answer[0])
	}
}

func (conn *ProxiedConnection) transportLoop() {
	defer func() {
		// Linger briefly, then send the last segment. The brief waiting
		// time allows the TC to transition to the closed state.
		time.Sleep(5 * time.Second)
		final, exists := conn.buf.Latest()
		if exists && final.Flags != 0 {
			if _, err := conn.lookupCNAME(final.DNSName(fmt.Sprintf("%c", dnsd.ProxyPrefix), conn.dnsHostName)); err != nil {
				conn.logger.Warning("", err, "failed to send the final segment")
			}
		}
		conn.logger.Info("", nil, "DNS data transport finished, the final segment was: %v", final)
	}()
	countHostNameLabels := dnsd.CountNameLabels(conn.dnsHostName)
	for {
		if conn.tc.State() == tcpoverdns.StateClosed {
			return
		}
		var incomingSeg, outgoingSeg, nextInBacklog tcpoverdns.Segment
		var exists bool
		var cname string
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
		cname, err = conn.lookupCNAME(outgoingSeg.DNSName(fmt.Sprintf("%c", dnsd.ProxyPrefix), conn.dnsHostName))
		conn.logger.Info(fmt.Sprint(conn.tc.ID), nil, "sent over DNS query in %dms: %+v", time.Since(begin).Milliseconds(), outgoingSeg)
		if err != nil {
			conn.logger.Warning(fmt.Sprint(conn.tc.ID), err, "failed to send output segment %v", outgoingSeg)
			conn.tc.IncreaseTimingInterval()
			goto busyWaitInterval
		}
		// Decode a segment from DNS query response and give it to the local
		// TC.
		incomingSeg = tcpoverdns.SegmentFromDNSName(countHostNameLabels, cname)
		if conn.debug {
			conn.logger.Info(fmt.Sprint(conn.tc.ID), nil, "DNS query response segment: %v", incomingSeg)
		}
		if !incomingSeg.Flags.Has(tcpoverdns.FlagMalformed) {
			if incomingSeg.Flags.Has(tcpoverdns.FlagKeepAlive) {
				// Increase the timing interval interval with each input
				// segment that does not carry data.
				conn.tc.IncreaseTimingInterval()
			} else {
				// Decrease the timing interval with each input segment that
				// carries data. This helps to temporarily increase the
				// throughput.
				conn.tc.DecreaseTimingInterval()
			}
			if _, err := conn.in.Write(incomingSeg.Packet()); err != nil {
				conn.logger.Warning(fmt.Sprint(conn.tc.ID), err, "failed to receive input segment %v", incomingSeg)
				conn.tc.IncreaseTimingInterval()
				goto busyWaitInterval
			}
		}
		// If the next output segment carries useful data or flags, then
		// send it out without delay.
		nextInBacklog, exists = conn.buf.First()
		if exists && len(nextInBacklog.Data) > 0 || nextInBacklog.Flags != 0 {
			continue
		}
		// If the input segment carried useful data, then shorten the
		// waiting interval. Transmission control should be sending out an
		// acknowledgement fairly soon.
		if len(incomingSeg.Data) > 0 && !incomingSeg.Flags.Has(tcpoverdns.FlagKeepAlive) {
			select {
			case <-time.After(time.Duration(conn.tc.LiveTimingInterval().AckDelay * 8 / 7)):
				continue
			case <-conn.context.Done():
				return
			}
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
