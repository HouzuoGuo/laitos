package snmpd

import (
	"bufio"
	"bytes"
	"crypto/subtle"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/snmpd/snmp"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

const (
	IOTimeoutSec         = 60   // IOTimeoutSec is the number of seconds to tolerate for network IO operations.
	RateLimitIntervalSec = 1    // RateLimitIntervalSec is the interval for rate limit calculation.
	MaxPacketSize        = 1500 // MaxPacketSize is the maximum acceptable UDP packet size. SNMP requests are small.
)

type Daemon struct {
	Address    string `json:"Address"`    // Address to listen on, e.g. 0.0.0.0 to listen on all network interfaces.
	Port       int    `json:"Port"`       // Port to listen on, by default SNMP uses port 161.
	PerIPLimit int    `json:"PerIPLimit"` // PerIPLimit is approximately how many requests are allowed from an IP within a designated interval.

	/*
		CommunityName is a password-like string that grants access to all SNMP nodes. Be aware that it is transmitted in
		plain text due to protocol limitation.
	*/
	CommunityName string `json:"CommunityName"`

	udpServer *common.UDPServer
}

// Initialise validates configuration and initialises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.Port == 0 {
		daemon.Port = 161
	}
	if daemon.PerIPLimit < 1 {
		/*
			By default, allow retrieval of all SNMP nodes within the interval. Due to protocol design, SNMP client often
			needs to make more than one request per OID.
		*/
		daemon.PerIPLimit = 3 * len(snmp.OIDSuffixList)
	}
	if len(daemon.CommunityName) < 6 {
		return fmt.Errorf("snmpd.Initialise: CommunityName must be at least 6 characters long")
	}
	daemon.udpServer = &common.UDPServer{
		ListenAddr:  daemon.Address,
		ListenPort:  daemon.Port,
		AppName:     "snmpd",
		App:         daemon,
		LimitPerSec: daemon.PerIPLimit,
	}
	daemon.udpServer.Initialise()
	return nil
}

// StartAndBlock starts UDP listener to serve SNMP clients. You may call this function only after having called Initialise().
func (daemon *Daemon) StartAndBlock() error {
	return daemon.udpServer.StartAndBlock()
}

// GetUDPStatsCollector returns the stats collector that counts and times UDP conversations.
func (daemon *Daemon) GetUDPStatsCollector() *misc.Stats {
	return common.SNMPStats
}

