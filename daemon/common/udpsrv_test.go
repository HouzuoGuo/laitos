package common

import (
	"fmt"
	"log"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

type UDPTestApp struct {
	stats *misc.Stats
}

func (app *UDPTestApp) GetUDPStatsCollector() *misc.Stats {
	return app.stats
}

func (app *UDPTestApp) HandleUDPClient(logger *lalog.Logger, clientIP string, client *net.UDPAddr, packet []byte, srv *net.UDPConn) {
	if clientIP == "" {
		panic("client IP must not be empty")
	}
	if !reflect.DeepEqual(packet, []byte{0}) {
		log.Panicf("unexpected incoming packet %v", packet)
	}
	if n, err := srv.WriteToUDP([]byte("hello"), client); err != nil || n != 5 {
		log.Panicf("n %d err %v", n, err)
	}
}

func TestUDPServer(t *testing.T) {
	srv := UDPServer{
		ListenAddr:  "127.0.0.1",
		ListenPort:  12382,
		AppName:     "TestUDPServer",
		App:         &UDPTestApp{stats: misc.NewStats()},
		LimitPerSec: 5,
	}
	srv.Initialise()

	// Expect server to start within three seconds
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := srv.StartAndBlock(); err != nil {
			t.Error(err)
			return
		}
		serverStopped <- struct{}{}
	}()
	time.Sleep(3 * time.Second)
	if !srv.IsRunning() {
		t.Fatal("not running")
	}

	// Connect to the server and expect a hello response
	client, err := net.Dial("udp", fmt.Sprintf("%s:%d", srv.ListenAddr, srv.ListenPort))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := client.Write([]byte{0}); err != nil || n != 1 {
		t.Fatal(err, n)
	}
	buf := make([]byte, 5)
	if n, err := client.Read(buf); err != nil || n != 5 {
		t.Fatal(n, err)
	}
	if string(buf) != "hello" {
		t.Fatal(buf)
	}

	// Wait for connection to close and then check stats counter
	time.Sleep(ServerRateLimitIntervalSec * 2)
	if count := srv.App.GetUDPStatsCollector().Count(); count != 1 {
		t.Fatal(count)
	}

	// Attempt to exceed the rate limit via connection attempts
	var success int
	for i := 0; i < 10; i++ {
		client, err := net.Dial("udp", fmt.Sprintf("%s:%d", srv.ListenAddr, srv.ListenPort))
		if err != nil {
			t.Fatal(err)
		}
		if n, err := client.Write([]byte{0}); err != nil || n != 1 {
			t.Fatal(err, n)
		}
		buf := make([]byte, 5)
		_ = client.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		_, _ = client.Read(buf)
		if string(buf) == "hello" {
			success++
		}
		time.Sleep(50 * time.Millisecond)
	}
	if success > srv.LimitPerSec*2 || success < srv.LimitPerSec/2 {
		t.Fatal(success)
	}

	// Attempt to exceed the rate limit via conversation
	time.Sleep(ServerRateLimitIntervalSec * 2)
	success = 0
	for i := 0; i < 10; i++ {
		if srv.AddAndCheckRateLimit("test") {
			success++
		}
	}
	if success > srv.LimitPerSec*2 || success < srv.LimitPerSec/2 {
		t.Fatal(success)
	}

	// Server must shut down within three seconds
	srv.Stop()
	<-serverStopped
	if srv.IsRunning() {
		t.Fatal("must not be running anymore")
	}

	// It is OK to repeatedly shut down a server
	srv.Stop()
	srv.Stop()
	if srv.IsRunning() {
		t.Fatal("must not be running anymore")
	}
}
