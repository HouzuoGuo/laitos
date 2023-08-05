package sockd

import (
	"crypto/rand"
	"io"
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

type UDPDaemon struct {
	Address    string
	Password   string
	PerIPLimit int
	UDPPort    int

	DNSDaemon *dnsd.Daemon

	logger          *lalog.Logger
	udpBacklog      *UDPBacklog
	derivedPassword []byte
	udpServer       *common.UDPServer
}

func (daemon *UDPDaemon) Initialise() error {
	daemon.udpServer = &common.UDPServer{
		ListenAddr:  daemon.Address,
		ListenPort:  daemon.UDPPort,
		AppName:     "sockd",
		App:         daemon,
		LimitPerSec: daemon.PerIPLimit,
	}
	daemon.udpServer.Initialise()
	daemon.derivedPassword = GetDerivedKey(daemon.Password)
	daemon.logger = &lalog.Logger{
		ComponentName: "sockd",
		ComponentID:   []lalog.LoggerIDField{{Key: "UDP", Value: strconv.Itoa(daemon.UDPPort)}},
	}
	daemon.udpBacklog = newUDPBacklog()
	return nil
}

func (daemon *UDPDaemon) GetUDPStatsCollector() *misc.Stats {
	return misc.SOCKDStatsUDP
}

func (daemon *UDPDaemon) HandleUDPClient(logger *lalog.Logger, ip string, client *net.UDPAddr, packet []byte, srv *net.UDPConn) {
	decryptedLen, err := DecryptUDPPacket(len(packet), packet, daemon.derivedPassword)
	if err != nil {
		logger.Info(ip, nil, "failed to decrypt packet - %v", err)
		WriteRandomToUDP(srv, client)
		return
	}
	packet = packet[:decryptedLen]
	proxyDestAddr := ParseDestAddr(packet)
	if proxyDestAddr == nil {
		logger.Info(ip, nil, "failed to get destination address - %v", err)
		WriteRandomToUDP(srv, client)
		return
	}
	destNameOrIP, destPort := proxyDestAddr.HostPort()
	if destNameOrIP == "" || destPort == 0 || strings.ContainsRune(destNameOrIP, 0) {
		logger.Info(ip, nil, "invalid destination IP (%s) or port (%d)", destNameOrIP, destPort)
		WriteRandomToUDP(srv, client)
		return
	}
	if parsedIP := net.ParseIP(destNameOrIP); parsedIP != nil {
		if IsReservedAddr(parsedIP) {
			logger.Info(ip, nil, "will not serve reserved address %s", destNameOrIP)
			return
		}
	}
	if daemon.DNSDaemon.IsInBlacklist(destNameOrIP) {
		logger.Info(ip, nil, "will not serve blacklisted destination %s", destNameOrIP)
		return
	}
	resolvedAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(destNameOrIP, strconv.Itoa(destPort)))
	if err != nil {
		logger.Info(ip, err, "failed to resolve destination \"%s\"", destNameOrIP)
		return
	}
	payload := packet[len(proxyDestAddr):]
	backlogConn := daemon.udpBacklog.Get(client.String())
	if backlogConn == nil {
		backlogConn, err = net.ListenPacket("udp", "")
		if err != nil {
			logger.Info(ip, nil, "failed to listen for destination %s - %v", destNameOrIP, err)
			return
		}
		daemon.udpBacklog.Add(client, &EncryptedUDPConn{PacketConn: srv, DerivedPassword: daemon.derivedPassword, buf: make([]byte, MaxPacketSize)}, backlogConn)
	}
	_, err = backlogConn.WriteTo(payload, resolvedAddr)
	if err != nil {
		logger.Info(ip, nil, "failed to write for destination %s - %v", destNameOrIP, err)
		return
	}
}

func (daemon *UDPDaemon) StartAndBlock() error {
	return daemon.udpServer.StartAndBlock()
}

func (daemon *UDPDaemon) Stop() {
	daemon.udpServer.Stop()
}

func (daemon *UDPDaemon) WriteRand(server net.PacketConn, dest net.Addr) {
	randBuf := make([]byte, RandNum(4, 50, 600))
	_, err := rand.Read(randBuf)
	if err != nil {
		daemon.logger.Info(dest.String(), nil, "failed to get random bytes - %v", err)
		return
	}
	if err := server.SetWriteDeadline(time.Now().Add(IOTimeout)); err != nil {
		daemon.logger.Info(dest.String(), nil, "failed to write random bytes - %v", err)
		return
	}
	if _, err := server.WriteTo(randBuf, dest); err != nil && !strings.Contains(err.Error(), "closed") {
		daemon.logger.Info(dest.String(), nil, "failed to write random bytes - %v", err)
		return
	}
}

type UDPBacklog struct {
	sync.RWMutex
	ongoingConns map[string]net.PacketConn
}

func newUDPBacklog() *UDPBacklog {
	backlog := &UDPBacklog{}
	backlog.ongoingConns = make(map[string]net.PacketConn)
	return backlog
}

func (backlog *UDPBacklog) Get(addr string) net.PacketConn {
	backlog.RLock()
	defer backlog.RUnlock()
	return backlog.ongoingConns[addr]
}

func (backlog *UDPBacklog) Delete(addr string) net.PacketConn {
	backlog.Lock()
	defer backlog.Unlock()
	conn, ok := backlog.ongoingConns[addr]
	if ok {
		delete(backlog.ongoingConns, addr)
		return conn
	}
	return nil
}

