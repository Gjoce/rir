package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type Config struct {
	Endpoint         string
	Region           string
	Bucket           string
	AccessKey        string
	SecretKey        string
	SignaturesBucket string
}

type Service struct {
	client           *s3.Client
	presigner        *s3.PresignClient
	bucket           string
	signaturesBucket string
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
		client:           client,
		presigner:        s3.NewPresignClient(client),
		bucket:           cfg.Bucket,
		signaturesBucket: cfg.SignaturesBucket,
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

// ErrSignatureExists means a signature record for the consultation is already
// stored. Records are written once and never replaced.
var ErrSignatureExists = errors.New("storage: signature already exists")

// ErrSignatureNotFound means no signature record is stored for the consultation.
var ErrSignatureNotFound = errors.New("storage: signature not found")

func signatureKey(consultationID string) string {
	return fmt.Sprintf("signatures/%s.json", consultationID)
}

// UploadSignature writes a JSON signature record to the signatures bucket under
// signatures/{consultationID}.json. The write is conditional: an existing
// record is never overwritten, it returns ErrSignatureExists. In production the
// bucket should also have S3 Object Lock (WORM), so nobody can delete or replace it.
func (s *Service) UploadSignature(ctx context.Context, consultationID string, record []byte) (string, error) {
	if s.signaturesBucket == "" {
		return "", fmt.Errorf("signatures bucket not configured")
	}

	key := signatureKey(consultationID)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.signaturesBucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(record),
		ContentLength: aws.Int64(int64(len(record))),
		ContentType:   aws.String("application/json"),
		IfNoneMatch:   aws.String("*"),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "PreconditionFailed" {
			return key, ErrSignatureExists
		}

		return "", fmt.Errorf("failed to upload signature for consultation %s: %w", consultationID, err)
	}

	return key, nil
}

// maxSignatureBytes caps how much of a signature record is read back. Records are
// a few kilobytes plus the opinion.
const maxSignatureBytes = 1 << 20

// GetSignature reads the signature record stored for the consultation.
func (s *Service) GetSignature(ctx context.Context, consultationID string) ([]byte, error) {
	if s.signaturesBucket == "" {
		return nil, fmt.Errorf("signatures bucket not configured")
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.signaturesBucket),
		Key:    aws.String(signatureKey(consultationID)),
	})
	if err != nil {
		var noKey *s3types.NoSuchKey
		if errors.As(err, &noKey) {
			return nil, ErrSignatureNotFound
		}

		return nil, fmt.Errorf("failed to read signature for consultation %s: %w", consultationID, err)
	}
	defer func() { _ = out.Body.Close() }()

	record, err := io.ReadAll(io.LimitReader(out.Body, maxSignatureBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read signature for consultation %s: %w", consultationID, err)
	}

	return record, nil
}
