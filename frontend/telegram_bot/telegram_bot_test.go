package telegram

import (
	"github.com/HouzuoGuo/websh/frontend/common"
	"strings"
	"testing"
)

func TestTelegramBot_StartAndBock(t *testing.T) {
	// Must not start if command processor is insane
	bot := TelegramBot{
		AuthorizationToken: "not right",
		Processor:          &common.CommandProcessor{},
	}
	if err := bot.StartAndBlock(); !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}

	// Well then it is really difficult to test the chat routine
	// So I am going to only do the API test call
	cmdproc := common.GetTestCommandProcessor()
	bot = TelegramBot{
		AuthorizationToken: "not right",
		Processor:          cmdproc,
	}
	if err := bot.StartAndBlock(); err == nil || strings.Index(err.Error(), "HTTP") == -1 {
		t.Fatal(err)
	}
}
