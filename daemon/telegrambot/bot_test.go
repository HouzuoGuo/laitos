package telegrambot

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/daemon/common"
)

func TestTelegramBot_StartAndBock(t *testing.T) {
	bot := Daemon{}
	if err := bot.Initialise(); err == nil || strings.Index(err.Error(), "filters must be configured") == -1 {
		t.Fatal(err)
	}
	// Must not start if command processor is insane
	bot = Daemon{
		AuthorizationToken: "dummy",
		Processor:          common.GetInsaneCommandProcessor(),
	}
	if err := bot.Initialise(); !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	// Give it a good command processor and check other initialisation errors
	cmdproc := common.GetTestCommandProcessor()
	bot = Daemon{
		AuthorizationToken: "",
		Processor:          cmdproc,
	}
	if err := bot.Initialise(); !strings.Contains(err.Error(), "Token") {
		t.Fatal(err)
	}
	bot.AuthorizationToken = "dummy"
	if err := bot.Initialise(); err != nil || bot.PerUserLimit != 2 {
		t.Fatal(err)
	}

	TestTelegramBot(&bot, t)
}
