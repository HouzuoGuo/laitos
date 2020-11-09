package sockd

import (
	"encoding/binary"
	"fmt"
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

// TweakTCPConnection tweaks the TCP connection settings for improved responsiveness.
func TweakTCPConnection(conn *net.TCPConn) {
	_ = conn.SetNoDelay(true)
	_ = conn.SetKeepAlive(true)
	_ = conn.SetKeepAlivePeriod(60 * time.Second)
	_ = conn.SetDeadline(time.Now().Add(time.Duration(IOTimeoutSec * time.Second)))
	_ = conn.SetLinger(5)
}

/*
PipeTCPConnection receives data from the first connection and copies the data into the second connection.
The function returns after the first connection is closed or other IO error occurs, and before returning
the function closes the second connection and optionally writes a random amount of data into the supposedly
already terminated first connection.
*/
func PipeTCPConnection(fromConn, toConn net.Conn, doWriteRand bool) {
	defer func() {
		_ = toConn.Close()
	}()
	buf := make([]byte, MaxPacketSize)
	for {
		if misc.EmergencyLockDown {
			lalog.DefaultLogger.Warning("PipeTCPConnection", "", misc.ErrEmergencyLockDown, "")
			return
		} else if err := fromConn.SetReadDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
			return
		}
		length, err := fromConn.Read(buf)
		if length > 0 {
			if err := toConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
				return
			} else if _, err := toConn.Write(buf[:length]); err != nil {
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

	cipher    *Cipher
	tcpServer *common.TCPServer
}

func (daemon *TCPDaemon) Initialise() error {
	daemon.cipher = &Cipher{}
	daemon.cipher.Initialise(daemon.Password)
	daemon.tcpServer = &common.TCPServer{
		ListenAddr:  daemon.Address,
		ListenPort:  daemon.TCPPort,
		AppName:     "sockd",
		App:         daemon,
		LimitPerSec: daemon.PerIPLimit,
	}
	daemon.tcpServer.Initialise()
	return nil
}

func (daemon *TCPDaemon) GetTCPStatsCollector() *misc.Stats {
	return misc.SOCKDStatsTCP
}

func (daemon *TCPDaemon) HandleTCPConnection(logger lalog.Logger, ip string, client *net.TCPConn) {
	NewTCPCipherConnection(daemon, client, daemon.cipher.Copy(), logger).HandleTCPConnection()
}

func (daemon *TCPDaemon) StartAndBlock() error {
	return daemon.tcpServer.StartAndBlock()
}

func (daemon *TCPDaemon) Stop() {
	daemon.tcpServer.Stop()
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

	n, err = ReadWithRetry(conn.Conn, cipherData)
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
	n, err = WriteWithRetry(conn.Conn, cipherData)

	if n >= headerLen {
		n -= headerLen
	}
	conn.mutex.Unlock()
	return
}

func (conn *TCPCipherConnection) ParseRequest() (destIP net.IP, destNoPort, destWithPort string, err error) {
	if err = conn.SetReadDeadline(time.Now().Add(IOTimeoutSec * time.Second)); err != nil {
		conn.logger.MaybeMinorError(err)
		return
	}

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
		destIP = buf[IPPacketIndex : IPPacketIndex+net.IPv4len]
		destNoPort = destIP.String()
		destWithPort = net.JoinHostPort(destIP.String(), strconv.Itoa(int(port)))
	case AddressTypeIPv6:
		destIP = buf[IPPacketIndex : IPPacketIndex+net.IPv6len]
		destNoPort = destIP.String()
		destWithPort = net.JoinHostPort(destIP.String(), strconv.Itoa(int(port)))
	case AddressTypeDM:
		dest := string(buf[DMAddrIndex : DMAddrIndex+int(buf[DMAddrLengthIndex])])
		destNoPort = dest
		destIP = net.ParseIP(dest)
		destWithPort = net.JoinHostPort(dest, strconv.Itoa(int(port)))
	}
	if strings.ContainsRune(destNoPort, 0) || strings.ContainsRune(destWithPort, 0) {
		err = fmt.Errorf("TCPCipherConnection.ParseRequest: destination must not contain NULL byte")
	}
	return
}

func (conn *TCPCipherConnection) WriteRandAndClose() {
	defer func() {
		_ = conn.Close()
	}()
	WriteRand(conn)
}

func (conn *TCPCipherConnection) HandleTCPConnection() {
	remoteAddr := conn.RemoteAddr().String()
	destIP, destNoPort, destWithPort, err := conn.ParseRequest()
	if err != nil {
		conn.logger.Info("HandleTCPConnection", remoteAddr, nil, "failed to get destination address - %v", err)
		conn.WriteRandAndClose()
		return
	}
	if strings.ContainsRune(destWithPort, 0) {
		conn.logger.Info("HandleTCPConnection", remoteAddr, nil, "will not serve invalid destination address with 0 in it")
		conn.WriteRandAndClose()
		return
	}
	if destIP != nil && IsReservedAddr(destIP) {
		conn.logger.Info("HandleTCPConnection", remoteAddr, nil, "will not serve reserved address %s", destNoPort)
		_ = conn.Close()
		return
	}
	if conn.daemon.DNSDaemon.IsInBlacklist(destNoPort) {
		conn.logger.Info("HandleTCPConnection", remoteAddr, nil, "will not serve blacklisted address %s", destNoPort)
		_ = conn.Close()
		return
	}
	dest, err := net.DialTimeout("tcp", destWithPort, IOTimeoutSec*time.Second)
	if err != nil {
		conn.logger.Info("HandleTCPConnection", remoteAddr, nil, "failed to connect to destination \"%s\" - %v", destWithPort, err)
		_ = conn.Close()
		return
	}
	TweakTCPConnection(conn.Conn.(*net.TCPConn))
	TweakTCPConnection(dest.(*net.TCPConn))
	go PipeTCPConnection(conn, dest, true)
	PipeTCPConnection(dest, conn, false)
}
