package tcpoverdns

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
)

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
	// Use the TC like a regular HTTP client.
	req := []string{
		"GET / HTTP 1.1\r\n",
		"Host: 1.1.1.1\r\n",
		"User-Agent: golang test\r\n",
		"Accept: */*\r\n",
		"\r\n",
	}
	for _, line := range req {
		n, err := tc.Write([]byte(line))
		if err != nil || n != len(line) {
			t.Fatalf("failed to write request line - n: %v, err: %v", n, err)
		}
	}
	// TODO FIXME: figure out why this doesn't work.
	resp, err := io.ReadAll(tc)
	if err != nil {
		t.Fatalf("failed to read from tc: %v", err)
	}
	t.Log("resposne: ", resp)
}
