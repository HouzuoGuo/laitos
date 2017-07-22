package sockd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/global"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	MD5SumLength         = 16
	IOTimeoutSec         = time.Duration(120 * time.Second)
	RateLimitIntervalSec = 10
	MaxPacketSize        = 9038
)

// Sockd is intentionally undocumented magic ^____^
type Sockd struct {
	Address    string `json:"Address"`
	Password   string `json:"Password"`
	PerIPLimit int    `json:"PerIPLimit"`

	TCPPort     int          `json:"TCPPort"`
	TCPListener net.Listener `json:"-"`

	UDPPort     int          `json:"UDPPort"`
	UDPBacklog  *UDPBackLog  `json:"-"`
	UDPListener *net.UDPConn `json:"-"`
	UDPTable    *UDPTable    `json:"-"`

	Logger           global.Logger  `json:"-"`
	cipher           *Cipher        `json:"-"`
	rateLimitTCP     *env.RateLimit `json:"-"`
	rateLimitUDP     *env.RateLimit `json:"-"`
	udpLoopIsRunning int32
	stopUDP          chan bool
}

func (sock *Sockd) Initialise() error {
	sock.Logger = global.Logger{ComponentName: "Sockd", ComponentID: fmt.Sprintf("%s:%d", sock.Address, sock.TCPPort)}
	if sock.Address == "" {
		return errors.New("Sockd.Initialise: listen address must not be empty")
	}
	if sock.TCPPort < 1 {
		return errors.New("Sockd.Initialise: listen port must be greater than 0")
	}
	if len(sock.Password) < 7 {
		return errors.New("Sockd.Initialise: password must be at least 7 characters long")
	}
	if sock.PerIPLimit < 10 {
		return errors.New("Sockd.Initialise: PerIPLimit must be greater than 9")
	}
	sock.rateLimitTCP = &env.RateLimit{
		Logger:   sock.Logger,
		MaxCount: sock.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
	}
	sock.rateLimitTCP.Initialise()
	sock.rateLimitUDP = &env.RateLimit{
		Logger:   sock.Logger,
		MaxCount: sock.PerIPLimit * 100,
		UnitSecs: RateLimitIntervalSec,
	}
	sock.rateLimitUDP.Initialise()

	sock.cipher = &Cipher{}
	sock.cipher.Initialise(sock.Password)

	sock.UDPBacklog = &UDPBackLog{backlog: make(map[string][]byte), mutex: new(sync.Mutex)}

	sock.stopUDP = make(chan bool)
	return nil
}

func (sock *Sockd) StartAndBlock() error {
	numListeners := 0
	errChan := make(chan error, 2)
	if sock.TCPPort != 0 {
		numListeners++
		go func() {
			err := sock.StartAndBlockTCP()
			errChan <- err
		}()
	}
	if sock.UDPPort != 0 {
		numListeners++
		go func() {
			err := sock.StartAndBlockUDP()
			errChan <- err
		}()
	}
	if numListeners == 0 {
		return fmt.Errorf("Sockd.StartAndBlock: neither TCP nor UDP listen port is defined, the daemon will not start.")
	}
	for i := 0; i < numListeners; i++ {
		if err := <-errChan; err != nil {
			sock.Stop()
			return err
		}
	}
	return nil
}

func (sock *Sockd) Stop() {
	if listener := sock.TCPListener; listener != nil {
		if err := listener.Close(); err != nil {
			sock.Logger.Warningf("Stop", "", err, "failed to close TCP listener")
		}
	}
	if listener := sock.UDPListener; listener != nil {
		if atomic.CompareAndSwapInt32(&sock.udpLoopIsRunning, 1, 0) {
			sock.stopUDP <- true
		}
		if err := listener.Close(); err != nil {
			sock.Logger.Warningf("Stop", "", err, "failed to close UDP listener")
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

func TestSockd(sockd *Sockd, t *testing.T) {
	var stopped bool
	go func() {
		if err := sockd.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stopped = true
	}()
	time.Sleep(2 * time.Second)
	if conn, err := net.Dial("tcp", sockd.Address+":"+strconv.Itoa(sockd.TCPPort)); err != nil {
		t.Fatal(err)
	} else if n, err := conn.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil && n != 10 {
		t.Fatal(err, n)
	}
	// Daemon should stop within a second
	sockd.Stop()
	time.Sleep(1 * time.Second)
	if !stopped {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	sockd.Stop()
	sockd.Stop()
}
