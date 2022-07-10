package tcpoverdns

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
)

func echoTCPServer(t *testing.T, port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("echo tcp server failed to listen: %v", err)
		return
	}
	lalog.DefaultLogger.Info("echoTCPServer", "", nil, "listening")
	go func() {
		conn, err := listener.Accept()
		lalog.DefaultLogger.Info("echoTCPServer", "", nil, "connected")
		if err != nil {
			lalog.DefaultLogger.Panic("echoTCPServer", "", err, "echo tcp server failed to accept")
			return
		}
		defer conn.Close()
		defer listener.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				lalog.DefaultLogger.Info("echoTCPServer", "", err, "read EOF")
				return
			}
			lalog.DefaultLogger.Info("echoTCPServer", "", err, "received: %+v", line)
			if err != nil {
				lalog.DefaultLogger.Panic("echoTCPServer", "", err, "echo tcp server read failure")
				return
			}
			_, err = writer.WriteString(line)
			if err == io.EOF {
				lalog.DefaultLogger.Panic("echoTCPServer", "", err, "write EOF")
				return
			}
			if err != nil {
				lalog.DefaultLogger.Panic("echoTCPServer", "", err, "echo tcp server write failure")
				return
			}
			if err := writer.Flush(); err != nil {
				lalog.DefaultLogger.Panic("echoTCPServer", "", err, "echo tcp server flush failure")
				return
			}
			if line == "end\n" {
				lalog.DefaultLogger.Info("echoTCPServer", "", err, "returning now")
				return
			}
		}
	}()
}

func readSegmentHeaderData(t *testing.T, ctx context.Context, in io.Reader) Segment {
	segHeader := make([]byte, SegmentHeaderLen)
	n, err := in.Read(segHeader)
	if err != nil || n != SegmentHeaderLen {
		t.Fatalf("failed to read segment header: %v %v", n, err)
		return Segment{}
	}

	segDataLen := int(binary.BigEndian.Uint16(segHeader[SegmentHeaderLen-2 : SegmentHeaderLen]))
	segData := make([]byte, segDataLen)
	n, err = in.Read(segData)
	if err != nil || n != segDataLen {
		t.Fatalf("failed to read segment data: %v %v", segDataLen, err)
		return Segment{}
	}

	return SegmentFromPacket(append(segHeader, segData...))
}

func TestProxy(t *testing.T) {
	echoTCPServer(t, 63238)

	proxy := &Proxy{
		Debug: true,
		// Keep the segment length short for the test,
		MaxSegmentLenExclHeader: 2,
	}
	proxy.Start(context.Background())

	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                   1111,
		Debug:                true,
		InputTransport:       inTransport,
		OutputTransport:      outTransport,
		InitiatorSegmentData: []byte(`{"p": 63238, "a": "127.0.0.1"}`),
		// Keep the segment length short for the test.
		MaxSegmentLenExclHeader: 2,
		Initiator:               true,
	}
	tc.Start(context.Background())
	go func() {
		for {
			// Pipe segments from TC to proxy.
			seg := readSegmentHeaderData(t, context.Background(), testOut)
			lalog.DefaultLogger.Info("", "", nil, "testOut tc to proxy: %+v", seg)
			resp, hasResp := proxy.Receive(seg)
			lalog.DefaultLogger.Info("", "", nil, "proxy resp: %+v %+v", resp, hasResp)
			if hasResp {
				// Send the response segment back to TC.
				_, err := testIn.Write(resp.Packet())
				if err != nil {
					panic("failed to write to testIn")
				}
			}
		}
	}()
	// Have a conversation with the echo server.
	req := []string{
		"aaaa\n",
		"bbb\n",
		"cc\n",
		"d\n",
	}
	// req len = 5 + 4 + 3 + 2 = 14
	reader := bufio.NewReader(tc)
	for _, line := range req {
		lalog.DefaultLogger.Info("", "", nil, "test is writing line: %v", line)
		n, err := tc.Write([]byte(line))
		if err != nil || n != len(line) {
			t.Fatalf("failed to write request line - n: %v, err: %v", n, err)
		}
		readBack, err := reader.ReadString('\n')
		if err != nil || readBack != line {
			t.Fatalf("failed to read back line - n: %s, err: %v", readBack, err)
		}
	}
	// Tell proxy to end the TC
	if _, err := tc.Write([]byte("end\n")); err != nil {
		t.Fatalf("failed to write request line: %v", err)
	}
	// The underlying TCP connection is closed after "end\n".
	checkTC(t, tc, 5, StateClosed, 14, 14, 14+4, nil, nil)
	checkTCError(t, tc, 5, 0, 0, 0)
}

func TestProxyCloudflareConnection(t *testing.T) {
	t.Skip("FIXME TODO")
	proxy := &Proxy{Debug: true}
	proxy.Start(context.Background())

	testIn, inTransport := net.Pipe()
	testOut, outTransport := net.Pipe()
	tc := &TransmissionControl{
		ID:                   1111,
		Debug:                true,
		InputTransport:       inTransport,
		OutputTransport:      outTransport,
		InitiatorSegmentData: []byte(`{"p": 80, "a": "1.1.1.1"}`),
		Initiator:            true,
	}
	tc.Start(context.Background())

	go func() {
		for {
			// Pipe segments from TC to proxy.
			seg := readSegmentHeaderData(t, context.Background(), testOut)
			lalog.DefaultLogger.Info("", "", nil, "testOut tc to proxy: %+v", seg)
			resp, hasResp := proxy.Receive(seg)
			lalog.DefaultLogger.Info("", "", nil, "proxy resp: %+v %+v", resp, hasResp)
			if hasResp {
				// Send the response segment back to TC.
				_, err := testIn.Write(resp.Packet())
				if err != nil {
					panic("failed to write to testIn")
				}
			}
		}
	}()

	req := []string{
		"GET / HTTP/1.1",
		"Host: 1.1.1.1",
		"User-Agent: HouzuoGuo-laitos",
		"Accept: */*",
		"Connection: close",
	}
	for _, line := range req {
		_, err := tc.Write([]byte(line + "\r\n"))
		if err != nil {
			t.Fatalf("write failure: %+v", err)
		}
	}
	resp, err := io.ReadAll(tc)
	if err != nil && err != io.EOF {
		t.Fatalf("read failure: %v", err)
	}
	respStr := string(resp)
	t.Logf("http response: %s", respStr)
	if !strings.Contains(respStr, `</html>`) {
		t.Fatalf("missing content")
	}
	checkTC(t, tc, 5, StateClosed, 14, 14, 14+4, nil, nil)
	checkTCError(t, tc, 5, 0, 0, 0)
}
