package sockd

import (
	cryptRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"io"
	pseudoRand "math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	AddressTypeMask  byte = 0xf
	AddressTypeIndex      = 0
	AddressTypeIPv4       = 1
	AddressTypeDM         = 3
	AddressTypeIPv6       = 4

	IPPacketIndex    = 1
	IPv4PacketLength = net.IPv4len + 2
	IPv6PacketLength = net.IPv6len + 2

	DMAddrIndex        = 2
	DMAddrLengthIndex  = 1
	DMAddrHeaderLength = 2
)

var TCPDurationStats = misc.NewStats()

func (sock *Daemon) StartAndBlockTCP() error {
	var err error
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", sock.Address, sock.TCPPort))
	if err != nil {
		return fmt.Errorf("sockd.StartAndBlockTCP: failed to listen on %s:%d - %v", sock.Address, sock.TCPPort, err)
	}
	defer listener.Close()
	sock.Logger.Printf("StartAndBlockTCP", "", nil, "going to listen for connections")
	sock.TCPListener = listener

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
		if sock.rateLimitTCP.Add(clientIP, true) {
			go NewTCPCipherConnection(conn, sock.cipher.Copy(), sock.Logger).HandleTCPConnection()
		} else {
			conn.Close()
		}
	}
}

type TCPCipherConnection struct {
	net.Conn
	*Cipher
	readBuf, writeBuf []byte
	logger            misc.Logger
}

func NewTCPCipherConnection(netConn net.Conn, cip *Cipher, logger misc.Logger) *TCPCipherConnection {
	return &TCPCipherConnection{
		Conn:     netConn,
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
	return
}

func (conn *TCPCipherConnection) ParseRequest() (destAddr string, err error) {
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

	switch maskedType {
	case AddressTypeIPv4:
		destAddr = net.IP(buf[IPPacketIndex : IPPacketIndex+net.IPv4len]).String()
	case AddressTypeIPv6:
		destAddr = net.IP(buf[IPPacketIndex : IPPacketIndex+net.IPv6len]).String()
	case AddressTypeDM:
		destAddr = string(buf[DMAddrIndex : DMAddrIndex+int(buf[DMAddrLengthIndex])])
	}
	port := binary.BigEndian.Uint16(buf[reqEnd-2 : reqEnd])
	destAddr = net.JoinHostPort(destAddr, strconv.Itoa(int(port)))
	return
}

func (conn *TCPCipherConnection) WriteRandAndClose() {
	defer conn.Close()
	randBuf := make([]byte, RandNum(1, 20, 300))
	_, err := cryptRand.Read(randBuf)
	if err != nil {
		conn.logger.Warningf("WriteRandAndClose", conn.Conn.RemoteAddr().String(), err, "failed to get random bytes")
		return
	}
	conn.SetDeadline(time.Now().Add(IOTimeoutSec))
	if _, err := conn.Write(randBuf); err != nil {
		conn.logger.Warningf("WriteRandAndClose", conn.Conn.RemoteAddr().String(), err, "failed to write random bytes")
	}
}

func (conn *TCPCipherConnection) HandleTCPConnection() {
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		TCPDurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	remoteAddr := conn.RemoteAddr().String()
	destAddr, err := conn.ParseRequest()
	if err != nil {
		conn.logger.Warningf("HandleTCPConnection", remoteAddr, err, "failed to get destination address")
		conn.WriteRandAndClose()
		return
	}
	if strings.ContainsRune(destAddr, 0x00) {
		conn.logger.Warningf("HandleTCPConnection", remoteAddr, nil, "will not serve invalid destination address with 0 in it")
		conn.WriteRandAndClose()
		return
	}
	dest, err := net.DialTimeout("tcp", destAddr, IOTimeoutSec)
	if err != nil {
		conn.logger.Warningf("HandleTCPConnection", remoteAddr, err, "failed to connect to destination \"%s\"", destAddr)
		conn.Close()
		return
	}
	go PipeTCPConnection(conn, dest, true)
	PipeTCPConnection(dest, conn, false)
	return
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

func WriteRand(conn net.Conn) {
	randBytesWritten := 0
	for i := 0; i < RandNum(1, 2, 3); i++ {
		randBuf := make([]byte, RandNum(4, 50, 600))
		if _, err := pseudoRand.Read(randBuf); err != nil {
			misc.DefaultLogger.Warningf("sockd.WriteRand", conn.RemoteAddr().String(), err, "failed to get random bytes")
			break
		}
		conn.SetDeadline(time.Now().Add(time.Duration(RandNum(7, 80, 900)) * time.Millisecond))
		if n, err := conn.Write(randBuf); err != nil && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "broken") {
			misc.DefaultLogger.Warningf("sockd.WriteRand", conn.RemoteAddr().String(), err, "failed to write random bytes")
			break
		} else {
			randBytesWritten += n
		}
	}
	if pseudoRand.Intn(100) < 2 {
		misc.DefaultLogger.Printf("sockd.WriteRand", conn.RemoteAddr().String(), nil, "wrote %d rand bytes", randBytesWritten)
	}
}
