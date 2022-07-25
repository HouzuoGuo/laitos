package tcpoverdns

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/lzw"
	"compress/zlib"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
)

func TestSegment_Packet(t *testing.T) {
	want := Segment{
		ID:     12345,
		Flags:  FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum: 23456,
		AckNum: 34567,
		Data:   []byte{1, 2, 3, 4},
	}

	packet := want.Packet()
	got := SegmentFromPacket(packet)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recovered: %+#v original: %+#v", got, want)
	}
}

func TestSegmentFromPacket(t *testing.T) {
	want := Segment{Flags: FlagMalformed}
	segWithData := Segment{Data: []byte{1, 2}}
	segWithMalformedLen := segWithData.Packet()
	for _, seg := range [][]byte{nil, {1}, segWithMalformedLen[:SegmentHeaderLen+1]} {
		if got := SegmentFromPacket(seg); !reflect.DeepEqual(got, want) {
			t.Fatalf("got: %+#v, want: %+#v", got, want)
		}
	}
}

func TestFlags(t *testing.T) {
	allFlags := FlagHandshakeSyn | FlagHandshakeAck | FlagAckOnly | FlagKeepAlive | FlagReset | FlagMalformed
	for _, flag := range []Flag{FlagHandshakeSyn, FlagHandshakeAck, FlagAckOnly, FlagKeepAlive, FlagReset, FlagMalformed} {
		if !allFlags.Has(flag) {
			t.Fatalf("missing %d", flag)
		}
	}
	if allFlags.Has(1 << 6) {
		t.Fatalf("should not have had flag %d", 1<<4)
	}
}

func TestSegment_Equals(t *testing.T) {
	original := Segment{
		ID:     12345,
		Flags:  FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum: 23456,
		AckNum: 34567,
		Data:   []byte{1, 2, 3, 4},
	}
	if !original.Equals(original) {
		t.Errorf("should have been equal")
	}

	tests := []struct {
		a, b Segment
	}{
		{a: Segment{ID: 1}, b: Segment{ID: 2}},
		{a: Segment{Flags: 1}, b: Segment{Flags: 2}},
		{a: Segment{SeqNum: 1}, b: Segment{SeqNum: 2}},
		{a: Segment{AckNum: 1}, b: Segment{AckNum: 2}},
		{a: Segment{Data: []byte{0}}, b: Segment{Data: []byte{1}}},
	}
	for _, test := range tests {
		if test.a.Equals(test.b) {
			t.Errorf("should not have been equal: %+v, %+v", test.a, test.b)
		}
	}
}

func TestCompression(t *testing.T) {
	// original := `The words of the Teacher, son of David, king in Jerusalem: “Meaningless! Meaningless!” says the Teacher. “Utterly meaningless! Everything is meaningless.” What do people gain from all their labors at which they toil under the sun?`
	original := `<!doctype html><html itemscope="" itemtype="http://schema.org/WebPage" lang="en-IE"><head><meta charset="UTF-8"><meta content="dark" name="color-scheme"><meta content="origin" name="referrer"><meta content="/images/branding/googleg/1x/googleg_standard_colo`

	t.Run("zlib", func(t *testing.T) {
		var b bytes.Buffer
		w, err := zlib.NewWriterLevel(&b, zlib.BestCompression)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(original))
		w.Close()
		fmt.Println("zlib", len(b.Bytes()), len(base64.StdEncoding.EncodeToString(b.Bytes())))
	})

	t.Run("lzw", func(t *testing.T) {
		var b bytes.Buffer
		w := lzw.NewWriter(&b, lzw.MSB, 8)
		w.Write([]byte(original))
		w.Close()
		fmt.Println("lzw", len(b.Bytes()), len(base64.StdEncoding.EncodeToString(b.Bytes())))
	})

	t.Run("flate", func(t *testing.T) {
		var b bytes.Buffer
		w, err := flate.NewWriter(&b, flate.BestCompression)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(original))
		w.Close()
		fmt.Println("flate", len(b.Bytes()), len(base64.StdEncoding.EncodeToString(b.Bytes())))
	})

	t.Run("gzip", func(t *testing.T) {
		var b bytes.Buffer
		w, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(original))
		w.Close()
		fmt.Println("gzip", len(b.Bytes()), len(base64.StdEncoding.EncodeToString(b.Bytes())))
	})

	t.Run("base64", func(t *testing.T) {
		fmt.Println(base64.StdEncoding.DecodeString("TQ=="))
	})
}

func TestCompressDecompressBytes(t *testing.T) {
	tests := [][]byte{
		{},
		{0},
		{0, 1},
		{0, 1, 2},
		[]byte(`<!doctype html><html itemscope="" itemtype="http://schema.org/WebPage" lang="en-IE"><head><meta charset="UTF-8"><meta content="dark" name="color-scheme"><meta content="origin" name="referrer"><meta content="/images/branding/googleg/1x/googleg_standard_colo`),
	}
	for _, original := range tests {
		compressed := CompressBytes(original)
		got, err := DecompressBytes(compressed)
		if err != nil || !reflect.DeepEqual(got, original) {
			t.Fatalf("DecompressBytes(%+v): got %+v, want %+v", compressed, got, original)
		}
	}
}

func TestSegment_DNSQuestion(t *testing.T) {
	randData := make([]byte, 100)
	if _, err := rand.Read(randData); err != nil {
		t.Fatal(err)
	}
	want := Segment{
		ID:     12345,
		Flags:  FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum: 23456,
		AckNum: 34567,
		Data:   randData,
	}
	dnsQuestion := want.DNSQuestion("prefix-label", "example.com")
	fmt.Println(dnsQuestion.Name.String())
	got := SegmentFromDNSQuestion(2, dnsQuestion)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recovered: %+#v original: %+#v", got, want)
	}
}

func TestSegment_DNSResource(t *testing.T) {
	randData := make([]byte, 100)
	if _, err := rand.Read(randData); err != nil {
		t.Fatal(err)
	}
	want := Segment{
		ID:     12345,
		Flags:  FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum: 23456,
		AckNum: 34567,
		Data:   randData,
	}
	ret := want.DNSResource()
	fmt.Println(ret)
	got := SegmentFromDNSResources(ret)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recovered: %+#v original: %+#v", got, want)
	}
}
