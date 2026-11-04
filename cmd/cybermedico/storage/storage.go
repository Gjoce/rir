package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Config struct {
	Endpoint           string
	Region             string
	Bucket             string
	AccessKey          string
	SecretKey          string
	SignaturesBucket   string
	SignaturesKMSKeyID string // KMS key ARN/ID for SSE-KMS on the signatures bucket
}

type Service struct {
	client             *s3.Client
	presigner          *s3.PresignClient
	bucket             string
	signaturesBucket   string
	signaturesKMSKeyID string
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	var opts []func(*config.LoadOptions) error

	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	// Supply explicit static credentials if they are provided in the configuration.
	// In production AWS, if these are omitted, the SDK automatically falls back to IAM execution roles.
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	opts = append(opts, config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired))

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 config: %w", err)
	}

	// Build the client natively using standard virtual-hosted routing configuration.
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	if _, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Bucket)}); err != nil {
		return nil, fmt.Errorf("S3 bucket %q is not accessible: %w", cfg.Bucket, err)
	}

	if cfg.SignaturesBucket != "" {
		if _, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.SignaturesBucket)}); err != nil {
			return nil, fmt.Errorf("signatures S3 bucket %q is not accessible: %w", cfg.SignaturesBucket, err)
		}
	}

	return &Service{
		client:             client,
		presigner:          s3.NewPresignClient(client),
		bucket:             cfg.Bucket,
		signaturesBucket:   cfg.SignaturesBucket,
		signaturesKMSKeyID: cfg.SignaturesKMSKeyID,
	}, nil
}

func (s *Service) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object %q: %w", key, err)
	}

	return nil
}

func (s *Service) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object %q: %w", key, err)
	}

	return nil
}

func (s *Service) PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to presign URL for %q: %w", key, err)
	}

	return req.URL, nil
}

// UploadSignature writes a JSON signature record to the signatures bucket.
// The key is signatures/{consultationID}.json. The bucket must have S3 Object
// Lock (WORM) enabled — that policy is enforced at the bucket level in AWS,
// not in this call.
func (s *Service) UploadSignature(ctx context.Context, consultationID string, record []byte) error {
	if s.signaturesBucket == "" {
		return fmt.Errorf("signatures bucket not configured")
	}

	key := fmt.Sprintf("signatures/%s.json", consultationID)
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.signaturesBucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(record),
		ContentLength: aws.Int64(int64(len(record))),
		ContentType:   aws.String("application/json"),
	}
	if s.signaturesKMSKeyID != "" {
		input.ServerSideEncryption = s3types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(s.signaturesKMSKeyID)
	}

	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("failed to upload signature for consultation %s: %w", consultationID, err)
	}

	return nil
}
