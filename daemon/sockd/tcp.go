package sockd

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

type TCPDaemon struct {
	Address    string `json:"Address"`
	Password   string `json:"Password"`
	PerIPLimit int    `json:"PerIPLimit"`
	TCPPort    int    `json:"TCPPort"`

	DNSDaemon *dnsd.Daemon `json:"-"` // it is assumed to be already initialised

	derivedPassword []byte
	tcpServer       *common.TCPServer
}

func (daemon *TCPDaemon) Initialise() error {
	daemon.tcpServer = &common.TCPServer{
		ListenAddr:  daemon.Address,
		ListenPort:  daemon.TCPPort,
		AppName:     "sockd",
		App:         daemon,
		LimitPerSec: daemon.PerIPLimit,
	}
	daemon.tcpServer.Initialise()
	daemon.derivedPassword = GetDerivedKey(daemon.Password)
	return nil
}

func (daemon *TCPDaemon) GetTCPStatsCollector() *misc.Stats {
	return misc.SOCKDStatsTCP
}

func (daemon *TCPDaemon) HandleTCPConnection(logger lalog.Logger, ip string, client *net.TCPConn) {
	logger.MaybeMinorError(client.SetReadDeadline(time.Now().Add(IOTimeout)))
	encryptedClientConn := &EncryptedTCPConn{Conn: client, DerivedPassword: daemon.derivedPassword}
	proxyDestAddr, err := ReadProxyDestAddr(encryptedClientConn, make([]byte, LenProxyConnectRequest))
	if err != nil {
		logger.Info("HandleTCPConnection", ip, nil, "failed to get destination address - %v", err)
		WriteRandomToTCP(client)
		return
	}
	destNameOrIP, destPort := proxyDestAddr.HostPort()
	if destNameOrIP == "" || destPort == 0 || strings.ContainsRune(destNameOrIP, 0) {
		logger.Info("HandleTCPConnection", ip, nil, "invalid destination IP (%s) or port (%d)", destNameOrIP, destPort)
		WriteRandomToTCP(client)
		return
	}
	if parsedIP := net.ParseIP(destNameOrIP); parsedIP != nil {
		if IsReservedAddr(parsedIP) {
			logger.Info("HandleTCPConnection", ip, nil, "will not serve reserved address %s", destNameOrIP)
			return
		}
	}
	if daemon.DNSDaemon.IsInBlacklist(destNameOrIP) {
		logger.Info("HandleTCPConnection", ip, nil, "will not serve blacklisted destination %s", destNameOrIP)
		return
	}
	proxyDestConn, err := net.Dial("tcp", net.JoinHostPort(destNameOrIP, strconv.Itoa(destPort)))
	if err != nil {
		logger.Info("HandleTCPConnection", ip, err, "failed to connect to destination \"%s:%d\"", destNameOrIP, destPort)
		return
	}
	misc.TweakTCPConnection(encryptedClientConn.Conn.(*net.TCPConn), IOTimeout)
	misc.TweakTCPConnection(proxyDestConn.(*net.TCPConn), IOTimeout)
	go PipeTCPConnection(encryptedClientConn, proxyDestConn, true)
	PipeTCPConnection(proxyDestConn, encryptedClientConn, false)
}

func (daemon *TCPDaemon) StartAndBlock() error {
	return daemon.tcpServer.StartAndBlock()
}

func (daemon *TCPDaemon) Stop() {
	daemon.tcpServer.Stop()
}

func ReadProxyDestAddr(client io.Reader, destWithPort []byte) (addr SocksDestAddr, err error) {
	// Read type (1 byte)
	_, err = io.ReadFull(client, destWithPort[:1])
	if err != nil {
		return nil, err
	}
	switch destWithPort[0] {
	case ProxyDestAddrTypeName:
		// Read length (1 byte)
		_, err = io.ReadFull(client, destWithPort[1:2])
		if err != nil {
			return nil, err
		}
		// Read name (length + 2 bytes)
		_, err = io.ReadFull(client, destWithPort[2:2+int(destWithPort[1])+2])
		addr = destWithPort[:1+1+int(destWithPort[1])+2]
	case ProxyDestAddrTypeV4:
		// Read IPv4 address (4 bytes).
		_, err = io.ReadFull(client, destWithPort[1:1+net.IPv4len+2])
		addr = destWithPort[:1+net.IPv4len+2]
	case ProxyDestAddrTypeV6:
		// Read IPv6 address (16 bytes).
		_, err = io.ReadFull(client, destWithPort[1:1+net.IPv6len+2])
		addr = destWithPort[:1+net.IPv6len+2]
	default:
		err = errors.New("unsupported proxy destination address type")
	}
	return
}

type EncryptedWriter struct {
	io.Writer
	cipher.AEAD
	nonce []byte
	buf   []byte
}

func NewEncryptedWriter(writer io.Writer, blockCipher cipher.AEAD) *EncryptedWriter {
	return &EncryptedWriter{
		AEAD:   blockCipher,
		Writer: writer,
		buf:    make([]byte, LenPayloadSize+blockCipher.Overhead()+PayloadSizeMask+blockCipher.Overhead()),
		nonce:  make([]byte, blockCipher.NonceSize()),
	}
}

func (writer *EncryptedWriter) Write(buf []byte) (int, error) {
	n, err := writer.ReadFrom(bytes.NewBuffer(buf))
	return int(n), err
}

