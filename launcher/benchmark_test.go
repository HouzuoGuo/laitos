package launcher

import (
	"testing"
	"time"
)

func TestBenchmark(t *testing.T) {
	var config Config
	if err := config.DeserialiseFromJSON([]byte(sampleConfigJSON)); err != nil {
		t.Fatal(err)
	}

	// Start benchmark daemons on rather arbitrary ports
	go func() {
		config.GetDNSD().TCPPort = 63122
		config.GetDNSD().UDPPort = 34211
		if err := config.GetDNSD().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()

	go func() {
		config.GetHTTPD().Port = 13871
		if err := config.GetHTTPD().StartAndBlockNoTLS(19381); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		config.GetPlainSocketDaemon().TCPPort = 47811
		config.GetPlainSocketDaemon().UDPPort = 58511
		if err := config.GetPlainSocketDaemon().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		config.GetMailDaemon().Port = 31891
		if err := config.GetMailDaemon().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		config.GetSockDaemon().TCPPort = 54872
		config.GetSockDaemon().UDPPort = 12989
		if err := config.GetSockDaemon().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()

	// Wait 5 seconds for daemons to settle
	time.Sleep(5 * time.Second)

	// Run benchmark for short 3 seconds, otherwise there are too many log entries.
	bench := Benchmark{
		Config:      &config,
		DaemonNames: []string{DNSDName, InsecureHTTPDName, PlainSocketName, SMTPDName, SOCKDName},
		HTTPPort:    53829,
	}
	bench.RunBenchmarkAndProfiler()
	time.Sleep(3 * time.Second)
	bench.Stop = true
}
