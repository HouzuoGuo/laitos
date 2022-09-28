package sockd

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/testingstub"
	"golang.org/x/crypto/hkdf"
)

const (
	IOTimeout              = 120 * time.Second
	PayloadSizeMask        = 16*1024 - 1
	LenPayloadSize         = 2
	LenDerivedPassword     = 32
	MaxPacketSize          = 64 * 1024
	MagicKeyDerivationInfo = "ss-subkey"
	ProxyDestAddrTypeV4    = 1
	ProxyDestAddrTypeName  = 3
	ProxyDestAddrTypeV6    = 4
	LenProxyConnectRequest = 1 + 1 + 1 + 254 + 2
)

var (
	ZeroBytes           [128]byte
	RandSeed            = int(time.Now().UnixNano())
	BlockedReservedCIDR = []net.IPNet{
		{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(32, 32)},
		{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
		{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)},
		{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
		{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)},
		{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},
		{IP: net.IPv4(192, 0, 0, 0), Mask: net.CIDRMask(24, 32)},
		{IP: net.IPv4(192, 0, 2, 0), Mask: net.CIDRMask(24, 32)},
		{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},
		{IP: net.IPv4(198, 18, 0, 0), Mask: net.CIDRMask(15, 32)},
		{IP: net.IPv4(198, 51, 100, 0), Mask: net.CIDRMask(24, 32)},
		{IP: net.IPv4(203, 0, 113, 0), Mask: net.CIDRMask(24, 32)},
		{IP: net.IPv4(240, 0, 0, 0), Mask: net.CIDRMask(4, 32)},
	}
	ErrMalformedPacket = errors.New("received a malformed packet")
)

type SocksDestAddr []byte

func (addr SocksDestAddr) HostPort() (nameOrIP string, port int) {
	switch addr[0] {
	case ProxyDestAddrTypeName:
		nameOrIP = string(addr[2 : 2+int(addr[1])])
		port = (int(addr[2+int(addr[1])]) << 8) | int(addr[2+int(addr[1])+1])
	case ProxyDestAddrTypeV4:
		nameOrIP = net.IP(addr[1 : 1+net.IPv4len]).String()
		port = (int(addr[1+net.IPv4len]) << 8) | int(addr[1+net.IPv4len+1])
	case ProxyDestAddrTypeV6:
		nameOrIP = net.IP(addr[1 : 1+net.IPv6len]).String()
		port = (int(addr[1+net.IPv6len]) << 8) | int(addr[1+net.IPv6len+1])
	}
	return
}

func IsReservedAddr(addr net.IP) bool {
	if addr == nil {
		return false
	}
	for _, reservedCIDR := range BlockedReservedCIDR {
		if reservedCIDR.Contains(addr) {
			return true
		}
	}
	return false
}

func GetDerivedKey(password string) []byte {
	var sum, remaining []byte
	md5Sum := md5.New()
	for len(sum) < LenDerivedPassword {
		md5Sum.Write(remaining)
		md5Sum.Write([]byte(password))
		sum = md5Sum.Sum(sum)
		remaining = sum[len(sum)-md5Sum.Size():]
		md5Sum.Reset()
	}
	return sum[:LenDerivedPassword]
}

func AEADBlockCipher(preSharedKey, salt []byte) (cipher.AEAD, error) {
	derivedKey := make([]byte, LenDerivedPassword)
	keyDerivation := hkdf.New(sha1.New, preSharedKey, salt, []byte(MagicKeyDerivationInfo))
	if _, err := io.ReadFull(keyDerivation, derivedKey); err != nil {
		return nil, err
	}
	blockCipher, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blockCipher)
}

