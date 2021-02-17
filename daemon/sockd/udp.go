package sockd

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	UDPIPv4PacketLength = 1 + IPv4PacketLength
	UDPIPv6PacketLength = 1 + IPv6PacketLength
	UDPIPAddrIndex      = 1
	DMHeaderLength      = 1 + 1 + 2
	UDPClearIntervalSec = 3 * IOTimeoutSec
)

var (
	ErrMalformedUDPPacket = errors.New("received packet is abnormally small")
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

	logger     lalog.Logger
	udpBackLog *UDPBackLog
	udpTable   *UDPTable
	cipher     *Cipher
	udpServer  *common.UDPServer
}

func (daemon *UDPDaemon) Initialise() error {
	daemon.cipher = &Cipher{}
	daemon.cipher.Initialise(daemon.Password)
	daemon.udpServer = &common.UDPServer{
		ListenAddr:  daemon.Address,
		ListenPort:  daemon.UDPPort,
		AppName:     "sockd",
		App:         daemon,
		LimitPerSec: daemon.PerIPLimit,
	}
	daemon.udpServer.Initialise()
	daemon.logger = lalog.Logger{
		ComponentName: "sockd",
		ComponentID:   []lalog.LoggerIDField{{Key: "UDP", Value: strconv.Itoa(daemon.UDPPort)}},
	}
	return nil
}

func (daemon *UDPDaemon) GetUDPStatsCollector() *misc.Stats {
	return misc.SOCKDStatsUDP
}

func (daemon *UDPDaemon) HandleUDPClient(logger lalog.Logger, ip string, client *net.UDPAddr, packet []byte, srv *net.UDPConn) {
	udpEncryptedServer := &UDPCipherConnection{PacketConn: srv, Cipher: daemon.cipher.Copy(), logger: logger}
	daemon.HandleUDPConnection(logger, udpEncryptedServer, len(packet), client, packet)
}

func (daemon *UDPDaemon) StartAndBlock() error {
	daemon.udpBackLog = &UDPBackLog{backlog: map[string][]byte{}, mutex: new(sync.Mutex)}
	daemon.udpTable = &UDPTable{connections: map[string]net.PacketConn{}, mutex: new(sync.Mutex)}
	go func() {
		for {
			if !daemon.udpServer.IsRunning() {
				return
			}
			daemon.logger.Info("StartAndBlock", "", nil, "going to clear %d backlog packets and %d connections", daemon.udpBackLog.Len(), daemon.udpTable.Len())
			daemon.udpBackLog.Clear()
			daemon.udpTable.Clear()
			time.Sleep(UDPClearIntervalSec * time.Second)
		}
	}()
	return daemon.udpServer.StartAndBlock()
}

func (daemon *UDPDaemon) Stop() {
	daemon.udpServer.Stop()
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

func (table *UDPTable) Clear() {
	table.mutex.Lock()
	defer table.mutex.Unlock()
	for _, conn := range table.connections {
		_ = conn.Close()
	}
	table.connections = map[string]net.PacketConn{}
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
	logger lalog.Logger
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
		conn.logger.Info("WriteRand", dest.String(), nil, "failed to get random bytes - %v", err)
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
		conn.logger.Info("WriteRand", dest.String(), nil, "failed to write random bytes - %v", err)

		return
	}
	if _, err := conn.WriteTo(randBuf, dest); err != nil && !errors.Is(err, net.ErrClosed) {
		conn.logger.Info("WriteRand", dest.String(), nil, "failed to write random bytes - %v", err)
		return
	}
}

