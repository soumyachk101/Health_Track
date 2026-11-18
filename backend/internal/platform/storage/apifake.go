package storage

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3API is the object-API surface S3Store uses, extracted so tests inject a
// fake instead of a network client.
type S3API interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	HeadObject(ctx context.Context, in *s3.HeadObjectInput, opts ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Presigner is the presign surface S3Store uses.
type S3Presigner interface {
	PresignPutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignGetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// NewS3StoreFromAPI builds a store over injected S3 seams — the "mocked S3
// API" test path.
func NewS3StoreFromAPI(api S3API, presign S3Presigner, bucket string) *S3Store {
	return &S3Store{api: api, presigner: presign, bucket: bucket}
}

// NewS3StoreFromFuncs is a thin alias over the concrete S3 client +
// presigner (production wiring convenience).
func NewS3StoreFromFuncs(api *s3.Client, presign *s3.PresignClient, bucket string) *S3Store {
	return NewS3StoreFromAPI(api, presign, bucket)
}

// FakeS3API is an in-memory S3API + S3Presigner for tests.
type FakeS3API struct {
	objects    map[string]fakeObject
	HeadErr    error
	PutErr     error
	Deleted    []string
	PresignGet func(key string, ttl int64) string
	PresignPut func(key, contentType string, ttl int64) string
}

// NewFakeS3API returns an empty fake.
func NewFakeS3API() *FakeS3API {
	return &FakeS3API{objects: map[string]fakeObject{}}
}

func (f *FakeS3API) PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.PutErr != nil {
		return nil, f.PutErr
	}
	f.objects[aws.ToString(in.Key)] = fakeObject{contentType: aws.ToString(in.ContentType)}
	return &s3.PutObjectOutput{}, nil
}

func (f *FakeS3API) HeadObject(ctx context.Context, in *s3.HeadObjectInput, opts ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if f.HeadErr != nil {
		return nil, f.HeadErr
	}
	if _, ok := f.objects[aws.ToString(in.Key)]; !ok {
		return nil, &types.NotFound{}
	}
	o := f.objects[aws.ToString(in.Key)]
	return &s3.HeadObjectOutput{ContentType: o.contentTypePtr()}, nil
}

func (f *FakeS3API) GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := aws.ToString(in.Key)
	if _, ok := f.objects[key]; !ok {
		return nil, &types.NotFound{}
	}
	return &s3.GetObjectOutput{ContentType: f.objects[key].contentTypePtr()}, nil
}

func (f *FakeS3API) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(f.objects, aws.ToString(in.Key))
	f.Deleted = append(f.Deleted, aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func (f *FakeS3API) PresignPutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	fake := "https://r2.test/" + aws.ToString(in.Key)
	if f.PresignPut != nil {
		fake = f.PresignPut(aws.ToString(in.Key), aws.ToString(in.ContentType), 300)
	}
	return &v4.PresignedHTTPRequest{URL: fake}, nil
}

func (f *FakeS3API) PresignGetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	fake := "https://r2.test/" + aws.ToString(in.Key)
	if f.PresignGet != nil {
		fake = f.PresignGet(aws.ToString(in.Key), 120)
	}
	return &v4.PresignedHTTPRequest{URL: fake}, nil
}

// ObjectContentType returns the stored content type for a key.
func (f *FakeS3API) ObjectContentType(key string) (string, bool) {
	o, ok := f.objects[key]
	return o.contentType, ok
}

func (o fakeObject) contentTypePtr() *string {
	if o.contentType == "" {
		return nil
	}
	return &o.contentType
}
