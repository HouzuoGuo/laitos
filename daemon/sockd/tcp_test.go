package sockd

import (
	"bytes"
	"fmt"
	"net"
	"testing"
)

func TestPipeTCPConnection(t *testing.T) {
	// The first server transfers 1MB of data to the connected client
	listener1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		srv1, err := listener1.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		if n, err := srv1.Write(bytes.Repeat([]byte{1}, 1048576)); err != nil || n != 1048576 {
			t.Errorf("err - %v, n - %d", err, n)
			return
		}
		_ = srv1.Close()
	}()

	// The second server reads 1MB of data from the connected client
	listener2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receiverDone := make(chan bool, 1)
	receivedData := make([]byte, 0)
	go func() {
		srv2, err := listener2.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		buf := make([]byte, 2*1048576)
		for n, err := int(0), error(nil); err == nil; n, err = srv2.Read(buf) {
			receivedData = append(receivedData, buf[:n]...)
		}
		_ = srv2.Close()
		receiverDone <- true
	}()

	// Connect to the first server and pipe the data received from it to the second server
	client1, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", listener1.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatal(err)
	}
	client2, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", listener2.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatal(err)
	}
	PipeTCPConnection(client1, client2, true)
	<-receiverDone

	// Should have received the correct data in full
	if len(receivedData) != 1048576 {
		t.Fatal(len(receivedData))
	}
	for i, b := range receivedData {
		if b != 1 {
			t.Fatal(i, b)
		}
	}
}
