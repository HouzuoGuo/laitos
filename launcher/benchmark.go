package launcher

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof" // pprof package has an init routine that installs profiler API handlers
	"net/smtp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
)

type Benchmark struct {
	Config      *Config       // Config is an initialised configuration structure that provides for all daemons involved in benchmark.
	DaemonNames []string      // DaemonNames is a list of daemons that have already started and waiting to run benchmark.
	Logger      *lalog.Logger // Logger is specified by caller if the caller wishes.
	HTTPPort    int           // HTTPPort is to be served by net/http/pprof on an HTTP server running on localhost.
	Stop        bool          // Stop, if true, will soon terminate ongoing benchmark. It may be reset to false in preparation for a new benchmark run.
}

/*
RunBenchmarkAndProfiler starts benchmark immediately and continuously reports progress via logger. The function kicks off
more goroutines for benchmarking individual daemons, and therefore does not block caller.

Benchmark cases usually uses randomly generated data and do not expect a normal response. Therefore, they serve well as
fuzzy tests too.

The function assumes that daemons are already started and ready to receive requests, therefore caller may wish to
consider waiting a short while for daemons to settle before running this benchmark routine.
*/
func (bench *Benchmark) RunBenchmarkAndProfiler() {
	// Expose profiler APIs via HTTP server running on a fixed port number on localhost
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("localhost:%d", bench.HTTPPort), nil); err != http.ErrServerClosed {
			bench.Logger.Abort(nil, err, "failed to start profiler HTTP server")
		}
	}()
	for _, daemonName := range bench.DaemonNames {
		// Kick off benchmarks
		switch daemonName {
		case DNSDName:
			go bench.BenchmarkDNSDaemon()
		case HTTPDName:
			go bench.BenchmarkHTTPSDaemon()
		case InsecureHTTPDName:
			go bench.BenchmarkHTTPDaemon()
		case MaintenanceName:
			// There is no benchmark for maintenance daemon
		case PlainSocketName:
			go bench.BenchmarkPlainSocketDaemon()
		case SMTPDName:
			go bench.BenchmarkSMTPDaemon()
		case SOCKDName:
			go bench.BenchmarkSockDaemon()
		case SimpleIPSvcName:
			go bench.BenchmarkSimpleIPSvcDaemon()
		case SNMPDName:
			go bench.BenchmarkSNMPDaemon()
		case TelegramName:
			// There is no benchmark for telegram daemon
		}
	}
}

/*
ReportRatePerSecond runs the input function (which most likely runs indefinitely) and logs rate of invocation of a
trigger function (fed to the input function) every second. The function blocks caller as long as input function
continues to run.
*/
func (bench *Benchmark) reportRatePerSecond(loop func(func()), name string, logger *lalog.Logger) {
	unitTimeSec := 1
	ticker := time.NewTicker(time.Duration(unitTimeSec) * time.Second)

	var counter, total int64
	go func() {
		for {
			if bench.Stop {
				return
			}
			<-ticker.C
			counter := atomic.LoadInt64(&counter)
			logger.Info(name, nil, "%d/s (total %d)", atomic.SwapInt64(&counter, 0)/int64(unitTimeSec), counter)
		}
	}()
	loop(func() {
		atomic.AddInt64(&counter, 1)
		atomic.AddInt64(&total, 1)
	})
}

// BenchmarkDNSDaemon continuously sends DNS queries via both TCP and UDP in a sequential manner.
func (bench *Benchmark) BenchmarkDNSDaemon() {
	var doUDP bool

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(bench.Config.GetDNSD().UDPPort))
	if err != nil {
		bench.Logger.Panic(nil, err, "failed to init UDP address")
		return
	}
	tcpPort := bench.Config.GetDNSD().TCPPort

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic(nil, err, "failed to acquire random bytes")
				return
			}

			if doUDP {
				doUDP = false
				clientConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			} else {
				doUDP = true
				clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(tcpPort))
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			}
		}
	}, "BenchmarkDNSDaemon", bench.Logger)
}

// BenchmarkHTTPDaemonn continuously sends HTTP requests in a sequential manner.
func (bench *Benchmark) BenchmarkHTTPDaemon() {
	allRoutes := make([]string, 0, 32)
	for installedRoute := range bench.Config.GetHTTPD().ResourcePaths {
		allRoutes = append(allRoutes, installedRoute)
	}
	if len(allRoutes) == 0 {
		bench.Logger.Abort(nil, nil, "HTTP daemon does not any route at all, cannot benchmark it.")
	}
	urlTemplate := fmt.Sprintf("http://localhost:%d%%s", bench.Config.GetHTTPD().PlainPort)

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()
			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic(nil, err, "failed to acquire random bytes")
				return
			}
			_, _ = inet.DoHTTP(context.Background(), inet.HTTPRequest{TimeoutSec: 3, Body: bytes.NewReader(buf)}, fmt.Sprintf(urlTemplate, allRoutes[rand.Intn(len(allRoutes))]))
		}
	}, "BenchmarkHTTPDaemon", bench.Logger)
}

