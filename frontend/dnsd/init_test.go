package dnsd

import (
	"encoding/hex"
	"os"
	"testing"
)

var githubComUDPQuery []byte
var githubComTCPQuery []byte

func TestMain(m *testing.M) {
	var err error
	githubComUDPQuery, err = hex.DecodeString("e575012000010000000000010667697468756203636f6d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
	githubComTCPQuery, err = hex.DecodeString("00274cc7012000010000000000010667697468756203636f6d00000100010000291000000000000000")
	if err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