func RandNum(absMin, variableLower, randMore int) int {
	lower := 0
	if variableLower != 0 {
		lower = RandSeed % variableLower
	}
	return absMin + lower + rand.Intn(randMore)
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
	// Knock on each of the TCP and UDP ports and anticipate random response due to incorrect shared key magic
	for _, port := range sockd.TCPPorts {
		fmt.Println("knocking on port", port)
		if conn, err := net.Dial("tcp", sockd.Address+":"+strconv.Itoa(port)); err != nil {
			t.Fatal(err)
		} else if n, err := conn.Write(bytes.Repeat([]byte{0}, 1000)); err != nil && n != 10 {
			t.Fatal(err, n)
		} else if resp, _ := ioutil.ReadAll(conn); len(resp) < 10 {
			// The returned error can be nil (EOF) or non-nil (connection
			// reset). Forget about it.
			t.Fatal(err, resp)
		}
	}
	for _, port := range sockd.UDPPorts {
		fmt.Println("knocking on port", port)
		resp := make([]byte, 100)
		if conn, err := net.Dial("udp", sockd.Address+":"+strconv.Itoa(port)); err != nil {
			t.Fatal(err)
		} else if n, err := conn.Write(bytes.Repeat([]byte{0}, 1000)); err != nil && n != 10 {
			t.Fatal(err, n)
		} else if n, err := conn.Read(resp); err != nil || n < 4 {
			t.Fatal(err, n)
		}
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

// Daemon is intentionally undocumented magic ^____^
type Daemon struct {
	Address    string `json:"Address"`
	Password   string `json:"Password"`
	PerIPLimit int    `json:"PerIPLimit"`
	TCPPorts   []int  `json:"TCPPorts"`
	UDPPorts   []int  `json:"UDPPorts"`

	// DNSDaemon is an initialised DNS daemon. It must not be nil.
	DNSDaemon *dnsd.Daemon `json:"-"`

	tcpDaemons []*TCPDaemon
	udpDaemons []*UDPDaemon

	logger lalog.Logger
}

func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 96
	}
	daemon.logger = lalog.Logger{
		ComponentName: "sockd",
		ComponentID:   []lalog.LoggerIDField{{Key: "Addr", Value: daemon.Address}},
	}
	if daemon.DNSDaemon == nil {
		return errors.New("sockd.Initialise: dns daemon must be assigned")
	}
	if daemon.TCPPorts == nil || len(daemon.TCPPorts) == 0 || daemon.TCPPorts[0] < 1 {
		return errors.New("sockd.Initialise: there has to be at least one TCP listen port")
	}
	if len(daemon.Password) < 7 {
		return errors.New("sockd.Initialise: password must be at least 7 characters long")
	}
	daemon.tcpDaemons = make([]*TCPDaemon, 0)
	daemon.udpDaemons = make([]*UDPDaemon, 0)
	return nil
}

func (daemon *Daemon) StartAndBlock() error {
	defer daemon.Stop()
	wg := new(sync.WaitGroup)

	if daemon.TCPPorts != nil {
		for _, tcpPort := range daemon.TCPPorts {
			tcpDaemon := &TCPDaemon{
				Address:    daemon.Address,
				Password:   daemon.Password,
				PerIPLimit: daemon.PerIPLimit,
				TCPPort:    tcpPort,
				DNSDaemon:  daemon.DNSDaemon,
			}
			if err := tcpDaemon.Initialise(); err != nil {
				daemon.Stop()
				return err
			}
			wg.Add(1)
			daemon.tcpDaemons = append(daemon.tcpDaemons, tcpDaemon)
			go func(tcpDaemon *TCPDaemon) {
				defer wg.Done()
				if tcpErr := tcpDaemon.StartAndBlock(); tcpErr != nil {
					daemon.logger.Warning(fmt.Sprintf("TCP-%d", tcpDaemon.TCPPort), tcpErr, "failed to start TCP daemon")
				}
			}(tcpDaemon)
		}
	}
	if daemon.UDPPorts != nil {
		for _, udpPort := range daemon.UDPPorts {
			udpDaemon := &UDPDaemon{
				Address:    daemon.Address,
				Password:   daemon.Password,
				PerIPLimit: daemon.PerIPLimit,
				UDPPort:    udpPort,
				DNSDaemon:  daemon.DNSDaemon,
			}
			if err := udpDaemon.Initialise(); err != nil {
				daemon.Stop()
				return err
			}
			wg.Add(1)
			daemon.udpDaemons = append(daemon.udpDaemons, udpDaemon)
			go func(udpDaemon *UDPDaemon) {
				defer wg.Done()
				if udpErr := udpDaemon.StartAndBlock(); udpErr != nil {
					daemon.logger.Warning(fmt.Sprintf("UDP-%d", udpDaemon.UDPPort), udpErr, "failed to start UDP daemon")
				}
			}(udpDaemon)
		}
	}
	wg.Wait()
	return nil
}

// Stop terminates all TCP and UDP servers.
func (daemon *Daemon) Stop() {
	if daemon.tcpDaemons != nil {
		for _, tcpDaemon := range daemon.tcpDaemons {
			if tcpDaemon != nil {
				tcpDaemon.Stop()
			}
		}
	}
	if daemon.udpDaemons != nil {
		for _, udpDaemon := range daemon.udpDaemons {
			if udpDaemon != nil {
				udpDaemon.Stop()
			}
		}
	}
	daemon.tcpDaemons = make([]*TCPDaemon, 0)
	daemon.udpDaemons = make([]*UDPDaemon, 0)
}
