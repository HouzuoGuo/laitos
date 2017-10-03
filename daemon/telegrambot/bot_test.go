package telegrambot

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"strings"
	"testing"
)

func TestTelegramBot_StartAndBock(t *testing.T) {
	// Must not start if command processor is insane
	bot := TelegramBot{
		AuthorizationToken: "dummy",
		Processor:          &common.CommandProcessor{},
	}
	if err := bot.Initialise(); !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal(err)
	}

	cmdproc := common.GetTestCommandProcessor()
	bot = TelegramBot{
		AuthorizationToken: "",
		Processor:          cmdproc,
	}
	if err := bot.Initialise(); !strings.Contains(err.Error(), "Token") {
		t.Fatal(err)
	}
	bot.AuthorizationToken = "dummy"
	if err := bot.Initialise(); !strings.Contains(err.Error(), "RateLimit") {
		t.Fatal(err)
	}

	bot.RateLimit = 10
	if err := bot.Initialise(); err != nil {
		t.Fatal(err)
	}

	TestTelegramBot(&bot, t)
}
