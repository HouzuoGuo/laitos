package tcpoverdns

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/lzw"
	"compress/zlib"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestSegment_Packet(t *testing.T) {
	want := Segment{
		ID:       12345,
		Flags:    FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum:   23456,
		AckNum:   34567,
		Reserved: 45678,
		Data:     []byte{1, 2, 3, 4},
	}

	packet := want.Packet()
	got := SegmentFromPacket(packet)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recovered: %+#v original: %+#v", got, want)
	}

	want.Flags = FlagHandshakeSyn
	packet = want.Packet()
	got = SegmentFromPacket(packet)
	if !reflect.DeepEqual(got, Segment{Flags: FlagMalformed, Data: []byte("missing initiator config")}) {
		t.Fatal("did not identify malformed segment without initiator config")
	}
}

func TestSegmentFromMalformedPacket(t *testing.T) {
	segWithData := Segment{Data: []byte{1, 2}}
	segWithMalformedLen := segWithData.Packet()
	for _, seg := range [][]byte{nil, {1}, segWithMalformedLen[:SegmentHeaderLen+1]} {
		if got := SegmentFromPacket(seg); got.Flags != FlagMalformed {
			t.Fatalf("%+v", got)
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
		ID:       12345,
		Flags:    FlagHandshakeAck & FlagHandshakeSyn,
		SeqNum:   23456,
		AckNum:   34567,
		Reserved: 45678,
		Data:     []byte{1, 2, 3, 4},
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

func TestBase62(t *testing.T) {
	randBuf := []byte{0, 0, 1, 2, 3, 4, 5, 0, 1, 2, 3, 4, 5}
	encoded := ToBase62Mod(randBuf)
	t.Log("encoded", encoded)
	got, err := ParseBase62Mod(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, randBuf) {
		t.Fatalf("got: %v, want: %v", got, randBuf)
	}
}

func TestCompression(t *testing.T) {
	base32EncodingNoPadding := base32.StdEncoding.WithPadding(base32.NoPadding)
	randBuf := make([]byte, 234)
	if _, err := rand.Read(randBuf); err != nil {
		t.Fatal(err)
	}
	var input = []struct {
		name    string
		content []byte
	}{
		{
			name:    "natural language (short)",
			content: []byte(`NIV The words of the Teacher, son of David, king in Jerusalem: “Meaningless! Meaningless!” says the Teacher. “Utterly meaningless! Everything is meaningless.” What do people gain from all their labors at which they toil under the sun?`),
		},
		{
			name:    "natural language (long)",
			content: []byte(`NIV The words of the Teacher, son of David, king in Jerusalem: “Meaningless! Meaningless!” says the Teacher. “Utterly meaningless! Everything is meaningless.” What do people gain from all their labors at which they toil under the sun? Generations come and generations go, but the earth remains forever. The sun rises and the sun sets, and hurries back to where it rises. The wind blows to the south and turns to the north; round and round it goes, ever returning on its course.`),
		},
		{
			name:    "html",
			content: []byte(`<!doctype html><html itemscope="" itemtype="http://schema.org/SearchResultsPage" lang="en-IE"><head><meta charset="UTF-8"><meta content="dark" name="color-scheme"><meta content="origin" name="referrer"><meta content="/images/branding/`),
		},
		{
			name:    "random",
			content: randBuf,
		},
	}

	t.Run("no-compression", func(t *testing.T) {
		for _, example := range input {
			var b bytes.Buffer
			b.Write(example.content)
			encodedLen := len(base32EncodingNoPadding.EncodeToString(b.Bytes()))
			fmt.Printf("no-compression - %v, original: %d, encoded: %d, ratio: %.3f\n", example.name, len(b.Bytes()), encodedLen, float64(encodedLen)/float64(len(example.content)))
		}
	})

	t.Run("zlib", func(t *testing.T) {
		for _, example := range input {
			var b bytes.Buffer
			w, err := zlib.NewWriterLevel(&b, zlib.BestCompression)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(example.content)
			w.Close()
			encodedLen := len(base32EncodingNoPadding.EncodeToString(b.Bytes()))
			fmt.Printf("zlib - %v, compressed: %d, encoded: %d, ratio: %.3f\n", example.name, len(b.Bytes()), encodedLen, float64(encodedLen)/float64(len(example.content)))
		}
	})

	t.Run("lzw", func(t *testing.T) {
		for _, example := range input {
			var b bytes.Buffer
			w := lzw.NewWriter(&b, lzw.MSB, 8)
			w.Write(example.content)
			w.Close()
			encodedLen := len(base32EncodingNoPadding.EncodeToString(b.Bytes()))
			fmt.Printf("lzm - %v, compressed: %d, encoded: %d, ratio: %.3f\n", example.name, len(b.Bytes()), encodedLen, float64(encodedLen)/float64(len(example.content)))
		}
	})

	t.Run("flate+base32", func(t *testing.T) {
		for _, example := range input {
			var b bytes.Buffer
			w, err := flate.NewWriter(&b, flate.BestCompression)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(example.content)
			w.Close()
			encodedLen := len(base32EncodingNoPadding.EncodeToString(b.Bytes()))
			fmt.Printf("flate - %v, compressed: %d, encoded: %d, ratio: %.3f\n", example.name, len(b.Bytes()), encodedLen, float64(encodedLen)/float64(len(example.content)))
		}
	})

	t.Run("flate+base62", func(t *testing.T) {
		for _, example := range input {
			var b bytes.Buffer
			w, err := flate.NewWriter(&b, flate.BestCompression)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(example.content)
			w.Close()
			encodedLen := len(ToBase62Mod(b.Bytes()))
			fmt.Printf("flate - %v, compressed: %d, encoded: %d, ratio: %.3f\n", example.name, len(b.Bytes()), encodedLen, float64(encodedLen)/float64(len(example.content)))
		}
	})

	t.Run("gzip", func(t *testing.T) {
		for _, example := range input {
			var b bytes.Buffer
			w, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(example.content)
			w.Close()
			encodedLen := len(base32EncodingNoPadding.EncodeToString(b.Bytes()))
			fmt.Printf("gzip - %v, compressed: %d, encoded: %d, ratio: %.3f\n", example.name, len(b.Bytes()), encodedLen, float64(encodedLen)/float64(len(example.content)))
		}
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

func TestInitiatorConfig(t *testing.T) {
	wantTiming := TimingConfig{
		SlidingWindowWaitDuration: 1000 * time.Millisecond,
		RetransmissionInterval:    1234 * time.Millisecond,
		AckDelay:                  3456 * time.Millisecond,
		KeepAliveInterval:         4567 * time.Millisecond,
		ReadTimeout:               5678 * time.Millisecond,
		WriteTimeout:              7890 * time.Millisecond,
	}
	want := &InitiatorConfig{
		SetConfig:               true,
		MaxSegmentLenExclHeader: 123,
		Debug:                   true,
		Timing:                  wantTiming,
	}
	serialised := want.Bytes()
	got := DeserialiseInitiatorConfig(serialised)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got: %+#v want: %+#v", got, want)
	}

	wantTC := &TransmissionControl{
		MaxSegmentLenExclHeader: 123,
		MaxSlidingWindow:        123 * 64,
		InitialTiming:           wantTiming,
		LiveTiming:              wantTiming,
		Debug:                   true,
	}
	gotTC := &TransmissionControl{}
	got.Config(gotTC)
	if !reflect.DeepEqual(gotTC, wantTC) {
		t.Fatalf("got: %+#v want: %+#v", gotTC, wantTC)
	}
}

func TestSegment_DNSNameQuery(t *testing.T) {
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
	query := want.DNSNameQuery("prefix-label", "example.com")
	fmt.Println(query)
	got := SegmentFromDNSName(2, query)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("recovered: %+#v original: %+#v", got, want)
	}
}
