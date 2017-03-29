package sockd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"io"
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

	MaxPacketSize         = 9038
	IPPacketIndex         = 1
	IPv4PacketLength      = net.IPv4len + 2
	IPv6PacketLength      = net.IPv6len + 2
	DMPacketIndex         = 2
	DMPacketLength        = 1
	DMPacketPaddingLength = 2

	MD5SumLength         = 16
	IOTimeoutSec         = time.Duration(120 * time.Second)
	RateLimitIntervalSec = 10
)

// Intentionally undocumented magic, please move along.
type Sockd struct {
	ListenAddress string               `json:"ListenAddress"`
	ListenPort    int                  `json:"ListenPort"`
	Password      string               `json:"Password"`
	PerIPLimit    int                  `json:"PerIPLimit"`
	Listener      net.Listener         `json:"-"`
	Logger        global.Logger        `json:"-"`
	cipher        *Cipher              `json:"-"`
	rateLimit     *ratelimit.RateLimit `json:"-"`
}

func (sock *Sockd) Initialise() error {
	if sock.ListenAddress == "" {
		return errors.New("Sockd.Initialise: listen address must not be empty")
	}
	if sock.ListenPort < 1 {
		return errors.New("Sockd.Initialise: listen port must be greater than 0")
	}
	if len(sock.Password) < 7 {
		return errors.New("Sockd.Initialise: password must be at least 7 characters long")
	}
	if sock.PerIPLimit < 10 {
		return errors.New("Sockd.Initialise: PerIPLimit must be greater than 9")
	}
	sock.cipher = &Cipher{}
	sock.cipher.Initialise(sock.Password)
	sock.rateLimit = &ratelimit.RateLimit{
		Logger:   sock.Logger,
		MaxCount: sock.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
	}
	sock.rateLimit.Initialise()
	return nil
}

func (sock *Sockd) StartAndBlock() error {
	sock.Logger.Printf("StartAndBlock", "", nil, "going to listen for connections")
	var err error
	sock.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", sock.ListenAddress, sock.ListenPort))
	if err != nil {
		return fmt.Errorf("Sockd.StartAndBlock: failed to listen on %s:%d - %v", sock.ListenAddress, sock.ListenPort, err)
	}
	for {
		if global.EmergencyStop {
			return global.ErrEmergencyStop
		}
		conn, err := sock.Listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			} else {
				return fmt.Errorf("Sockd.StartAndBlock: failed to accept new connection - %v", err)
			}
		}
		clientIP := conn.RemoteAddr().String()[:strings.LastIndexByte(conn.RemoteAddr().String(), ':')]
		if sock.rateLimit.Add(clientIP, true) {
			go NewCipherConnection(conn, sock.cipher.Copy(), sock.Logger).HandleAndCloseConnection()
		} else {
			conn.Close()
		}
	}
}

func (sock *Sockd) Stop() {
	if sock.Listener != nil {
		if err := sock.Listener.Close(); err != nil {
			sock.Logger.Printf("Stop", "", err, "failed to close listener")
		}
	}
}

type Cipher struct {
	EncryptionStream cipher.Stream
	DecryptionStream cipher.Stream
	Key              []byte
	IV               []byte
	KeyLength        int
	IVLength         int
}

func md5Sum(d []byte) []byte {
	md5Digest := md5.New()
	md5Digest.Write(d)
	return md5Digest.Sum(nil)
}

func (cip *Cipher) Initialise(password string) {
	cip.KeyLength = 32
	cip.IVLength = 16

	segmentLength := (cip.KeyLength-1)/MD5SumLength + 1
	buf := make([]byte, segmentLength*MD5SumLength)
	copy(buf, md5Sum([]byte(password)))
	destinationBuf := make([]byte, MD5SumLength+len(password))
	start := 0
	for i := 1; i < segmentLength; i++ {
		start += MD5SumLength
		copy(destinationBuf, buf[start-MD5SumLength:start])
		copy(destinationBuf[MD5SumLength:], password)
		copy(buf[start:], md5Sum(destinationBuf))
	}
	cip.Key = buf[:cip.KeyLength]
}

