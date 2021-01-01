package toolbox

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSubjectReportRequest_Lint(t *testing.T) {
	req := SubjectReportRequest{
		SubjectIP:       strings.Repeat("I", 1000),
		SubjectHostName: strings.Repeat("H", 1000),
		SubjectPlatform: strings.Repeat("P", 1000),
		SubjectComment:  strings.Repeat("C", 10000),
		CommandRequest:  AppCommandRequest{Command: strings.Repeat("R", MaxCmdLength+100)},
		CommandResponse: AppCommandResponse{Command: strings.Repeat("W", MaxCmdLength+100)},
	}
	req.Lint()
	if req.SubjectIP != strings.Repeat("I", 64) {
		t.Fatal(req.SubjectIP)
	}
	if req.SubjectHostName != strings.Repeat("H", 256) {
		t.Fatal(req.SubjectHostName)
	}
	if req.SubjectPlatform != strings.Repeat("P", 128) {
		t.Fatal(req.SubjectPlatform)
	}
	if req.SubjectComment != strings.Repeat("C", 4*1024) {
		t.Fatal(req.SubjectComment)
	}
	if req.CommandRequest.Command != strings.Repeat("R", MaxCmdLength) {
		t.Fatal(req.CommandRequest.Command)
	}
	if req.CommandResponse.Command != strings.Repeat("W", MaxCmdLength) {
		t.Fatal(req.CommandResponse.Command)
	}
}

func TestSubjectReportRequest_SerialiseCompact(t *testing.T) {
	req := SubjectReportRequest{
		SubjectIP:       "123.132.123.123",
		SubjectHostName: "hzgl-dev-abc.example.com",
		SubjectPlatform: "windows/amd64",
		SubjectComment:  "hello there\nsecond line",
		CommandRequest: AppCommandRequest{
			Command: "123456098765.s start-computer",
		},
		CommandResponse: AppCommandResponse{
			Command:        "123456098765.s stop-computer",
			ReceivedAt:     time.Unix(1234567890, 0),
			Result:         "stopped the computer all right\nsecond line",
			RunDurationSec: 182,
		},
	}
	serialised := req.SerialiseCompact()
	t.Log(len(serialised))
	t.Log(serialised)
	// Deserialise the complete string
	var deserialised SubjectReportRequest
	if err := deserialised.DeserialiseFromCompact(serialised); err != nil {
		t.Log(err)
	}
	if !reflect.DeepEqual(deserialised, req) {
		t.Fatalf("\n%+v\n%+v\n", deserialised, req)
	}
	// Deserialise truncated string
	var deserialised2 SubjectReportRequest
	if err := deserialised2.DeserialiseFromCompact(serialised[:30]); err != ErrSubjectReportTruncated {
		t.Fatal(err)
	}
	if deserialised2.SubjectHostName != "hzgl-dev-abc.example.com" || deserialised2.CommandRequest.Command != "12345" {
		t.Fatalf("%+v", deserialised2)
	}

}
