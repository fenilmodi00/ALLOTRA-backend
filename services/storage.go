package services

import (
	"bytes"
	"context"
	"fmt"
	"os"

	storage_go "github.com/supabase-community/storage-go"
	supabase "github.com/supabase-community/supabase-go"
)

// StorageService wraps Supabase Storage for IPO logo management.
type StorageService struct {
	client *supabase.Client
	bucket string
}

// NewStorageService creates a StorageService using env vars SUPABASE_URL and
// SUPABASE_SERVICE_KEY.
func NewStorageService() (*StorageService, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		return nil, fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY are required")
	}

	client, err := supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}

	return &StorageService{client: client, bucket: "ipo-logos"}, nil
}

// UploadLogo uploads raw image bytes to the ipo-logos bucket and returns the
// storage key of the uploaded object.
func (s *StorageService) UploadLogo(_ context.Context, ipoID string, data []byte, contentType string) (string, error) {
	relativePath := fmt.Sprintf("logos/%s", ipoID)

	opts := storage_go.FileOptions{
		ContentType: &contentType,
	}

	resp, err := s.client.Storage.UploadFile(s.bucket, relativePath, bytes.NewReader(data), opts)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("upload error: %s", resp.Error)
	}

	return resp.Key, nil
}

// GetLogoURL returns the public URL for a logo stored in the ipo-logos bucket.
func (s *StorageService) GetLogoURL(ipoID string) string {
	relativePath := fmt.Sprintf("logos/%s", ipoID)
	resp := s.client.Storage.GetPublicUrl(s.bucket, relativePath)
	return resp.SignedURL
}