func (cip *Cipher) GetCipherStream(key, iv []byte) (cipher.Stream, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewCTR(block, iv), nil
}

func (cip *Cipher) InitEncryptionStream() (iv []byte) {
	var err error
	if cip.IV == nil {
		iv = make([]byte, cip.IVLength)
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			panic(err)
		}
		cip.IV = iv
	} else {
		iv = cip.IV
	}
	cip.EncryptionStream, err = cip.GetCipherStream(cip.Key, iv)
	if err != nil {
		panic(err)
	}
	return
}

func (cip *Cipher) Encrypt(dest, src []byte) {
	cip.EncryptionStream.XORKeyStream(dest, src)
}

func (cip *Cipher) InitDecryptionStream(iv []byte) {
	var err error
	cip.DecryptionStream, err = cip.GetCipherStream(cip.Key, iv)
	if err != nil {
		panic(err)
	}
}

func (cip *Cipher) Decrypt(dest, src []byte) {
	cip.DecryptionStream.XORKeyStream(dest, src)
}

func (cip *Cipher) Copy() *Cipher {
	newCipher := *cip
	newCipher.EncryptionStream = nil
	newCipher.DecryptionStream = nil
	return &newCipher
}

type CipherConnection struct {
	net.Conn
	*Cipher
	readBuf, writeBuf []byte
	logger            global.Logger
}

func NewCipherConnection(netConn net.Conn, cip *Cipher, logger global.Logger) *CipherConnection {
	return &CipherConnection{
		Conn:     netConn,
		Cipher:   cip,
		readBuf:  make([]byte, MaxPacketSize),
		writeBuf: make([]byte, MaxPacketSize),
		logger:   logger,
	}
}

func (conn *CipherConnection) Close() error {
	return conn.Conn.Close()
}

func (conn *CipherConnection) Read(b []byte) (n int, err error) {
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

func (conn *CipherConnection) Write(buf []byte) (n int, err error) {
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

func (conn *CipherConnection) ParseRequest() (destAddr string, err error) {
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
		if _, err = io.ReadFull(conn, buf[AddressTypeIndex+1:DMPacketLength+1]); err != nil {
			return
		}
		reqStart, reqEnd = DMPacketIndex, DMPacketIndex+int(buf[DMPacketLength])+DMPacketPaddingLength
	default:
		err = fmt.Errorf("Unknown type %d", maskedType)
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
		destAddr = string(buf[DMPacketIndex : DMPacketIndex+int(buf[DMPacketLength])])
	}
	port := binary.BigEndian.Uint16(buf[reqEnd-2 : reqEnd])
	destAddr = net.JoinHostPort(destAddr, strconv.Itoa(int(port)))
	return
}

func (conn *CipherConnection) HandleAndCloseConnection() {
	remoteAddr := conn.RemoteAddr().String()

	defer func() {
		conn.Close()
	}()

	destAddr, err := conn.ParseRequest()
	if err != nil {
		conn.logger.Printf("HandleAndCloseConnection", remoteAddr, err, "failed to get destination address")
		return
	}
	if strings.ContainsRune(destAddr, 0x00) {
		conn.logger.Printf("HandleAndCloseConnection", remoteAddr, err, "will not serve invalid destination address with 0 in it")
		return
	}
	dest, err := net.DialTimeout("tcp", destAddr, IOTimeoutSec)
	if err != nil {
		conn.logger.Printf("HandleAndCloseConnection", remoteAddr, err, "failed to connect to destination \"%s\"", destAddr)
		return
	}
	defer dest.Close()
	go PipeAndCloseConnection(conn, dest)
	PipeAndCloseConnection(dest, conn)
	return
}

func PipeAndCloseConnection(fromConn, toConn net.Conn) {
	defer toConn.Close()
	buf := make([]byte, MaxPacketSize)
	for {
		fromConn.SetReadDeadline(time.Now().Add(IOTimeoutSec))
		n, err := fromConn.Read(buf)
		if n > 0 {
			toConn.SetWriteDeadline(time.Now().Add(IOTimeoutSec))
			if _, err := toConn.Write(buf[0:n]); err != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
