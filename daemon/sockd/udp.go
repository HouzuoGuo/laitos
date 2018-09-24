package sockd

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	UDPIPv4PacketLength = 1 + IPv4PacketLength
	UDPIPv6PacketLength = 1 + IPv6PacketLength
	UDPIPAddrIndex      = 1
	DMHeaderLength      = 1 + 1 + 2
)

var (
	ErrMalformedUDPPacket = errors.New("received packet is abnormally small")
	BacklogClearInterval  = 2 * IOTimeoutSec
	UDPDurationStats      = misc.NewStats()
)

func MakeUDPRequestHeader(addr net.Addr) ([]byte, int) {
	ipStr, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, 0
	}
	ip := net.ParseIP(ipStr)
	ipLength := 0
	v4IP := ip.To4()
	header := make([]byte, 20)
	if v4IP == nil {
		v4IP = ip.To16()
		header[0] = AddressTypeIPv6
		ipLength = net.IPv6len
	} else {
		header[0] = AddressTypeIPv4
		ipLength = net.IPv4len
	}
	copy(header[1:], v4IP)
	iPort, _ := strconv.Atoi(port)
	binary.BigEndian.PutUint16(header[1+ipLength:], uint16(iPort))
	return header[:1+ipLength+2], 1 + ipLength + 2
}

type UDPDaemon struct {
	Address    string
	Password   string
	PerIPLimit int
	UDPPort    int

	DNSDaemon *dnsd.Daemon

	udpBackLog       *UDPBackLog
	udpListener      *net.UDPConn
	udpTable         *UDPTable
	rateLimitUDP     *misc.RateLimit
	udpLoopIsRunning int32
	stopUDP          chan bool

	cipher *Cipher
	logger misc.Logger
}

func (daemon *UDPDaemon) Initialise() error {
	daemon.logger = misc.Logger{
		ComponentName: "sockd",
		ComponentID:   []misc.LoggerIDField{{"Addr", daemon.Address}, {"UDP", daemon.UDPPort}},
	}
	daemon.rateLimitUDP = &misc.RateLimit{
		Logger:   daemon.logger,
		MaxCount: daemon.PerIPLimit * 100,
		UnitSecs: RateLimitIntervalSec,
	}
	daemon.rateLimitUDP.Initialise()
	daemon.udpBackLog = &UDPBackLog{backlog: make(map[string][]byte), mutex: new(sync.Mutex)}
	daemon.stopUDP = make(chan bool)

	daemon.cipher = &Cipher{}
	daemon.cipher.Initialise(daemon.Password)
	return nil
}

func (daemon *UDPDaemon) StartAndBlock() error {
	listenAddr := net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.UDPPort))
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("sockd.StartAndBlockUDP: failed to resolve address %s - %v", listenAddr, err)
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("sockd.StartAndBlockUDP: failed to listen on %s - %v", listenAddr, err)
	}
	defer udpServer.Close()
	daemon.udpListener = udpServer
	daemon.logger.Info("StartAndBlockUDP", "", nil, "going to listen for data")

	daemon.udpBackLog = &UDPBackLog{backlog: map[string][]byte{}, mutex: new(sync.Mutex)}
	daemon.udpTable = &UDPTable{connections: map[string]net.PacketConn{}, mutex: new(sync.Mutex)}
	go func() {
		intervalTick := time.NewTicker(BacklogClearInterval).C
		loggerTick := time.NewTicker(15 * time.Minute).C
		for {
			select {
			case <-intervalTick:
				daemon.udpBackLog.Clear()
			case <-loggerTick:
				daemon.logger.Info("StartAndBlockUDP", "", nil, "current backlog length %d, connection table length %d",
					daemon.udpBackLog.Len(), daemon.udpTable.Len())
			case <-daemon.stopUDP:
				return
			}
		}
	}()

	udpEncryptedServer := &UDPCipherConnection{PacketConn: udpServer, Cipher: daemon.cipher.Copy()}
	for {
		if misc.EmergencyLockDown {
			return misc.ErrEmergencyLockDown
		}
		atomic.StoreInt32(&daemon.udpLoopIsRunning, 1)
		packetBuf := make([]byte, MaxPacketSize)
		packetLength, clientAddr, err := udpEncryptedServer.ReadFrom(packetBuf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			daemon.logger.Warning("StartAndBlockUDP", "", err, "failed to read packet")
			continue
		}
		udpClientAddr := clientAddr.(*net.UDPAddr)
		clientPacket := make([]byte, packetLength)
		copy(clientPacket, packetBuf[:packetLength])

		clientIP := udpClientAddr.IP.String()
		if daemon.rateLimitUDP.Add(clientIP, true) {
			go daemon.HandleUDPConnection(udpEncryptedServer, packetLength, udpClientAddr, packetBuf)
		}
	}
	return nil
}

