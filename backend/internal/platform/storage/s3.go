package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store is the Cloudflare R2 backend of ObjectStorage. The S3 endpoint is
// the account R2 edge, so the client is built with a custom BaseEndpoint +
// static keypair credentials — never the default AWS provider chain.
type S3Store struct {
	api       S3API
	presigner S3Presigner
	bucket    string
}

// NewS3Store builds the R2 client from cfg (endpoint override supported).
// The region is a constant: R2 S3-compatibility is single-region, so the
// "auto" region signs correctly against a custom endpoint.
func NewS3Store(cfg R2Config) (*S3Store, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.EndpointURL()),
		Region:       "auto",
		Credentials:  credentials.StaticCredentialsProvider{Value: aws.Credentials{AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey}},
		UsePathStyle: true,
	})
	return &S3Store{api: client, presigner: s3.NewPresignClient(client), bucket: cfg.Bucket}, nil
}

// PutObject uploads the body.
func (s *S3Store) PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := s.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	return err
}

// HeadObject returns the stored object's size + content type, or ErrNotFound.
func (s *S3Store) HeadObject(ctx context.Context, key string) (*ObjectInfo, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	out, err := s.api.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var nf *types.NotFound
		var nsk *types.NoSuchKey
		if errors.As(err, &nf) || errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	info := &ObjectInfo{Key: key, ContentType: aws.ToString(out.ContentType)}
	if out.ContentLength != nil {
		info.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		info.LastModified = *out.LastModified
	}
	return info, nil
}

// GetObject downloads the object body.
func (s *S3Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	out, err := s.api.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var nf *types.NotFound
		var nsk *types.NoSuchKey
		if errors.As(err, &nf) || errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// DeleteObject removes the object; a missing object is a no-op.
func (s *S3Store) DeleteObject(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := s.api.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}

// PresignPut mints the browser upload URL. The PUT carries the MIME type so
// the object arrives with its content type for the confirm check.
func (s *S3Store) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	out, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

// PresignGet mints the download URL.
func (s *S3Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	out, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)},
		func(o *s3.PresignOptions) {
			o.Expires = ttl
		})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

// validateKey rejects empty, absolute and traversal keys: an object key is
// a flat name (owner-id/uuid-ext), and a leading "/" or ".." segment would
// escape the intended layout.
func validateKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: empty or absolute key", ErrObjectKey)
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			return fmt.Errorf("%w: traversal in key", ErrObjectKey)
		}
	}
	return nil
}
