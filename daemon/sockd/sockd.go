package sockd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"io"
	pseudoRand "math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MD5SumLength         = 16
	IOTimeoutSec         = time.Duration(60 * time.Second)
	RateLimitIntervalSec = 1
	MaxPacketSize        = 9038
)

// Daemon is intentionally undocumented magic ^____^
type Daemon struct {
	Address    string `json:"Address"`
	Password   string `json:"Password"`
	PerIPLimit int    `json:"PerIPLimit"`
	TCPPort    int    `json:"TCPPort"`
	UDPPort    int    `json:"UDPPort"`

	DNSDaemon *dnsd.Daemon `json:"-"` // it is assumed to be already initialised

	tcpListener  net.Listener
	rateLimitTCP *misc.RateLimit

	udpBackLog       *UDPBackLog
	udpListener      *net.UDPConn
	udpTable         *UDPTable
	rateLimitUDP     *misc.RateLimit
	udpLoopIsRunning int32
	stopUDP          chan bool

	cipher *Cipher
	logger misc.Logger
}

func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 288
	}
	daemon.logger = misc.Logger{
		ComponentName: "sockd",
		ComponentID:   []misc.LoggerIDField{{"Addr", daemon.Address}, {"TCP", daemon.TCPPort}, {"UDP", daemon.UDPPort}},
	}
	if daemon.DNSDaemon == nil {
		return errors.New("sockd.Initialise: dns daemon must be assigned")
	}
	if daemon.TCPPort < 1 {
		return errors.New("sockd.Initialise: TCP listen port must be greater than 0")
	}
	if len(daemon.Password) < 7 {
		return errors.New("sockd.Initialise: password must be at least 7 characters long")
	}
	daemon.rateLimitTCP = &misc.RateLimit{
		Logger:   daemon.logger,
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
	}
	daemon.rateLimitTCP.Initialise()
	daemon.rateLimitUDP = &misc.RateLimit{
		Logger:   daemon.logger,
		MaxCount: daemon.PerIPLimit * 100,
		UnitSecs: RateLimitIntervalSec,
	}
	daemon.rateLimitUDP.Initialise()

	daemon.cipher = &Cipher{}
	daemon.cipher.Initialise(daemon.Password)

	daemon.udpBackLog = &UDPBackLog{backlog: make(map[string][]byte), mutex: new(sync.Mutex)}

	daemon.stopUDP = make(chan bool)
	return nil
}

func (daemon *Daemon) StartAndBlock() error {
	numListeners := 0
	errChan := make(chan error, 2)
	if daemon.TCPPort != 0 {
		numListeners++
		go func() {
			err := daemon.StartAndBlockTCP()
			errChan <- err
		}()
	}
	if daemon.UDPPort != 0 {
		numListeners++
		go func() {
			err := daemon.StartAndBlockUDP()
			errChan <- err
		}()
	}
	for i := 0; i < numListeners; i++ {
		if err := <-errChan; err != nil {
			daemon.Stop()
			return err
		}
	}
	return nil
}

func (daemon *Daemon) Stop() {
	if listener := daemon.tcpListener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warning("Stop", "", err, "failed to close TCP listener")
		}
	}
	if listener := daemon.udpListener; listener != nil {
		if atomic.CompareAndSwapInt32(&daemon.udpLoopIsRunning, 1, 0) {
			daemon.stopUDP <- true
		}
		if err := listener.Close(); err != nil {
			daemon.logger.Warning("Stop", "", err, "failed to close UDP listener")
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

var randSeed = int(time.Now().UnixNano())

func RandNum(absMin, variableLower, randMore int) int {
	return absMin + randSeed%variableLower + pseudoRand.Intn(randMore)
}

func TestSockd(sockd *Daemon, t testingstub.T) {
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
