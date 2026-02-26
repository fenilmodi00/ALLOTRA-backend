package services

import (
	"context"
	"fmt"
	"os"

	"github.com/supabase-community/supabase-go"
)

type StorageService struct {
	client *supabase.Client
	bucket string
}

func NewStorageService() (*StorageService, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")

	client := supabase.NewClient(supabaseURL, supabaseKey)
	return &StorageService{client: client, bucket: "ipo-logos"}, nil
}

func (s *StorageService) UploadLogo(ctx context.Context, ipoID string, data []byte, contentType string) (string, error) {
	path := fmt.Sprintf("logos/%s", ipoID)

	resp, err := s.client.Storage.From(s.bucket).Upload(path, data, supabase.StorageUploadOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	return resp, nil
}

func (s *StorageService) GetLogoURL(ipoID string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/logos/%s",
		os.Getenv("SUPABASE_URL"), s.bucket, ipoID)
}
