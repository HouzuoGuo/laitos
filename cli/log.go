package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/awsinteg"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/aws/aws-xray-sdk-go/awsplugins/beanstalk"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ec2"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ecs"
	"github.com/aws/aws-xray-sdk-go/strategy/ctxmissing"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/aws/aws-xray-sdk-go/xraylog"
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
		logger.Info(nil, nil, "installing callback for sending logger warning messages to SQS")
		loggerSQSClientInitOnce.Do(func() {
			sqsClient, err := awsinteg.NewSQSClient()
			if err != nil {
				lalog.DefaultLogger.Warning(nil, err, "failed to initialise SQS client")
				return
			}
			// Give SQS a copy of each warning message
			lalog.GlobalLogWarningCallback = func(componentName, componentID, funcName string, actorName interface{}, err error, msg string) {
				// By contract, the function body must avoid generating a warning log message to avoid infinite recurison.
				sendTimeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				logMessageRecord := LogWarningCallbackQueueMessageBody{
					UnixNanoSec:   time.Now().UnixNano(),
					UnixSec:       time.Now().Unix(),
					ComponentName: componentName,
					ComponentID:   componentID,
					FunctionName:  funcName,
					ActorName:     fmt.Sprint(actorName),
					Error:         err,
					Message:       msg,
				}
				_ = sqsClient.SendMessage(sendTimeoutCtx, sqsURL, string(logMessageRecord.GetJSON()))
			}
		})
	}
}

func InitialiseAWS() {
	if inet.IsAWS() {
		// Integrate the decorated handler with AWS x-ray. The crucial x-ray daemon program seems to be only capable of running on AWS compute resources.
		_ = os.Setenv("AWS_XRAY_CONTEXT_MISSING", "LOG_ERROR")
		_ = xray.Configure(xray.Config{ContextMissingStrategy: ctxmissing.NewDefaultIgnoreErrorStrategy()})
		xray.SetLogger(xraylog.NewDefaultLogger(ioutil.Discard, xraylog.LogLevelWarn))
		go func() {
			// These functions of aws lib take their sweet time, don't let them block main's progress. It's OK to miss a couple of traces.
			beanstalk.Init()
			ecs.Init()
			ec2.Init()
		}()
	}
}

// ClearDedupBuffersInBackground periodically clears the global LRU buffers used
// for de-duplicating log messages.
func ClearDedupBuffersInBackground() {
	go func() {
		tickerChan := time.Tick(5 * time.Second)
		for {
			numDropped := lalog.NumDropped.Load()
			<-tickerChan
			newDropped := lalog.NumDropped.Load()
			if diff := newDropped - numDropped; diff > 0 {
				lalog.DefaultLogger.Warning(nil, nil, "dropped %d log messages", diff)
			}
			lalog.ClearDedupBuffers()
		}
	}()
}
