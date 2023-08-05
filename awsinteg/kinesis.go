package awsinteg

import (
	"context"
	"fmt"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/aws/aws-xray-sdk-go/xray"
)

func NewKinesisHoseClient() (*KinesisHoseClient, error) {
	logger := &lalog.Logger{ComponentName: "kinesis"}
	regionName := inet.GetAWSRegion()
	if regionName == "" {
		return nil, fmt.Errorf("NewKinesisHoseClient: unable to determine AWS region, is it set in environment variable AWS_REGION?")
	}
	logger.Info(nil, nil, "initialising using AWS region name \"%s\"", regionName)
	apiSession, err := session.NewSession(&aws.Config{Region: aws.String(regionName)})
	if err != nil {
		return nil, err
	}
	firehoseInst := firehose.New(apiSession)
	xray.AWS(firehoseInst.Client)
	return &KinesisHoseClient{
		apiSession: apiSession,
		logger:     logger,
		client:     firehoseInst,
	}, nil
}

type KinesisHoseClient struct {
	logger     *lalog.Logger
	apiSession *session.Session
	client     *firehose.Firehose
}

func (hoseClient *KinesisHoseClient) PutRecord(ctx context.Context, streamName string, recordData []byte) error {
	startTimeNano := time.Now().UnixNano()
	_, err := hoseClient.client.PutRecordWithContext(ctx, &firehose.PutRecordInput{
		DeliveryStreamName: aws.String(streamName),
		Record:             &firehose.Record{Data: recordData},
	})
	durationMilli := (time.Now().UnixNano() - startTimeNano) / 1000000
	hoseClient.logger.Info(streamName, err, "PutRecordWithContext completed in %d milliseconds for a %d bytes long record", durationMilli, len(recordData))
	return err
}