// HandleUDPClient converses
func (daemon *Daemon) HandleUDPClient(logger lalog.Logger, clientIP string, client *net.UDPAddr, reqPacket []byte, srv *net.UDPConn) {
	reader := bufio.NewReader(bytes.NewReader(reqPacket))
	// Parse the input packet
	packet := snmp.Packet{}
	if err := packet.ReadFrom(reader); err != nil {
		logger.Warning("HandleUDPClient", clientIP, err, "failed to parse request packet")
		return
	}
	// Validate community name i.e. password
	if subtle.ConstantTimeCompare([]byte(packet.CommunityName), []byte(daemon.CommunityName)) != 1 {
		logger.Info("HandleUDPClient", clientIP, nil, "incorrect community name")
		return
	}
	// Process the request
	var resp []byte
	var err error
	switch packet.PDU {
	case snmp.PDUGetNextRequest:
		baseOID := packet.Structure.(snmp.GetNextRequest).BaseOID
		nextOID, endOfMibView := snmp.GetNextNode(baseOID)
		nextNodeFun, exists := snmp.GetNode(nextOID)
		if !exists {
			logger.Warning("HandleUDPClient", clientIP, nil, "failed to retrieve OID %v, this is a programming error.", nextOID)
			return
		}
		nodeValue := nextNodeFun()
		packet.Structure = snmp.GetResponse{
			RequestedOID:   nextOID,
			Value:          nodeValue,
			NoSuchInstance: false,
			EndOfMIBView:   endOfMibView,
		}
		if strBytes, isByteArray := nodeValue.([]byte); isByteArray {
			logger.Info("HandleUDPClient", clientIP, nil, "GetNext OID %v = (%v) %s", baseOID, nextOID, strBytes)
		} else {
			logger.Info("HandleUDPClient", clientIP, nil, "GetNext OID %v = (%v) %v", baseOID, nextOID, nodeValue)
		}
	case snmp.PDUGetRequest:
		requestedOID := packet.Structure.(snmp.GetRequest).RequestedOID
		nextNodeFun, exists := snmp.GetNode(requestedOID)
		if exists {
			nodeValue := nextNodeFun()
			packet.Structure = snmp.GetResponse{
				RequestedOID:   requestedOID,
				Value:          nodeValue,
				NoSuchInstance: false,
				EndOfMIBView:   false,
			}
			if strBytes, isByteArray := nodeValue.([]byte); isByteArray {
				logger.Info("HandleUDPClient", clientIP, nil, "Get OID %v = %s", requestedOID, strBytes)
			} else {
				logger.Info("HandleUDPClient", clientIP, nil, "Get OID %v = %v", requestedOID, nodeValue)
			}
		} else {
			packet.Structure = snmp.GetResponse{
				RequestedOID:   requestedOID,
				Value:          nil,
				NoSuchInstance: true,
				EndOfMIBView:   false,
			}
			logger.Info("HandleUDPClient", clientIP, nil, "Get OID %v = NoSuchInstance", requestedOID)
		}
	default:
		logger.Info("HandleUDPClient", clientIP, nil, "unknown PDU %d", packet.PDU)
		return
	}
	packet.PDU = snmp.PDUGetResponse
	resp, err = packet.Encode()
	if err != nil {
		logger.Warning("HandleUDPClient", clientIP, err, "failed to encode response")
		return
	}
	if err := srv.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
		logger.Warning("HandleUDPClient", clientIP, err, "failed to answer to client")
		return
	}
	if _, err = srv.WriteTo(resp, client); err != nil {
		logger.Warning("HandleUDPClient", clientIP, err, "failed to answer to client")
		return
	}
}

// Stop closes server listener so that it ceases to process incoming requests.
func (daemon *Daemon) Stop() {
	daemon.udpServer.Stop()
}

