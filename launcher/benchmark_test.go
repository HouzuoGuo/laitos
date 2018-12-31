package launcher

import (
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd"
	"github.com/HouzuoGuo/laitos/misc"
)

func TestBenchmark(t *testing.T) {
	if misc.HostIsWindows() {
		// FIXME: fix this test case for Windows
		t.Skip("FIXME: enable this test case for Windows")
	}
	var config Config
	if err := config.DeserialiseFromJSON([]byte(sampleConfigJSON)); err != nil {
		t.Fatal(err)
	}

	httpd.PrepareForTestHTTPD(t)

	// Alter daemon configuration to use rather arbitrary ports
	config.DNSDaemon.TCPPort = 63122
	config.DNSDaemon.UDPPort = 34211
	config.HTTPDaemon.Port = 13871
	config.PlainSocketDaemon.TCPPort = 47811
	config.PlainSocketDaemon.UDPPort = 58511
	config.MailDaemon.Port = 31891
	config.SockDaemon.TCPPorts = []int{54872}
	config.SockDaemon.UDPPorts = []int{12989}
	config.SimpleIPSvcDaemon.ActiveUsersPort = 8545
	config.SimpleIPSvcDaemon.DayTimePort = 34434
	config.SimpleIPSvcDaemon.QOTDPort = 24390
	config.SNMPDaemon.Port = 24822

	// Re-initialise and then Start all daemons
	if err := config.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := config.GetDNSD().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		if err := config.GetHTTPD().StartAndBlockNoTLS(19381); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		if err := config.GetPlainSocketDaemon().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		if err := config.GetMailDaemon().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		if err := config.GetSockDaemon().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		if err := config.GetSimpleIPSvcD().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		if err := config.GetSNMPD().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()

	// Wait 5 seconds for daemons to settle
	time.Sleep(5 * time.Second)

	// Run benchmark for short 3 seconds, otherwise there are too many log entries.
	bench := Benchmark{
		Config:      &config,
		DaemonNames: []string{DNSDName, InsecureHTTPDName, PlainSocketName, SimpleIPSvcName, SMTPDName, SNMPDName, SOCKDName},
		HTTPPort:    53829,
	}
	// Conduct benchmark for 10 seconds
	bench.RunBenchmarkAndProfiler()
	time.Sleep(10 * time.Second)
	bench.Stop = true
}
