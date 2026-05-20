package minio

import (
	"bytes"
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps the MinIO SDK with project-specific helpers.
type Client struct {
	inner  *minio.Client
	bucket string
}

// New creates a MinIO client and ensures the target bucket exists.
func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	inner, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &Client{inner: inner, bucket: bucket}, nil
}

// EnsureBucket creates the bucket if it does not exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.inner.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return c.inner.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
	}
	return nil
}

// PutObject stores data at the given object key.
func (c *Client) PutObject(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := c.inner.PutObject(ctx, c.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

// GetObject retrieves the full contents of an object.
func (c *Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	obj, err := c.inner.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

// DeleteObject removes an object.
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	return c.inner.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// Inner exposes the underlying minio.Client for listing operations.
func (c *Client) Inner() *minio.Client { return c.inner }
func (c *Client) Bucket() string       { return c.bucket }
