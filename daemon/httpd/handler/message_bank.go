package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const HandleMessageBankPage = `<html>
<head>
	<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
	<title>laitos message bank</title>
</head>
<body>
    <p>Message bank "default", incoming direction:</p>
    <pre>%s</pre>
    <hr/>
    <p>Message bank "default", outgoing direction:</p>
    <pre>%s</pre>
    <form action="%s" method="post">
        <p><input type="text" name="messageForDefault" /><input type="submit" value="Submit outgoing message"/></p>
    </form>
    <hr/>
    <p>Message bank "TTN", incoming direction:</p>
    <pre>%s</pre>
    <hr/>
    <p>Message bank "TTN", outgoing direction:</p>
    <pre>%s</pre>
    <form action="%s" method="post">
        <p><input type="text" name="messageForTTN" /><input type="submit" value="Submit outgoing message"/></p>
    </form>
    <hr/>
</body>
</html>
`

type HandleMessageBank struct {
	cmdProc                    *toolbox.CommandProcessor
	stripURLPrefixFromResponse string
	logger                     lalog.Logger
}

func (bank *HandleMessageBank) Initialise(logger lalog.Logger, cmdProc *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	bank.cmdProc = cmdProc
	bank.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	bank.logger = logger
	return nil
}

func (bank *HandleMessageBank) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	NoCache(w)
	handlerURL := strings.TrimPrefix(r.RequestURI, bank.stripURLPrefixFromResponse)
	if r.Method == http.MethodPost {
		if messageForDefault := r.FormValue("messageForDefault"); messageForDefault != "" {
			if err := bank.cmdProc.Features.MessageBank.Store(toolbox.MessageBankTagDefault, toolbox.MessageDirectionOutgoing, time.Now(), messageForDefault); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		} else if messageForTTN := r.FormValue("messageForTTN"); messageForTTN != "" {
			if err := bank.cmdProc.Features.MessageBank.Store(toolbox.MessageBankTagTTN, toolbox.MessageDirectionOutgoing, time.Now(), messageForTTN); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
	// Render the page.
	_, _ = w.Write([]byte(fmt.Sprintf(
		HandleMessageBankPage,
		toolbox.MessagesToString(bank.cmdProc.Features.MessageBank.Get(toolbox.MessageBankTagDefault, toolbox.MessageDirectionIncoming)),
		toolbox.MessagesToString(bank.cmdProc.Features.MessageBank.Get(toolbox.MessageBankTagDefault, toolbox.MessageDirectionOutgoing)),
		handlerURL,
		toolbox.MessagesToString(bank.cmdProc.Features.MessageBank.Get(toolbox.MessageBankTagTTN, toolbox.MessageDirectionIncoming)),
		toolbox.MessagesToString(bank.cmdProc.Features.MessageBank.Get(toolbox.MessageBankTagTTN, toolbox.MessageDirectionOutgoing)),
		handlerURL)))
}

func (*HandleMessageBank) GetRateLimitFactor() int {
	return 1
}

func (*HandleMessageBank) SelfTest() error {
	return nil
}
