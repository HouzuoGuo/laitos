package sockd

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	pseudoRand "math/rand"
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

func WriteRand(conn net.Conn) {
	randBytesWritten := 0
	for i := 0; i < RandNum(0, 1, 2); i++ {
		randBuf := make([]byte, RandNum(70, 120, 200))
		if _, err := pseudoRand.Read(randBuf); err != nil {
			lalog.DefaultLogger.Warning("sockd.WriteRand", conn.RemoteAddr().String(), err, "failed to get random bytes")
			break
		}
		conn.SetWriteDeadline(time.Now().Add(time.Duration(RandNum(330, 540, 880)) * time.Millisecond))
		if n, err := conn.Write(randBuf); err != nil && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "broken") {
			lalog.DefaultLogger.Warning("sockd.WriteRand", conn.RemoteAddr().String(), err, "failed to write random bytes")
			break
		} else {
			randBytesWritten += n
		}
	}
	if pseudoRand.Intn(100) < 3 {
		lalog.DefaultLogger.Info("sockd.WriteRand", conn.RemoteAddr().String(), nil, "wrote %d rand bytes", randBytesWritten)
	}
}

func TweakTCPConnection(conn *net.TCPConn) {
	conn.SetNoDelay(true)
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(60 * time.Second)
}

func PipeTCPConnection(fromConn, toConn net.Conn, doWriteRand bool) {
	defer toConn.Close()
	buf := make([]byte, MaxPacketSize)
	for {
		fromConn.SetReadDeadline(time.Now().Add(IOTimeoutSec))
		length, err := fromConn.Read(buf)
		if length > 0 {
			toConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
			if _, err := toConn.Write(buf[:length]); err != nil {
				return
			}
		}
		if err != nil {
			if doWriteRand {
				WriteRand(fromConn)
			}
			return
		}
	}
}

type TCPDaemon struct {
	Address    string `json:"Address"`
	Password   string `json:"Password"`
	PerIPLimit int    `json:"PerIPLimit"`
	TCPPort    int    `json:"TCPPort"`

	DNSDaemon *dnsd.Daemon `json:"-"` // it is assumed to be already initialised

	tcpListener  net.Listener
	rateLimitTCP *misc.RateLimit

	cipher *Cipher
	logger lalog.Logger
}

func (daemon *TCPDaemon) Initialise() error {
	daemon.logger = lalog.Logger{
		ComponentName: "sockd",
		ComponentID:   []lalog.LoggerIDField{{Key: "TCP", Value: daemon.TCPPort}},
	}
	daemon.rateLimitTCP = &misc.RateLimit{
		Logger:   daemon.logger,
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
	}
	daemon.rateLimitTCP.Initialise()

	daemon.cipher = &Cipher{}
	daemon.cipher.Initialise(daemon.Password)

	return nil
}

func (daemon *TCPDaemon) StartAndBlock() error {
	var err error
	listener, err := net.Listen("tcp", net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.TCPPort)))
	if err != nil {
		return fmt.Errorf("sockd.StartAndBlockTCP: failed to listen on %s:%d - %v", daemon.Address, daemon.TCPPort, err)
	}
	defer listener.Close()
	daemon.logger.Info("StartAndBlockTCP", "", nil, "going to listen for connections")
	daemon.tcpListener = listener

	for {
		if misc.EmergencyLockDown {
			return misc.ErrEmergencyLockDown
		}
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			} else {
				return fmt.Errorf("sockd.StartAndBlockTCP: failed to accept new connection - %v", err)
			}
		}
		clientIP := conn.RemoteAddr().(*net.TCPAddr).IP.String()
		if daemon.rateLimitTCP.Add(clientIP, true) {
			go NewTCPCipherConnection(daemon, conn, daemon.cipher.Copy(), daemon.logger).HandleTCPConnection()
		} else {
			conn.Close()
		}
	}
}

func (daemon *TCPDaemon) Stop() {
	if listener := daemon.tcpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warning("Stop", "", err, "failed to close TCP listener")
		}
	}
}

type TCPCipherConnection struct {
	net.Conn
	*Cipher
	daemon            *TCPDaemon
	mutex             sync.Mutex
	readBuf, writeBuf []byte
	logger            lalog.Logger
}

func NewTCPCipherConnection(daemon *TCPDaemon, netConn net.Conn, cip *Cipher, logger lalog.Logger) *TCPCipherConnection {
	return &TCPCipherConnection{
		Conn:     netConn,
		daemon:   daemon,
		Cipher:   cip,
		readBuf:  make([]byte, MaxPacketSize),
		writeBuf: make([]byte, MaxPacketSize),
		logger:   logger,
	}
}

func (conn *TCPCipherConnection) Close() error {
	return conn.Conn.Close()
}

func (conn *TCPCipherConnection) Read(b []byte) (n int, err error) {
	if conn.DecryptionStream == nil {
		iv := make([]byte, conn.IVLength)
		if _, err = io.ReadFull(conn.Conn, iv); err != nil {
			return
		}
		conn.InitDecryptionStream(iv)
		if len(conn.IV) == 0 {
			conn.IV = iv
		}
	}

	cipherData := conn.readBuf
	if len(b) > len(cipherData) {
		cipherData = make([]byte, len(b))
	} else {
		cipherData = cipherData[:len(b)]
	}

	n, err = conn.Conn.Read(cipherData)
	if n > 0 {
		conn.Decrypt(b[0:n], cipherData[0:n])
	}
	return
}