func (writer *EncryptedWriter) ReadFrom(reader io.Reader) (n int64, err error) {
	for {
		buf := writer.buf
		payloadBuf := buf[LenPayloadSize+writer.Overhead() : LenPayloadSize+writer.Overhead()+PayloadSizeMask]
		readLen, readErr := reader.Read(payloadBuf)
		if readLen > 0 {
			n += int64(readLen)
			buf = buf[:LenPayloadSize+writer.Overhead()+readLen+writer.Overhead()]
			payloadBuf = payloadBuf[:readLen]
			buf[0], buf[1] = byte(readLen>>8), byte(readLen)
			writer.Seal(buf[:0], writer.nonce, buf[:LenPayloadSize], nil)
			IncreaseNounce(writer.nonce)
			writer.Seal(payloadBuf[:0], writer.nonce, payloadBuf, nil)
			IncreaseNounce(writer.nonce)
			if _, writeErr := writer.Writer.Write(buf); writeErr != nil {
				err = writeErr
				break
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				err = readErr
			}
			break
		}
	}
	return
}

type EncryptedReader struct {
	io.Reader
	cipher.AEAD
	nonce     []byte
	buf       []byte
	remaining []byte
}

func NewEncryptedReader(reader io.Reader, blockCipher cipher.AEAD) *EncryptedReader {
	return &EncryptedReader{
		Reader: reader,
		AEAD:   blockCipher,
		buf:    make([]byte, PayloadSizeMask+blockCipher.Overhead()),
		nonce:  make([]byte, blockCipher.NonceSize()),
	}
}

func (reader *EncryptedReader) read() (int, error) {
	buf := reader.buf[:LenPayloadSize+reader.Overhead()]
	_, err := io.ReadFull(reader.Reader, buf)
	if err != nil {
		return 0, err
	}
	_, err = reader.Open(buf[:0], reader.nonce, buf, nil)
	IncreaseNounce(reader.nonce)
	if err != nil {
		return 0, err
	}
	n := (int(buf[0])<<8 + int(buf[1])) & PayloadSizeMask
	buf = reader.buf[:n+reader.Overhead()]
	_, err = io.ReadFull(reader.Reader, buf)
	if err != nil {
		return 0, err
	}
	_, err = reader.Open(buf[:0], reader.nonce, buf, nil)
	IncreaseNounce(reader.nonce)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (reader *EncryptedReader) Read(buf []byte) (int, error) {
	if len(reader.remaining) > 0 {
		n := copy(buf, reader.remaining)
		reader.remaining = reader.remaining[n:]
		return n, nil
	}
	readLen, err := reader.read()
	copiedLen := copy(buf, reader.buf[:readLen])
	if copiedLen < readLen {
		reader.remaining = reader.buf[copiedLen:readLen]
	}
	return copiedLen, err
}

func (reader *EncryptedReader) WriteTo(writer io.Writer) (n int64, err error) {
	for len(reader.remaining) > 0 {
		readLen, readErr := writer.Write(reader.remaining)
		reader.remaining = reader.remaining[readLen:]
		n += int64(readLen)
		if readErr != nil {
			return n, readErr
		}
	}
	for {
		readLen, readErr := reader.read()
		if readLen > 0 {
			writeLen, writeErr := writer.Write(reader.buf[:readLen])
			n += int64(writeLen)
			if writeErr != nil {
				err = writeErr
				break
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				err = readErr
			}
			break
		}
	}
	return n, err
}

func IncreaseNounce(nounceBuf []byte) {
	for i := range nounceBuf {
		nounceBuf[i]++
		if nounceBuf[i] != 0 {
			return
		}
	}
}

type EncryptedTCPConn struct {
	net.Conn
	DerivedPassword []byte
	reader          *EncryptedReader
	writer          *EncryptedWriter
}

func (conn *EncryptedTCPConn) Initialise() error {
	salt := make([]byte, LenDerivedPassword)
	if _, err := io.ReadFull(conn.Conn, salt); err != nil {
		return err
	}
	aead, err := AEADBlockCipher(conn.DerivedPassword, salt)
	if err != nil {
		return err
	}
	conn.reader = NewEncryptedReader(conn.Conn, aead)
	return nil
}

func (conn *EncryptedTCPConn) Read(buf []byte) (int, error) {
	if conn.reader == nil {
		if err := conn.Initialise(); err != nil {
			return 0, err
		}
	}
	return conn.reader.Read(buf)
}

func (conn *EncryptedTCPConn) WriteTo(writer io.Writer) (int64, error) {
	if conn.reader == nil {
		if err := conn.Initialise(); err != nil {
			return 0, err
		}
	}
	return conn.reader.WriteTo(writer)
}

func (conn *EncryptedTCPConn) InitialiseWriter() error {
	salt := make([]byte, LenDerivedPassword)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	aead, err := AEADBlockCipher(conn.DerivedPassword, salt)
	if err != nil {
		return err
	}
	_, err = conn.Conn.Write(salt)
	if err != nil {
		return err
	}
	conn.writer = NewEncryptedWriter(conn.Conn, aead)
	return nil
}

func (conn *EncryptedTCPConn) Write(buf []byte) (int, error) {
	if conn.writer == nil {
		if err := conn.InitialiseWriter(); err != nil {
			return 0, err
		}
	}
	return conn.writer.Write(buf)
}

func (conn *EncryptedTCPConn) ReadFrom(reader io.Reader) (int64, error) {
	if conn.writer == nil {
		if err := conn.InitialiseWriter(); err != nil {
			return 0, err
		}
	}
	return conn.writer.ReadFrom(reader)
}
