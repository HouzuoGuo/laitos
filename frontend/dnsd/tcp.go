package dnsd

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/global"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// FIXME: avoid opening new TCP connection for every query
func (dnsd *DNSD) HandleTCPQuery(clientConn net.Conn) {
	defer clientConn.Close()
	// Check address against rate limit
	clientIP := clientConn.RemoteAddr().String()[:strings.LastIndexByte(clientConn.RemoteAddr().String(), ':')]
	if !dnsd.RateLimit.Add(clientIP, true) {
		return
	}
	// Check address against allowed IP prefixes
	var prefixOK bool
	for _, prefix := range dnsd.AllowQueryIPPrefixes {
		if strings.HasPrefix(clientIP, prefix) {
			prefixOK = true
			break
		}
	}
	if !prefixOK {
		dnsd.Logger.Warningf("HandleTCPQuery", clientIP, nil, "client IP is not allowed to query")
		return
	}
	// Read query length
	clientConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
	queryLenBuf := make([]byte, 2)
	_, err := clientConn.Read(queryLenBuf)
	if err != nil {
		dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read query length from client")
		return
	}
	queryLen := int(queryLenBuf[0])*256 + int(queryLenBuf[1])
	// Read query
	if queryLen > MaxPacketSize || queryLen < 1 {
		dnsd.Logger.Warningf("HandleTCPQuery", clientIP, nil, "bad query length from client")
		return
	}
	queryBuf := make([]byte, queryLen)
	_, err = clientConn.Read(queryBuf)
	if err != nil {
		dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read query from client")
		return
	}
	domainName := ExtractDomainName(queryBuf)
	// Formulate response
	var responseLen int
	var responseLenBuf []byte
	var responseBuf []byte
	var doForward bool
	if len(domainName) == 0 {
		// If I cannot figure out what domain is from the query, simply forward it without much concern.
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "handle non-name query")
		doForward = true
	} else {
		// This is a domain name query, check the name against black list and then forward.
		if dnsd.NamesAreBlackListed(domainName) {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "handle black-listed domain \"%s\"", domainName[0])
			responseBuf = RespondWith0(queryBuf)
			responseLen = len(responseBuf)
			responseLenBuf = make([]byte, 2)
			responseLenBuf[0] = byte(responseLen / 256)
			responseLenBuf[1] = byte(responseLen % 256)
		} else {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "handle domain \"%s\"", domainName[0])
			doForward = true
		}
	}
	// If queried domain is not black listed, forward the query to forwarder.
	if doForward {
		myForwarder, err := net.DialTimeout("tcp", dnsd.TCPForwardTo, IOTimeoutSec*time.Second)
		if err != nil {
			dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to connect to forwarder")
			return
		}
		defer myForwarder.Close()
		// Send original query to forwarder without modification
		myForwarder.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err = myForwarder.Write(queryLenBuf); err != nil {
			dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to write length to forwarder")
			return
		} else if _, err = myForwarder.Write(queryBuf); err != nil {
			dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to write query to forwarder")
			return
		}
		// Retrieve forwarder's response
		responseLenBuf = make([]byte, 2)
		if _, err = myForwarder.Read(responseLenBuf); err != nil {
			dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read length from forwarder")
			return
		}
		responseLen = int(responseLenBuf[0])*256 + int(responseLenBuf[1])
		if responseLen > MaxPacketSize || responseLen < 1 {
			dnsd.Logger.Warningf("HandleTCPQuery", clientIP, nil, "bad response length from forwarder")
			return
		}
		responseBuf = make([]byte, responseLen)
		if _, err = myForwarder.Read(responseBuf); err != nil {
			dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to read response from forwarder")
			return
		}
	}
	// Send response to my client
	if _, err = clientConn.Write(responseLenBuf); err != nil {
		dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to answer length to client")
		return
	} else if _, err = clientConn.Write(responseBuf); err != nil {
		dnsd.Logger.Warningf("HandleTCPQuery", clientIP, err, "failed to answer to client")
		return
	}
	return
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon to listen on TCP port only. Block caller.
*/
func (dnsd *DNSD) StartAndBlockTCP() error {
	listenAddr := fmt.Sprintf("%s:%d", dnsd.TCPListenAddress, dnsd.TCPListenPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	dnsd.Logger.Printf("StartAndBlockTCP", listenAddr, nil, "going to listen for queries")
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		clientConn, err := listener.Accept()
		if err != nil {
			return err
		}
		go dnsd.HandleTCPQuery(clientConn)
	}

}

// Run unit tests on DNS TCP daemon. See TestDNSD_StartAndBlockTCP for daemon setup.
func TestTCPQueries(dnsd *DNSD, t *testing.T) {
	// Prevent daemon from listening to UDP queries in this TCP test case
	dnsd.UDPListenPort = 0
	udpListenPort := dnsd.UDPListenPort
	dnsd.UDPListenPort = 0
	defer func() {
		dnsd.UDPListenPort = udpListenPort
	}()
	// Server should start within two seconds
	go func() {
		if err := dnsd.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	packetBuf := make([]byte, MaxPacketSize)
	success := 0
	// Try to reach rate limit
	for i := 0; i < 40; i++ {
		go func() {
			clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(dnsd.TCPListenPort))
			if err != nil {
				t.Fatal(err)
			}
			defer clientConn.Close()
			if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := clientConn.Write(githubComTCPQuery); err != nil {
				t.Fatal(err)
			}
			resp, err := ioutil.ReadAll(clientConn)
			if err == nil && len(resp) > 50 {
				success++
			}
		}()
	}
	// Wait out rate limit
	time.Sleep(RateLimitIntervalSec * time.Second)
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Blacklist github and see if query gets a black hole response
	dnsd.BlackList["github.com"] = struct{}{}
	// This test is flaky and I do not understand why, is it throttled by google dns?
	var blackListSuccess bool
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(dnsd.TCPListenPort))
		if err != nil {
			continue
		}
		if err := clientConn.SetDeadline(time.Now().Add((RateLimitIntervalSec - 1) * time.Second)); err != nil {
			continue
			clientConn.Close()
		}
		if _, err := clientConn.Write(githubComTCPQuery); err != nil {
			continue
			clientConn.Close()
		}
		respLen, err := clientConn.Read(packetBuf)
		if err != nil {
			continue
			clientConn.Close()
		}
		clientConn.Close()
		if bytes.Index(packetBuf[:respLen], BlackHoleAnswer) != -1 {
			blackListSuccess = true
			break
		}
	}
	if !blackListSuccess {
		t.Fatal("did not answer to blacklist domain")
	}
}
