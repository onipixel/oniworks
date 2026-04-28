package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// S3Config holds credentials and settings for an S3-compatible object store.
type S3Config struct {
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	// Endpoint is the base URL for S3-compatible services (MinIO, Wasabi, R2).
	// Leave empty for AWS S3.
	Endpoint   string
	// BaseURL is the public CDN/bucket URL used by URL(). If empty, the S3 URL is used.
	BaseURL    string
	// DefaultACL is the default ACL for new objects (default: "private").
	DefaultACL string
	// ForcePathStyle enables path-style addressing (required by MinIO).
	ForcePathStyle bool
}

// S3 is an S3-compatible storage driver.
type S3 struct {
	cfg    S3Config
	client *s3.Client
	pre    *s3.PresignClient
}

// NewS3 creates an S3 driver.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	if cfg.DefaultACL == "" {
		cfg.DefaultACL = "private"
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("storage/s3: load config: %w", err)
	}

	s3Opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = cfg.ForcePathStyle
		},
	}
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	return &S3{cfg: cfg, client: client, pre: s3.NewPresignClient(client)}, nil
}

func (s *S3) Put(ctx context.Context, path string, r io.Reader, opts ...PutOptions) error {
	ct := "application/octet-stream"
	acl := s.cfg.DefaultACL
	var meta map[string]string

	if len(opts) > 0 {
		if opts[0].ContentType != "" {
			ct = opts[0].ContentType
		}
		if opts[0].ACL != "" {
			acl = opts[0].ACL
		}
		meta = opts[0].Metadata
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.Bucket),
		Key:         aws.String(path),
		Body:        r,
		ContentType: aws.String(ct),
		Metadata:    meta,
	}
	if acl != "" && acl != "private" {
		input.ACL = types.ObjectCannedACL(acl)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("storage/s3: put %q: %w", path, err)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("storage/s3: get %q: %w", path, err)
	}
	return out.Body, nil
}

func (s *S3) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(path),
	})
	return err
}

func (s *S3) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		// S3 returns a NoSuchKey-shaped error on 404
		return false, nil
	}
	return true, nil
}

func (s *S3) URL(path string) string {
	if s.cfg.BaseURL != "" {
		return strings.TrimRight(s.cfg.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.cfg.Bucket, s.cfg.Region, path)
}

func (s *S3) SignedURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	out, err := s.pre.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(path),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return "", fmt.Errorf("storage/s3: sign url %q: %w", path, err)
	}
	return out.URL, nil
}

func (s *S3) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.Bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("storage/s3: list %q: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}
	return keys, nil
}

func (s *S3) Size(ctx context.Context, path string) (int64, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return 0, err
	}
	if out.ContentLength != nil {
		return *out.ContentLength, nil
	}
	return 0, nil
}

func (s *S3) Copy(ctx context.Context, src, dst string) error {
	source := fmt.Sprintf("%s/%s", s.cfg.Bucket, src)
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.cfg.Bucket),
		CopySource: aws.String(source),
		Key:        aws.String(dst),
	})
	return err
}

func (s *S3) Move(ctx context.Context, src, dst string) error {
	if err := s.Copy(ctx, src, dst); err != nil {
		return err
	}
	return s.Delete(ctx, src)
}

// ensure v4 is referenced (it's transitively required by aws-sdk-go-v2)
var _ = v4.NewSigner