func (daemon *UDPDaemon) HandleUDPConnection(logger lalog.Logger, server *UDPCipherConnection, n int, clientAddr *net.UDPAddr, packet []byte) {
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		misc.SOCKDStatsUDP.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()

	if len(packet) < 3 {
		logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
		server.WriteRand(clientAddr)
		return
	}

	var destIP net.IP
	var packetLen int
	addrType := packet[AddressTypeIndex]

	maskedType := addrType & AddressTypeMask
	switch maskedType {
	case AddressTypeIPv4:
		packetLen = UDPIPv4PacketLength
		if len(packet) < packetLen {
			logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		destIP = packet[UDPIPAddrIndex : UDPIPAddrIndex+net.IPv4len]
	case AddressTypeIPv6:
		packetLen = UDPIPv6PacketLength
		if len(packet) < packetLen {
			logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		destIP = packet[UDPIPAddrIndex : UDPIPAddrIndex+net.IPv6len]
	case AddressTypeDM:
		packetLen = int(packet[DMAddrLengthIndex]) + DMHeaderLength
		if len(packet) < packetLen {
			logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		dest := string(packet[DMAddrHeaderLength : DMAddrHeaderLength+int(packet[DMAddrLengthIndex])])
		if strings.ContainsRune(dest, 0) {
			logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve destination that contains NULL byte")
			return
		}
		destIP = net.ParseIP(dest)
		if destIP != nil && IsReservedAddr(destIP) {
			logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve reserved address %s", dest)
			return
		}
		if daemon.DNSDaemon.IsInBlacklist(dest) {
			logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve blacklisted domain name %s", dest)
			return
		}
		resolveDestIP, err := net.ResolveIPAddr("ip", dest)
		if err != nil {
			logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "failed to resolve domain name \"%s\"", dest)
			return
		}
		destIP = resolveDestIP.IP
	default:
		logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "unknown mask type %d", maskedType)
		server.WriteRand(clientAddr)
		return
	}
	if IsReservedAddr(destIP) {
		logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve reserved address %s", destIP.String())
		return
	}
	if daemon.DNSDaemon.IsInBlacklist(destIP.String()) {
		logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not serve blacklisted address %s", destIP.String())
		return
	}
	destAddr := &net.UDPAddr{
		IP:   destIP,
		Port: int(binary.BigEndian.Uint16(packet[packetLen-2 : packetLen])),
	}
	if destAddr.Port < 1 {
		logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "will not connect to invalid destination port %s:%d", destIP.String(), destAddr.Port)
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
		logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "failed to retrieve connection from table - %v", err)
		return
	}
	if !found {
		go func() {
			daemon.PipeUDPConnection(server, clientAddr, udpClient)
			daemon.udpTable.Delete(clientAddr.String())
		}()
	}
	logger.MaybeMinorError(udpClient.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)))
	_, err = udpClient.WriteTo(packet[packetLen:n], destAddr)
	if err != nil {
		logger.Info("HandleUDPConnection", clientAddr.IP.String(), nil, "failed to respond to client - %v", err)
		if conn := daemon.udpTable.Delete(clientAddr.String()); conn != nil {
			conn.Close()
		}
	}
}

func (daemon *UDPDaemon) PipeUDPConnection(server net.PacketConn, clientAddr *net.UDPAddr, client net.PacketConn) {
	packet := make([]byte, MaxPacketSize)
	defer func() {
		_ = client.Close()
	}()
	for {
		if misc.EmergencyLockDown {
			lalog.DefaultLogger.Warning("PipeTCPConnection", "", misc.ErrEmergencyLockDown, "")
			return
		} else if err := client.SetReadDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
			return
		}
		length, addr, err := client.ReadFrom(packet)
		if err != nil {
			return
		}
		if err := server.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
			return
		}
		if backlogPacket, found := daemon.udpBackLog.Get(addr.String()); found {
			if _, err := server.WriteTo(append(backlogPacket, packet[:length]...), clientAddr); err != nil {
				return
			}
		} else {
			header, headerLength := MakeUDPRequestHeader(addr)
			if _, err := server.WriteTo(append(header[:headerLength], packet[:length]...), clientAddr); err != nil {
				return
			}
		}
	}
}
