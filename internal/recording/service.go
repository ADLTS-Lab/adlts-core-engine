package recording

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type FrameInfo struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

type Service struct {
	repo  *Repository
	minio *MinioClient
}

func NewService(r *Repository, m *MinioClient) *Service {
	return &Service{repo: r, minio: m}
}

// StreamMJPEG streams frames as multipart/x-mixed-replace at ~10fps
func (s *Service) StreamMJPEG(ctx context.Context, w http.ResponseWriter, testID uuid.UUID) error {
	prefix, _, err := s.repo.GetTestRecording(ctx, testID)
	if err != nil {
		return err
	}
	keys, err := s.minio.ListFrames(ctx, prefix)
	if err != nil {
		return err
	}
	boundary := "frame"
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(time.Millisecond * 100) // 10 fps
	defer ticker.Stop()

	for _, k := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			obj, err := s.minio.client.GetObject(ctx, s.minio.bucket, k, minio.GetObjectOptions{})
			if err != nil {
				// skip missing or error frames
				continue
			}
			// Read binary data safely
			data, err := io.ReadAll(obj)
			_ = obj.Close()
			if err != nil {
				// skip frames we can't read
				continue
			}
			_, _ = fmt.Fprintf(w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(data))
			if _, err := w.Write(data); err != nil {
				return err
			}
			_, _ = w.Write([]byte("\r\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
	return nil
}

// FrameList returns a list of frames with presigned URLs valid for expiry
func (s *Service) FrameList(ctx context.Context, testID uuid.UUID, expiry time.Duration) ([]FrameInfo, error) {
	prefix, _, err := s.repo.GetTestRecording(ctx, testID)
	if err != nil {
		return nil, err
	}
	keys, err := s.minio.ListFrames(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]FrameInfo, 0, len(keys))
	for _, k := range keys {
		u, err := s.minio.PresignURL(ctx, k, expiry)
		if err != nil {
			// if presign fails, skip
			continue
		}
		out = append(out, FrameInfo{Key: k, URL: u})
	}
	return out, nil
}
