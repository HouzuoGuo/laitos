package awsinteg

import (
	"context"
	"fmt"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-xray-sdk-go/xray"
)

func NewSNSClient() (*SNSClient, error) {
	logger := lalog.Logger{ComponentName: "sns"}
	regionName := inet.GetAWSRegion()
	if regionName == "" {
		return nil, fmt.Errorf("NewSNSClient: unable to determine AWS region, is it set in environment variable AWS_REGION?")
	}
	logger.Info("", nil, "initialising using AWS region name \"%s\"", regionName)
	apiSession, err := session.NewSession(&aws.Config{Region: aws.String(regionName)})
	if err != nil {
		return nil, err
	}
	snsInst := sns.New(apiSession)
	xray.AWS(snsInst.Client)
	return &SNSClient{
		apiSession: apiSession,
		client:     snsInst,
		logger:     logger,
	}, nil
}

type SNSClient struct {
	logger     lalog.Logger
	apiSession *session.Session
	client     *sns.SNS
}

func (snsClient *SNSClient) Publish(ctx context.Context, topicARN, text string) error {
	startTimeNano := time.Now().UnixNano()
	snsClient.logger.Info(topicARN, nil, "publishing a %d bytes long message", len(text))
	_, err := snsClient.client.PublishWithContext(ctx, &sns.PublishInput{Message: aws.String(text), TopicArn: aws.String(topicARN)})
	durationMilli := (time.Now().UnixNano() - startTimeNano) / 1000000
	snsClient.logger.Info(topicARN, nil, "PublishWithContext completed in %d milliseconds for a %d bytes long message (err? %v)",
		durationMilli, len(text), err)
	return err
}