// BenchmarkHTTPDaemonn continuously sends HTTPS requests in a sequential manner.
func (bench *Benchmark) BenchmarkHTTPSDaemon() {
	allRoutes := make([]string, 0, 32)
	for installedRoute := range bench.Config.GetHTTPD().ResourcePaths {
		allRoutes = append(allRoutes, installedRoute)
	}
	if len(allRoutes) == 0 {
		bench.Logger.Abort(nil, nil, "HTTP daemon does not any route at all, cannot benchmark it.")
	}
	urlTemplate := fmt.Sprintf("https://localhost:%d%%s", bench.Config.GetHTTPD().PlainPort)

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()
			_, _ = inet.DoHTTP(context.Background(), inet.HTTPRequest{TimeoutSec: 3}, fmt.Sprintf(urlTemplate, allRoutes[rand.Intn(len(allRoutes))]))
		}
	}, "BenchmarkHTTPSDaemon", bench.Logger)

}

// BenchmarkPlainSocketDaemon continuously sends toolbox commands via both TCP and UDP in a sequential manner.
func (bench *Benchmark) BenchmarkPlainSocketDaemon() {
	var doUDP bool

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(bench.Config.GetPlainSocketDaemon().UDPPort))
	if err != nil {
		bench.Logger.Panic(nil, err, "failed to init UDP address")
		return
	}
	tcpPort := bench.Config.GetPlainSocketDaemon().TCPPort

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic(nil, err, "failed to acquire random bytes")
				return
			}

			if doUDP {
				doUDP = false
				clientConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			} else {
				doUDP = true
				clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(tcpPort))
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			}
		}
	}, "BenchmarkPlainSocketDaemon", bench.Logger)
}

// BenchmarkSimpleIPSvcDaemon continuously sends requests to all simple IP services via both TCP and UDP.
func (bench *Benchmark) BenchmarkSimpleIPSvcDaemon() {
	allPorts := []int{bench.Config.GetSimpleIPSvcD().ActiveUsersPort, bench.Config.GetSimpleIPSvcD().DayTimePort, bench.Config.GetSimpleIPSvcD().QOTDPort}
	counter := int64(0)

	bench.reportRatePerSecond(func(trigger func()) {
		for port := allPorts[0]; ; port = allPorts[int(atomic.AddInt64(&counter, 1))%len(allPorts)] {
			udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(port))
			if err != nil {
				bench.Logger.Panic(nil, err, "failed to init UDP address")
				return
			}

			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic(nil, err, "failed to acquire random bytes")
				return
			}

			if rand.Intn(2) == 0 {
				// UDP request
				clientConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			} else {
				// TCP request
				clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			}
		}
	}, "BenchmarkSimpleIPSvcDaemon", bench.Logger)
}

// BenchmarkSMTPDaemon continuously sends emails in a sequential manner.
func (bench *Benchmark) BenchmarkSMTPDaemon() {
	port := bench.Config.GetMailDaemon().Port
	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic(nil, err, "failed to acquire random bytes")
				return
			}

			_ = smtp.SendMail(fmt.Sprintf("localhost:%d", port), nil, "ClientFrom@localhost", []string{"ClientTo@does-not-exist.com"}, buf)
		}
	}, "BenchmarkSMTPDaemon", bench.Logger)

}

// BenchmarkSockDaemon continuously sends packets via both TCP and UDP in a sequential manner.
func (bench *Benchmark) BenchmarkSockDaemon() {
	var doUDP bool

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(bench.Config.GetSockDaemon().UDPPorts[0]))
	if err != nil {
		bench.Logger.Panic(nil, err, "failed to init UDP address")
		return
	}
	tcpPort := bench.Config.GetSockDaemon().TCPPorts[0]
	rand.Seed(time.Now().UnixNano())

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic(nil, err, "failed to acquire random bytes")
				return
			}

			if doUDP {
				doUDP = false
				clientConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			} else {
				doUDP = true
				clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(tcpPort))
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			}
		}
	}, "BenchmarkSockDaemon", bench.Logger)
}

// BenchmarkSNMPDaemon sends random data to SNMP port, aims to catch hidden mistakes in SNMP packet decoder.
func (bench *Benchmark) BenchmarkSNMPDaemon() {
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(bench.Config.GetSNMPD().Port))
	if err != nil {
		bench.Logger.Panic(nil, err, "failed to init UDP address")
		return
	}

	rand.Seed(time.Now().UnixNano())

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic(nil, err, "failed to acquire random bytes")
				return
			}

			clientConn, err := net.DialUDP("udp", nil, udpAddr)
			if err != nil {
				continue
			}
			if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				clientConn.Close()
				continue
			}
			if _, err := clientConn.Write(buf); err != nil {
				clientConn.Close()
				continue
			}
			clientConn.Close()
		}
	}, "BenchmarkSNMPkDaemon", bench.Logger)
}