func (conn *TCPCipherConnection) Write(buf []byte) (n int, err error) {
	conn.mutex.Lock()
	bufSize := len(buf)
	headerLen := len(buf) - bufSize

	var iv []byte
	if conn.EncryptionStream == nil {
		iv = conn.InitEncryptionStream()
	}

	cipherData := conn.writeBuf
	dataSize := len(buf) + len(iv)
	if dataSize > len(cipherData) {
		cipherData = make([]byte, dataSize)
	} else {
		cipherData = cipherData[:dataSize]
	}

	if iv != nil {
		copy(cipherData, iv)
	}

	conn.Encrypt(cipherData[len(iv):], buf)
	n, err = conn.Conn.Write(cipherData)

	if n >= headerLen {
		n -= headerLen
	}
	conn.mutex.Unlock()
	return
}

func (conn *TCPCipherConnection) ParseRequest() (destIP net.IP, destNoPort, destWithPort string, err error) {
	conn.SetReadDeadline(time.Now().Add(IOTimeoutSec))

	buf := make([]byte, 269)
	if _, err = io.ReadFull(conn, buf[:AddressTypeIndex+1]); err != nil {
		return
	}

	var reqStart, reqEnd int
	addrType := buf[AddressTypeIndex]
	maskedType := addrType & AddressTypeMask
	switch maskedType {
	case AddressTypeIPv4:
		reqStart, reqEnd = IPPacketIndex, IPPacketIndex+IPv4PacketLength
	case AddressTypeIPv6:
		reqStart, reqEnd = IPPacketIndex, IPPacketIndex+IPv6PacketLength
	case AddressTypeDM:
		if _, err = io.ReadFull(conn, buf[AddressTypeIndex+1:DMAddrLengthIndex+1]); err != nil {
			return
		}
		reqStart, reqEnd = DMAddrIndex, DMAddrIndex+int(buf[DMAddrLengthIndex])+DMAddrHeaderLength
	default:
		err = fmt.Errorf("TCPCipherConnection.ParseRequest: unknown mask type %d", maskedType)
		return
	}

	if _, err = io.ReadFull(conn, buf[reqStart:reqEnd]); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(buf[reqEnd-2 : reqEnd])
	if port < 1 {
		err = fmt.Errorf("TCPCipherConnection.ParseRequest: invalid destination port %d", port)
		return
	}

	switch maskedType {
	case AddressTypeIPv4:
		destIP = net.IP(buf[IPPacketIndex : IPPacketIndex+net.IPv4len])
		destNoPort = destIP.String()
		destWithPort = net.JoinHostPort(destIP.String(), strconv.Itoa(int(port)))
	case AddressTypeIPv6:
		destIP = net.IP(buf[IPPacketIndex : IPPacketIndex+net.IPv6len])
		destNoPort = destIP.String()
		destWithPort = net.JoinHostPort(destIP.String(), strconv.Itoa(int(port)))
	case AddressTypeDM:
		dest := string(buf[DMAddrIndex : DMAddrIndex+int(buf[DMAddrLengthIndex])])
		destNoPort = dest
		destIP = net.ParseIP(dest)
		destWithPort = net.JoinHostPort(dest, strconv.Itoa(int(port)))
	}
	return
}

func (conn *TCPCipherConnection) WriteRandAndClose() {
	defer conn.Close()
	randBuf := make([]byte, RandNum(20, 70, 200))
	_, err := rand.Read(randBuf)
	if err != nil {
		conn.logger.Warning("WriteRandAndClose", conn.Conn.RemoteAddr().String(), err, "failed to get random bytes")
		return
	}
	conn.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
	if _, err := conn.Write(randBuf); err != nil {
		conn.logger.Warning("WriteRandAndClose", conn.Conn.RemoteAddr().String(), err, "failed to write random bytes")
	}
}

func (conn *TCPCipherConnection) HandleTCPConnection() {
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		common.SOCKDStatsTCP.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	remoteAddr := conn.RemoteAddr().String()
	destIP, destNoPort, destWithPort, err := conn.ParseRequest()
	if err != nil {
		conn.logger.Warning("HandleTCPConnection", remoteAddr, err, "failed to get destination address")
		conn.WriteRandAndClose()
		return
	}
	if strings.ContainsRune(destWithPort, 0x00) {
		conn.logger.Warning("HandleTCPConnection", remoteAddr, nil, "will not serve invalid destination address with 0 in it")
		conn.WriteRandAndClose()
		return
	}
	if destIP != nil && IsReservedAddr(destIP) {
		conn.logger.Info("HandleTCPConnection", remoteAddr, nil, "will not serve reserved address %s", destNoPort)
		conn.Close()
		return
	}
	if conn.daemon.DNSDaemon.IsInBlacklist(destNoPort) {
		conn.logger.Info("HandleTCPConnection", remoteAddr, nil, "will not serve blacklisted address %s", destNoPort)
		conn.Close()
		return
	}
	dest, err := net.DialTimeout("tcp", destWithPort, IOTimeoutSec)
	if err != nil {
		conn.logger.Warning("HandleTCPConnection", remoteAddr, err, "failed to connect to destination \"%s\"", destWithPort)
		conn.Close()
		return
	}
	TweakTCPConnection(conn.Conn.(*net.TCPConn))
	TweakTCPConnection(dest.(*net.TCPConn))
	go PipeTCPConnection(conn, dest, true)
	PipeTCPConnection(dest, conn, false)
	return
}
