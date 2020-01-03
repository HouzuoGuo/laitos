package sockd

import (
	"bytes"
	"fmt"
	"net"
	"testing"
)

func TestReadWriteAndWriteRand(t *testing.T) {
	// The server keeps data received from its one and only client in a buffer
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receiverDone := make(chan bool, 1)
	receivedData := make([]byte, 0)
	go func() {
		client, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		buf := make([]byte, 1048576)
		for n, err := int(0), error(nil); err == nil; n, err = ReadWithRetry(client, buf) {
			receivedData = append(receivedData, buf[:n]...)
		}
		_ = client.Close()
		receiverDone <- true
	}()

	// Write 2MB of regular data
	client, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := WriteWithRetry(client, bytes.Repeat([]byte{1}, 2*1048576)); err != nil || n != 2*1048576 {
		t.Fatal(err, n)
	}
	// Write random data
	if n := WriteRand(client); n < 200 {
		t.Fatal(n)
	}
	_ = client.Close()

	// Verify that server got the correct data
	<-receiverDone
	if len(receivedData) < 1048576*2+200 {
		t.Fatal(len(receivedData))
	}
	for i := 0; i < 2*1048576; i++ {
		if receivedData[i] != 1 {
			t.Fatal(i, receivedData[i])
		}
	}
}
