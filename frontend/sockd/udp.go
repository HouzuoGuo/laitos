package sockd

import (
	cryptRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/global"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	UDPIPv4PacketLength = 1 + IPv4PacketLength
	UDPIPv6PacketLength = 1 + IPv6PacketLength
	UDPIPAddrIndex      = 1
	DMHeaderLength      = 1 + 1 + 2
)

var (
	ErrMalformedUDPPacket = fmt.Errorf("Received packet is abnormally small")
	BacklogClearInterval  = 2 * IOTimeoutSec
	UDPDurationStats      = env.NewStats()
)

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
	logger global.Logger
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
	randBuf := make([]byte, rand.Intn(1024))
	_, err := cryptRand.Read(randBuf)
	if err != nil {
		conn.logger.Warningf("WriteRand", dest.String(), err, "ran out of randomness")
		return
	}
	conn.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
	if _, err := conn.WriteTo(randBuf, dest); err != nil {
		conn.logger.Warningf("WriteRand", dest.String(), err, "failed to write random UDP response")
	}
}

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

func (sock *Sockd) StartAndBlockUDP() error {
	listenAddr := fmt.Sprintf("%s:%d", sock.Address, sock.UDPPort)
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("Sockd.StartAndBlockUDP: failed to resolve address %s - %v", listenAddr, err)
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("Sockd.StartAndBlockUDP: failed to listen on %s - %v", listenAddr, err)
	}
	defer udpServer.Close()
	sock.UDPListener = udpServer
	sock.Logger.Printf("StartAndBlockUDP", listenAddr, nil, "going to listen for data")

	sock.UDPBacklog = &UDPBackLog{backlog: map[string]([]byte){}, mutex: new(sync.Mutex)}
	sock.UDPTable = &UDPTable{connections: map[string]net.PacketConn{}, mutex: new(sync.Mutex)}
	go func() {
		intervalTick := time.NewTicker(BacklogClearInterval).C
		loggerTick := time.NewTicker(15 * time.Minute).C
		for {
			select {
			case <-intervalTick:
				sock.UDPBacklog.Clear()
			case <-loggerTick:
				sock.Logger.Printf("StartAndBlockUDP", "", nil, "current backlog length %d, connection table length %d",
					sock.UDPBacklog.Len(), sock.UDPTable.Len())
			case <-sock.stopUDP:
				return
			}
		}
	}()

	udpEncryptedServer := &UDPCipherConnection{PacketConn: udpServer, Cipher: sock.cipher.Copy()}
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		atomic.StoreInt32(&sock.udpLoopIsRunning, 1)
		packetBuf := make([]byte, MaxPacketSize)
		packetLength, clientAddr, err := udpEncryptedServer.ReadFrom(packetBuf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			sock.Logger.Warningf("StartAndBlockUDP", "", err, "failed to read packet")
			continue
		}
		udpClientAddr := clientAddr.(*net.UDPAddr)
		clientPacket := make([]byte, packetLength)
		copy(clientPacket, packetBuf[:packetLength])

		clientIP := udpClientAddr.IP.String()
		if sock.rateLimitUDP.Add(clientIP, true) {
			go sock.HandleUDPConnection(udpEncryptedServer, packetLength, udpClientAddr, packetBuf)
		}
	}
}

func (sock *Sockd) HandleUDPConnection(server *UDPCipherConnection, n int, clientAddr *net.UDPAddr, packet []byte) {
	beginTimeNano := time.Now().UnixNano()
	defer UDPDurationStats.Trigger(float64((time.Now().UnixNano() - beginTimeNano) / 1000000))
	var destIP net.IP
	var packetLen int
	addrType := packet[AddressTypeIndex]

	maskedType := addrType & AddressTypeMask
	switch maskedType {
	case AddressTypeIPv4:
		packetLen = UDPIPv4PacketLength
		if len(packet) < packetLen {
			sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		destIP = net.IP(packet[UDPIPAddrIndex : UDPIPAddrIndex+net.IPv4len])
	case AddressTypeIPv6:
		packetLen = UDPIPv6PacketLength
		if len(packet) < packetLen {
			sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		destIP = net.IP(packet[UDPIPAddrIndex : UDPIPAddrIndex+net.IPv6len])
	case AddressTypeDM:
		packetLen = int(packet[DMAddrLengthIndex]) + DMHeaderLength
		if len(packet) < packetLen {
			sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), nil, "incoming packet is abnormally small")
			server.WriteRand(clientAddr)
			return
		}
		resolveName := string(packet[DMAddrHeaderLength : DMAddrHeaderLength+int(packet[DMAddrLengthIndex])])
		if strings.ContainsRune(resolveName, 0x00) {
			sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), nil, "dm address contains invalid byte 0")
			server.WriteRand(clientAddr)
			return
		}
		resolveDestIP, err := net.ResolveIPAddr("ip", resolveName)
		if err != nil {
			sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), nil, "failed to resolve domain name \"%s\"", resolveName)
			server.WriteRand(clientAddr)
			return
		}
		destIP = resolveDestIP.IP
	default:
		sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), nil, "unknown mask type %d", maskedType)
		server.WriteRand(clientAddr)
		return
	}
	destAddr := &net.UDPAddr{
		IP:   destIP,
		Port: int(binary.BigEndian.Uint16(packet[packetLen-2 : packetLen])),
	}
	if _, found := sock.UDPBacklog.Get(destAddr.String()); !found {
		backlogPacket := make([]byte, packetLen)
		copy(backlogPacket, packet)
		sock.UDPBacklog.Put(destAddr.String(), backlogPacket)
	}

	udpClient, found, err := sock.UDPTable.Get(clientAddr.String())
	if err != nil || udpClient == nil {
		sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), err, "failed to retrieve connection from table")
		return
	}
	if !found {
		go func() {
			sock.PipeUDPConnection(server, clientAddr, udpClient)
			sock.UDPTable.Delete(clientAddr.String())
		}()
	}
	udpClient.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
	_, err = udpClient.WriteTo(packet[packetLen:n], destAddr)
	if err != nil {
		sock.Logger.Warningf("HandleUDPConnection", clientAddr.IP.String(), err, "failed to respond to client")
		if conn := sock.UDPTable.Delete(clientAddr.String()); conn != nil {
			conn.Close()
		}
	}
	return
}

func (sock *Sockd) PipeUDPConnection(server net.PacketConn, clientAddr *net.UDPAddr, client net.PacketConn) {
	packet := make([]byte, MaxPacketSize)
	defer client.Close()
	for {
		client.SetReadDeadline(time.Now().Add(IOTimeoutSec))
		length, addr, err := client.ReadFrom(packet)
		if err != nil {
			return
		}
		if backlogPacket, found := sock.UDPBacklog.Get(addr.String()); found {
			server.WriteTo(append(backlogPacket, packet[:length]...), clientAddr)
		} else {
			header, headerLength := MakeUDPRequestHeader(addr)
			server.WriteTo(append(header[:headerLength], packet[:length]...), clientAddr)
		}
	}
}
