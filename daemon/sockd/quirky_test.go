package sockd

import (
	"bytes"
	"fmt"
	"math/bits"
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
			t.Error(err)
			return
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
	if n := WriteRandomToTCP(client); n < 200 {
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

func TestRandomText(t *testing.T) {
	txt := RandomText(3)
	if len(txt) != 3 {
		t.Fatalf("unexpected length: %q", txt)
	}
	txt = RandomText(500)
	if len(txt) != 500 {
		t.Fatalf("unexpected length: %q", txt)
	}
	for i := 0; i < 500; i++ {
		for _, length := range []int{128, 128 * 4, 128 * 16, 128 * 128} {
			txt := RandomText(length)
			if len(txt) != length {
				t.Fatalf("unexpected length: %q", txt)
			}
			var popCount int
			for _, c := range txt {
				popCount += bits.OnesCount(uint(c))
			}
			popRate := float32(popCount) / float32(len(txt)*8)
			if popRate < 0.6 {
				t.Fatalf("unexpected pop rate: %v - %q", popRate, txt)
			}
			t.Log("pop rate for length", length, "is", popRate)
		}
	}
	for _, r := range txt {
		if !(r >= 65 && r <= 90 || r >= 97 && r <= 122 || r == ' ' || r == '.' || r == '/') {
			t.Fatalf("unexpected character: %q", txt)
		}
	}
}