func (daemon *UDPDaemon) Stop() {
	if listener := daemon.udpListener; listener != nil {
		if atomic.CompareAndSwapInt32(&daemon.udpLoopIsRunning, 1, 0) {
			daemon.stopUDP <- true
		}
		if err := listener.Close(); err != nil {
			daemon.logger.Warning("Stop", "", err, "failed to close UDP listener")
		}
	}
}

type UDPBackLog struct {
	mutex   *sync.Mutex
	backlog map[string][]byte
}

func (backlog *UDPBackLog) Clear() {
	backlog.mutex.Lock()
	backlog.backlog = make(map[string][]byte)
	backlog.mutex.Unlock()
}

func (backlog *UDPBackLog) Get(addr string) (packet []byte, found bool) {
	backlog.mutex.Lock()
	packet, found = backlog.backlog[addr]
	backlog.mutex.Unlock()
	return
}

func (backlog *UDPBackLog) Put(addr string, packet []byte) {
	backlog.mutex.Lock()
	backlog.backlog[addr] = packet
	backlog.mutex.Unlock()
	return
}

func (backlog *UDPBackLog) Len() (ret int) {
	backlog.mutex.Lock()
	ret = len(backlog.backlog)
	backlog.mutex.Unlock()
	return
}

type UDPTable struct {
	mutex       *sync.Mutex
	connections map[string]net.PacketConn
}

func (table *UDPTable) Delete(clientID string) net.PacketConn {
	table.mutex.Lock()
	defer table.mutex.Unlock()
	conn, found := table.connections[clientID]
	if found {
		delete(table.connections, clientID)
		return conn
	}
	return nil
}

func (table *UDPTable) Get(clientID string) (conn net.PacketConn, found bool, err error) {
	table.mutex.Lock()
	defer table.mutex.Unlock()
	conn, found = table.connections[clientID]
	if !found {
		conn, err = net.ListenPacket("udp", "")
		if err != nil {
			return nil, false, err
		}
		table.connections[clientID] = conn
	}
	return
}

func (table *UDPTable) Len() (ret int) {
	table.mutex.Lock()
	ret = len(table.connections)
	table.mutex.Unlock()
	return
}

type UDPCipherConnection struct {
	net.PacketConn
	*Cipher
	logger misc.Logger
}

func (conn *UDPCipherConnection) Close() error {
	return conn.PacketConn.Close()
}

func (conn *UDPCipherConnection) ReadFrom(b []byte) (n int, src net.Addr, err error) {
	cipher := conn.Copy()
	buf := make([]byte, MaxPacketSize)
	n, src, err = conn.PacketConn.ReadFrom(buf)
	if err != nil {
		return
	}
	if n < conn.IVLength {
		return 0, nil, ErrMalformedUDPPacket
	}

	iv := make([]byte, conn.IVLength)
	copy(iv, buf[:conn.IVLength])
	cipher.InitDecryptionStream(iv)
	cipher.Decrypt(b[0:], buf[conn.IVLength:n])

	n -= conn.IVLength
	return
}

func (conn *UDPCipherConnection) WriteTo(b []byte, dest net.Addr) (n int, err error) {
	cipher := conn.Copy()
	iv := cipher.InitEncryptionStream()
	packetLen := len(b) + len(iv)
	cipherData := make([]byte, packetLen)
	copy(cipherData, iv)

	cipher.Encrypt(cipherData[len(iv):], b)
	n, err = conn.PacketConn.WriteTo(cipherData, dest)
	return
}

func (conn *UDPCipherConnection) WriteRand(dest net.Addr) {
	randBuf := make([]byte, RandNum(4, 50, 600))
	_, err := rand.Read(randBuf)
	if err != nil {
		conn.logger.Warning("WriteRand", dest.String(), err, "failed to get random bytes")
		return
	}
	conn.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
	if _, err := conn.WriteTo(randBuf, dest); err != nil && !strings.Contains(err.Error(), "closed") {
		conn.logger.Warning("WriteRand", dest.String(), err, "failed to write random bytes")
	}
}

