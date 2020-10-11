package awsinteg

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-xray-sdk-go/xray"
)

func NewS3Client() (*S3Client, error) {
	logger := lalog.Logger{ComponentName: "s3"}
	regionName := inet.GetAWSRegion()
	if regionName == "" {
		return nil, fmt.Errorf("NewS3Client: unable to determine AWS region, is it set in environment variable AWS_REGION?")
	}
	logger.Info("NewS3Client", "", nil, "initialising using AWS region name \"%s\"", regionName)
	apiSession, err := session.NewSession(&aws.Config{Region: aws.String(regionName)})
	if err != nil {
		return nil, err
	}
	s3Inst := s3.New(apiSession)
	xray.AWS(s3Inst.Client)
	return &S3Client{
		apiSession: apiSession,
		uploader:   s3manager.NewUploaderWithClient(s3Inst),
		logger:     logger,
	}, nil
}

type S3Client struct {
	logger     lalog.Logger
	apiSession *session.Session
	uploader   *s3manager.Uploader
}

func (s3Client *S3Client) Upload(ctx context.Context, bucketName, objectKey string, objectValue io.Reader) error {
	startTimeNano := time.Now().UnixNano()
	s3Client.logger.Info("PutObject", bucketName, nil, "uploading object \"%s\"", objectKey)
	_, err := s3Client.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Body:   objectValue,
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	durationMilli := (time.Now().UnixNano() - startTimeNano) / 1000000
	s3Client.logger.Info("PutObject", bucketName, nil, "UploadWithContext completed in %d milliseconds for object \"%s\" (err? %v)", durationMilli, objectKey, err)
	return err
}
