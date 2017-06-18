package telegrambot

import (
	"github.com/HouzuoGuo/laitos/frontend/common"
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

	// Must not start if auth token is empty
	cmdproc := common.GetTestCommandProcessor()
	bot = TelegramBot{
		AuthorizationToken: "",
		Processor:          cmdproc,
	}
	if err := bot.Initialise(); !strings.Contains(err.Error(), "Token") {
		t.Fatal(err)
	}

	// Well then it is really difficult to test the chat routine
	// So I am going to only going to start the daemon using invalid configuration, which is definitely failing.
	bot = TelegramBot{
		AuthorizationToken: "dummy",
		Processor:          cmdproc,
	}
	if err := bot.Initialise(); err != nil {
		t.Fatal(err)
	}

	TestTelegramBot(&bot, t)
}