func (daemon *UDPDaemon) HandleUDPConnection(server *UDPCipherConnection, n int, clientAddr *net.UDPAddr, packet []byte) {
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		UDPDurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	var destIP net.IP
	var packetLen int
	addrType := packet[AddressTypeIndex]

	maskedType := addrType & AddressTypeMask
	switch maskedType {
	case AddressTypeIPv4:
		packetLen = UDPIPv4PacketLength
		if len(packet) < packetLen {
			daemon.logger.Warning("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		destIP = net.IP(packet[UDPIPAddrIndex : UDPIPAddrIndex+net.IPv4len])
	case AddressTypeIPv6:
		packetLen = UDPIPv6PacketLength
		if len(packet) < packetLen {
			daemon.logger.Warning("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		destIP = net.IP(packet[UDPIPAddrIndex : UDPIPAddrIndex+net.IPv6len])
	case AddressTypeDM:
		packetLen = int(packet[DMAddrLengthIndex]) + DMHeaderLength
		if len(packet) < packetLen {
			daemon.logger.Warning("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		dest := string(packet[DMAddrHeaderLength : DMAddrHeaderLength+int(packet[DMAddrLengthIndex])])
		destIP = net.ParseIP(dest)
		if destIP != nil && IsReservedAddr(destIP) {
			daemon.logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve reserved address %s", dest)
			return
		}
		if daemon.DNSDaemon.IsInBlacklist(dest) {
			daemon.logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve blacklisted domain name %s", dest)
			return
		}
		resolveDestIP, err := net.ResolveIPAddr("ip", dest)
		if err != nil {
			daemon.logger.Warning("HandleUDPConnection", clientAddr.IP.String(), nil, "failed to resolve domain name \"%s\"", dest)
			return
		}
		destIP = resolveDestIP.IP
	default:
		daemon.logger.Warning("HandleUDPConnection", clientAddr.IP.String(), nil, "unknown mask type %d", maskedType)
		server.WriteRand(clientAddr)
		return
	}
	if IsReservedAddr(destIP) {
		daemon.logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve reserved address %s", destIP.String())
		return
	}
	if daemon.DNSDaemon.IsInBlacklist(destIP.String()) {
		daemon.logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve blacklisted address %s", destIP.String())
		return
	}
	destAddr := &net.UDPAddr{
		IP:   destIP,
		Port: int(binary.BigEndian.Uint16(packet[packetLen-2 : packetLen])),
	}
	if destAddr.Port < 1 {
		daemon.logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not connect to invalid destination port %s:%d", destIP.String(), destAddr.Port)
		server.WriteRand(clientAddr)
		return
	}
	if _, found := daemon.udpBackLog.Get(destAddr.String()); !found {
		backlogPacket := make([]byte, packetLen)
		copy(backlogPacket, packet)
		daemon.udpBackLog.Put(destAddr.String(), backlogPacket)
	}

	udpClient, found, err := daemon.udpTable.Get(clientAddr.String())
	if err != nil || udpClient == nil {
		daemon.logger.Warning("HandleUDPConnection", clientAddr.IP.String(), err, "failed to retrieve connection from table")
		return
	}
	if !found {
		go func() {
			daemon.PipeUDPConnection(server, clientAddr, udpClient)
			daemon.udpTable.Delete(clientAddr.String())
		}()
	}
	udpClient.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
	_, err = udpClient.WriteTo(packet[packetLen:n], destAddr)
	if err != nil {
		daemon.logger.Warning("HandleUDPConnection", clientAddr.IP.String(), err, "failed to respond to client")
		if conn := daemon.udpTable.Delete(clientAddr.String()); conn != nil {
			conn.Close()
		}
	}
	return
}

func (daemon *UDPDaemon) PipeUDPConnection(server net.PacketConn, clientAddr *net.UDPAddr, client net.PacketConn) {
	packet := make([]byte, MaxPacketSize)
	defer client.Close()
	for {
		client.SetReadDeadline(time.Now().Add(IOTimeoutSec))
		length, addr, err := client.ReadFrom(packet)
		if err != nil {
			return
		}
		if backlogPacket, found := daemon.udpBackLog.Get(addr.String()); found {
			server.WriteTo(append(backlogPacket, packet[:length]...), clientAddr)
		} else {
			header, headerLength := MakeUDPRequestHeader(addr)
			server.WriteTo(append(header[:headerLength], packet[:length]...), clientAddr)
		}
	}
}
