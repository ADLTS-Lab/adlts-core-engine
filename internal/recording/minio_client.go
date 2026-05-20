package recording

import (
	"context"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClient struct {
	client *minio.Client
	bucket string
}

func NewMinioClientFromEnv() (*MinioClient, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	access := os.Getenv("MINIO_ACCESS_KEY")
	secret := os.Getenv("MINIO_SECRET_KEY")
	bucket := os.Getenv("MINIO_BUCKET")
	if endpoint == "" || access == "" || secret == "" || bucket == "" {
		return nil, ErrMinioNotConfigured
	}
	secure := false
	if strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "wss://") {
		secure = true
		// strip scheme for minio client
		u, err := url.Parse(endpoint)
		if err == nil {
			endpoint = u.Host
		}
	}
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(access, secret, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}
	return &MinioClient{client: cli, bucket: bucket}, nil
}

var ErrMinioNotConfigured = &MinioError{"minio not configured"}

type MinioError struct{ msg string }

func (e *MinioError) Error() string { return e.msg }

// ListFrames lists object keys under prefix and returns lexicographically sorted keys
func (m *MinioClient) ListFrames(ctx context.Context, prefix string) ([]string, error) {
	ch := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	var keys []string
	for obj := range ch {
		if obj.Err != nil {
			// skip troublesome object but continue
			continue
		}
		keys = append(keys, obj.Key)
	}
	sort.Strings(keys)
	return keys, nil
}

// PresignURL returns a presigned GET URL for an object valid for `expiry` duration
func (m *MinioClient) PresignURL(ctx context.Context, object string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := m.client.PresignedGetObject(ctx, m.bucket, object, expiry, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