func (backlog *UDPBacklog) Add(client net.Addr, dest, srv net.PacketConn) {
	backlog.Lock()
	defer backlog.Unlock()
	backlog.ongoingConns[client.String()] = srv
	go func() {
		CopyWithTimeout(dest, client, srv)
		if pc := backlog.Delete(client.String()); pc != nil {
			pc.Close()
		}
	}()
}

func CopyWithTimeout(destConn net.PacketConn, client net.Addr, srv net.PacketConn) error {
	buf := make([]byte, MaxPacketSize)
	for {
		srv.SetReadDeadline(time.Now().Add(IOTimeout))
		n, addr, err := srv.ReadFrom(buf)
		if err != nil {
			return err
		}
		srcAddr := GetSocksAddr(addr)
		copy(buf[len(srcAddr):], buf[:n])
		copy(buf, srcAddr)
		_, err = destConn.WriteTo(buf[:len(srcAddr)+n], client)
		if err != nil {
			return err
		}
	}
}

type EncryptedUDPConn struct {
	net.PacketConn
	DerivedPassword []byte
	sync.Mutex
	buf []byte
}

func (encConn *EncryptedUDPConn) WriteTo(buf []byte, client net.Addr) (int, error) {
	encConn.Lock()
	defer encConn.Unlock()
	salt := encConn.buf[:LenDerivedPassword]
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return 0, err
	}
	cipher, err := AEADBlockCipher(encConn.DerivedPassword, salt)
	if err != nil {
		return 0, err
	}
	if len(encConn.buf) < LenDerivedPassword+len(buf)+cipher.Overhead() {
		return 0, io.ErrShortBuffer
	}
	sealed := cipher.Seal(encConn.buf[LenDerivedPassword:LenDerivedPassword], ZeroBytes[:cipher.NonceSize()], buf, nil)
	if err != nil {
		return 0, err
	}
	_, err = encConn.PacketConn.WriteTo(encConn.buf[:LenDerivedPassword+len(sealed)], client)
	return len(buf), err
}

func (encConn *EncryptedUDPConn) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, clientAddr, err := encConn.PacketConn.ReadFrom(buf)
	if err != nil {
		return n, clientAddr, err
	}
	lenDecrypted, err := DecryptUDPPacket(n, buf, encConn.DerivedPassword)
	return lenDecrypted, clientAddr, err
}

func ListenPacket(network, address string, handleFunc func(net.PacketConn) net.PacketConn) (net.PacketConn, error) {
	c, err := net.ListenPacket(network, address)
	return handleFunc(c), err
}

func DecryptUDPPacket(n int, buf []byte, derivedPassword []byte) (int, error) {
	if len(buf) < LenDerivedPassword {
		return 0, ErrMalformedPacket
	}
	encrypted := buf[:n]
	if len(encrypted) < LenDerivedPassword {
		return 0, ErrMalformedPacket
	}
	salt := encrypted[:LenDerivedPassword]
	cipher, err := AEADBlockCipher(derivedPassword, salt)
	if err != nil {
		return 0, err
	}
	if len(encrypted) < LenDerivedPassword+cipher.Overhead() {
		return 0, ErrMalformedPacket
	}
	decryptDest := buf[LenDerivedPassword:]
	if LenDerivedPassword+len(decryptDest)+cipher.Overhead() < len(encrypted) {
		return 0, io.ErrShortBuffer
	}
	decryptedBuf, err := cipher.Open(decryptDest[:0], ZeroBytes[:cipher.NonceSize()], encrypted[LenDerivedPassword:], nil)
	if err != nil {
		return 0, err
	}
	copy(buf, decryptedBuf)
	return len(decryptedBuf), err
}

func ParseDestAddr(buf []byte) SocksDestAddr {
	if len(buf) < 5 {
		return nil
	}
	var addrLen int
	switch buf[0] {
	case ProxyDestAddrTypeName:
		if len(buf) < 2 {
			return nil
		}
		addrLen = 1 + 1 + int(buf[1]) + 2
	case ProxyDestAddrTypeV4:
		addrLen = 1 + net.IPv4len + 2
	case ProxyDestAddrTypeV6:
		addrLen = 1 + net.IPv6len + 2
	default:
		return nil
	}
	if len(buf) < addrLen {
		return nil
	}
	return buf[:addrLen]
}

func GetSocksAddr(netAddr net.Addr) SocksDestAddr {
	var addr SocksDestAddr
	host, port, err := net.SplitHostPort(netAddr.String())
	if err != nil {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		// IPv4 destination
		if ip4 := ip.To4(); ip4 != nil {
			addr = make([]byte, 1+net.IPv4len+2)
			addr[0] = ProxyDestAddrTypeV4
			copy(addr[1:], ip4)
		} else {
			// IPv6 destination
			addr = make([]byte, 1+net.IPv6len+2)
			addr[0] = ProxyDestAddrTypeV6
			copy(addr[1:], ip)
		}
	} else {
		// Domain name destination
		if len(host) > 254 {
			return nil
		}
		addr = make([]byte, 1+1+len(host)+2)
		addr[0] = ProxyDestAddrTypeName
		addr[1] = byte(len(host))
		copy(addr[2:], host)
	}
	// Turn the port number integer [0,65536) into 2 bytes
	portInt, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil
	}
	// Big endian is the inter-network byte order
	addr[len(addr)-2], addr[len(addr)-1] = byte(portInt>>8), byte(portInt)
	return addr
}
