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

	// Start benchmark daemons
	go func() {
		if err := config.GetDNSD().StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()

	go func() {
		if err := config.GetHTTPD().StartAndBlockNoTLS(0); err != nil {
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

	// Wait 5 seconds for daemons to settle
	time.Sleep(5 * time.Second)

	// Run benchmark for short 3 seconds, otherwise there are too many log entries.
	bench := Benchmark{
		Config:      &config,
		DaemonNames: []string{DNSDName, InsecureHTTPDName, PlainSocketName, SMTPDName, SOCKDName},
		HTTPPort:    59678,
	}
	bench.RunBenchmarkAndProfiler()
	time.Sleep(3 * time.Second)
	bench.Stop = true
}
