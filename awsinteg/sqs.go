package awsinteg

import (
	"context"
	"fmt"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-xray-sdk-go/xray"
)

func NewSQSClient() (*SQSClient, error) {
	logger := lalog.Logger{ComponentName: "sqs"}
	regionName := inet.GetAWSRegion()
	if regionName == "" {
		return nil, fmt.Errorf("NewSQSClient: unable to determine AWS region, is it set in environment variable AWS_REGION?")
	}
	logger.Info("NewSQSClient", "", nil, "initialising using AWS region name \"%s\"", regionName)
	apiSession, err := session.NewSession(&aws.Config{Region: aws.String(regionName)})
	if err != nil {
		return nil, err
	}
	sqsInst := sqs.New(apiSession)
	xray.AWS(sqsInst.Client)
	return &SQSClient{
		apiSession: apiSession,
		client:     sqsInst,
		logger:     logger,
	}, nil
}

type SQSClient struct {
	logger     lalog.Logger
	apiSession *session.Session
	client     *sqs.SQS
}

func (sqsClient *SQSClient) SendMessage(ctx context.Context, queueURL, text string) error {
	startTimeNano := time.Now().UnixNano()
	/*
		This function may end up called by logger's warning callback. For now, avoid generating warning messages from
		here, and avoid placing "err" into logger.Info's error parameter input.
	*/
	sqsClient.logger.Info("SendMessage", queueURL, nil, "sending a %d bytes long message", len(text))
	_, err := sqsClient.client.SendMessageWithContext(ctx, &sqs.SendMessageInput{
		// The new message is made immediately visible to consumers for processing
		DelaySeconds: aws.Int64(0),
		MessageBody:  aws.String(text),
		QueueUrl:     aws.String(queueURL),
	})
	durationMilli := (time.Now().UnixNano() - startTimeNano) / 1000000
	sqsClient.logger.Info(
		"SendMessage", queueURL, nil, "SendMessageWithContext completed in %d milliseconds for a %d bytes long message (err? %v)",
		durationMilli, len(text), err)
	return err
}
