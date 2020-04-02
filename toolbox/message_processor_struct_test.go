package toolbox

import (
	"reflect"
	"testing"
	"time"
)

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
