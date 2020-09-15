package awsinteg

import (
	"context"
	"io"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func NewS3Client() (*S3Client, error) {
	apiSession, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	return &S3Client{
		apiSession: apiSession,
		uploader:   s3manager.NewUploader(apiSession),
		logger:     lalog.Logger{ComponentName: "s3"},
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