// TestSNMPD conducts unit tests on SNMP daemon, see TestSNMPD for daemon setup.
func TestSNMPD(daemon *Daemon, t testingstub.T) {
	// Server should start within two seconds
	var stoppedNormally bool
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)

	// Create a UDP client
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(daemon.Port))
	if err != nil {
		t.Fatal(err)
	}
	clientConn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	// Send a GetNextRequest from a non-existing OID 1.3.6.1.2.1.1.1.1.0
	getNextRequest := []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x2a, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU1    SZ   INT    SZ  REQID460219274...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa1, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x8a, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .1    .1    .0   NUL    SZ
		0x02, 0x01, 0x01, 0x01, 0x01, 0x00, 0x05, 0x00,
	}
	if _, err := clientConn.Write(getNextRequest); err != nil {
		t.Fatal(err)
	}
	// Expect a valid response to IP address query i.e. the first supported OID
	packetBuf := make([]byte, MaxPacketSize)
	clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = clientConn.Read(packetBuf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(packetBuf, []byte(daemon.CommunityName)) ||
		!bytes.Contains(packetBuf, []byte{0x1b, 0x6e, 0x63, 0x8a}) || // request ID
		!bytes.Contains(packetBuf, []byte(inet.GetPublicIP())) {
		t.Fatalf("%s\n%#v", string(packetBuf), packetBuf)
	}

	// Send a GetNextRequest from a non-existing OID 1.3.6.1.2.1.1.1.1.0 using wrong community string
	getNextRequest = []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x2a, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     E APDU1    SZ   INT    SZ  REQID460219274...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x12, 0xa1, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ  1.3     .6    .1
		0x8a, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x09, 0x2b, 0x06, 0x01,
		//.2    .1    .1    .1    .1    .0   NUL    SZ
		0x02, 0x01, 0x01, 0x01, 0x01, 0x00, 0x05, 0x00,
	}
	if _, err := clientConn.Write(getNextRequest); err != nil {
		t.Fatal(err)
	}
	// Expect a valid response to IP address query i.e. the first supported OID
	packetBuf = make([]byte, MaxPacketSize)
	clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = clientConn.Read(packetBuf)
	if err == nil {
		t.Fatal("should not have responded")
	}

	// Send a GetRequest for IP address's OID 1.3.6.1.4.1.52535.121.100
	getNextRequest = []byte{
		//ASN1  SZ   INT    SZ
		0x30, 0x2a, 0x02, 0x01,
		//v2  OSTR    SZ     p     u    b      l     i     c APDU0    SZ   INT    SZ  REQID460219274...
		0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa0, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
		//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ   1.3    .6    .1
		0x8a, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x0a, 0x2b, 0x06, 0x01,
		//.4  .1  .52535..........  .121  .100   NUL    SZ
		0x4, 0x1, 0x83, 0x9a, 0x37, 0x79, 0x64, 0x05, 0x00,
	}
	if _, err := clientConn.Write(getNextRequest); err != nil {
		t.Fatal(err)
	}
	// Expect a valid response
	packetBuf = make([]byte, MaxPacketSize)
	clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = clientConn.Read(packetBuf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(packetBuf, []byte(daemon.CommunityName)) ||
		!bytes.Contains(packetBuf, []byte{0x1b, 0x6e, 0x63, 0x8a}) || // request ID
		!bytes.Contains(packetBuf, []byte(inet.GetPublicIP())) {
		t.Fatalf("%s\n%#v", string(packetBuf), packetBuf)
	}

	// Send a GetNextRequest on the very last of supported OID, 1.3.6.1.4.1.52535.121.115
	lastValidOIDTest := func() []byte {
		// Re-dial because this function is used going to be used for rate limit test
		clientConn, err := net.DialUDP("udp", nil, serverAddr)
		if err != nil {
			t.Fatal(err)
		}
		defer clientConn.Close()
		getNextRequest = []byte{
			//ASN1  SZ   INT    SZ
			0x30, 0x2a, 0x02, 0x01,
			//v2  OSTR    SZ     p     u    b      l     i     c APDU1    SZ   INT    SZ  REQID460219274...
			0x01, 0x04, 0x06, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0xa1, 0x1d, 0x02, 0x04, 0x1b, 0x6e, 0x63,
			//..   INT   SZ   NoErr  INT   SZ  EIDX0  ASN1    SZ  ASN1    SZ   OID    SZ   1.3    .6    .1
			0x8a, 0x02, 0x01, 0x00, 0x02, 0x01, 0x00, 0x30, 0x0f, 0x30, 0x0d, 0x06, 0x0a, 0x2b, 0x06, 0x01,
			//.4  .1  .52535..........  .121  .115   NUL    SZ
			0x4, 0x1, 0x83, 0x9a, 0x37, 0x79, 0x73, 0x05, 0x00,
		}
		if _, err := clientConn.Write(getNextRequest); err != nil {
			t.Fatal(err)
		}
		// Expect an EndOfMibView response
		replyBuf := make([]byte, MaxPacketSize)
		clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		// Should IO error occur, the return value shall be an empty byte slice.
		n, _ := clientConn.Read(replyBuf)
		replyBuf = replyBuf[:n]
		return replyBuf
	}
	packetBuf = lastValidOIDTest()
	if !bytes.Contains(packetBuf, []byte(daemon.CommunityName)) ||
		!bytes.Contains(packetBuf, []byte{0x1b, 0x6e, 0x63, 0x8a}) || // request ID
		!bytes.Contains(packetBuf, []byte{snmp.TagEndOfMIBView, 0x00}) {
		t.Fatalf("%s\n%#v", string(packetBuf), packetBuf)
	}

	// Daemon must stop in a second
	daemon.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.Stop()
	daemon.Stop()
}
