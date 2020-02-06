package telegrambot

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestTelegramBot_StartAndBock(t *testing.T) {
	bot := Daemon{}
	if err := bot.Initialise(); err == nil || !strings.Contains(err.Error(), "filters must be configured") {
		t.Fatal(err)
	}
	// Must not start if command processor is insane
	bot = Daemon{
		AuthorizationToken: "dummy",
		Processor:          toolbox.GetInsaneCommandProcessor(),
	}
	if err := bot.Initialise(); !strings.Contains(err.Error(), toolbox.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	// Give it a good command processor and check other initialisation errors
	cmdproc := toolbox.GetTestCommandProcessor()
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
