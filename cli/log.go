package cli

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/awsinteg"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

var (
	loggerSQSClientInitOnce = new(sync.Once)
)

// LogWarningCallbackQueueMessageBody contains details of a warning log entry, ready to be serialised into JSON for sending as an SQS message.
type LogWarningCallbackQueueMessageBody struct {
	UnixNanoSec   int64  `json:"unix_nano_sec"`
	UnixSec       int64  `json:"unix_sec"`
	ComponentName string `json:"component_name"`
	ComponentID   string `json:"component_id"`
	FunctionName  string `json:"function_name"`
	ActorName     string `json:"actor_name"`
	Error         error  `json:"error"`
	Message       string `json:"message"`
}

// GetJSON returns the message body serialised into JSON.
func (messageBody LogWarningCallbackQueueMessageBody) GetJSON() []byte {
	serialised, err := json.Marshal(messageBody)
	if err != nil {
		return []byte{}
	}
	return serialised
}

/*
InstallOptionalLoggerSQSCallback installs a global callback function for all laitos loggers to forward a copy of each warning
log entry to AWS SQS.
This behaviour is enabled optionally by specifying the queue URL in environment variable LAITOS_SEND_WARNING_LOG_TO_SQS_URL.
*/
func InstallOptionalLoggerSQSCallback(logger lalog.Logger, sqsURL string) {
	if misc.EnableAWSIntegration && sqsURL != "" {
		logger.Info("InstallOptionalLoggerSQSCallback", "", nil, "installing callback for sending logger warning messages to SQS")
		loggerSQSClientInitOnce.Do(func() {
			sqsClient, err := awsinteg.NewSQSClient()
			if err != nil {
				lalog.DefaultLogger.Warning("InstallLoggerSQSCallback", "", err, "failed to initialise SQS client")
				return
			}
			// Give SQS a copy of each warning message
			lalog.GlobalLogWarningCallback = func(componentName, componentID, funcName, actorName string, err error, msg string) {
				// By contract, the function body must avoid generating a warning log message to avoid infinite recurison.
				sendTimeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				logMessageRecord := LogWarningCallbackQueueMessageBody{
					UnixNanoSec:   time.Now().UnixNano(),
					UnixSec:       time.Now().Unix(),
					ComponentName: componentName,
					ComponentID:   componentID,
					FunctionName:  funcName,
					ActorName:     actorName,
					Error:         err,
					Message:       msg,
				}
				_ = sqsClient.SendMessage(sendTimeoutCtx, sqsURL, string(logMessageRecord.GetJSON()))
			}
		})
	}
}
