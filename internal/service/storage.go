package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"log"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
)

// StorageService handles audio file storage via Cloudinary.
type StorageService struct {
	cld *cloudinary.Cloudinary
}

// NewStorageService creates a storage service backed by Cloudinary.
// cloudinaryURL format: cloudinary://api_key:api_secret@cloud_name
func NewStorageService(cloudinaryURL string) (*StorageService, error) {
	cld, err := cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to init cloudinary: %w", err)
	}

	return &StorageService{cld: cld}, nil
}

// Upload saves a file to Cloudinary and writes a local temp copy for Whisper processing.
// Returns the Cloudinary secure URL (for DB) and the local temp path (for Whisper).
// The caller is responsible for cleaning up the temp file after processing.
func (s *StorageService) Upload(file io.Reader, originalFilename string) (cloudinaryURL string, localTempPath string, err error) {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if ext == "" {
		ext = ".webm"
	}
	uniqueName := uuid.New().String()

	// Write to temp file (Whisper needs a local path)
	tmpPath := filepath.Join(os.TempDir(), uniqueName+ext)
	dst, err := os.Create(tmpPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("failed to write temp file: %w", err)
	}
	dst.Close()

	// Open the temp file for Cloudinary upload (pass *os.File, not path string,
	// because the SDK misinterprets Windows paths as URLs)
	uploadFile, err := os.Open(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("failed to open temp file for upload: %w", err)
	}
	defer uploadFile.Close()

	// Upload to Cloudinary
	ctx := context.Background()
	resp, err := s.cld.Upload.Upload(ctx, uploadFile, uploader.UploadParams{
		PublicID:     "bard-audio/" + uniqueName,
		ResourceType: "video", // Cloudinary uses "video" for audio files
	})
	if err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("cloudinary upload failed: %w", err)
	}

	log.Printf("Cloudinary upload response: SecureURL=%s, Error=%v", resp.SecureURL, resp.Error)

	if resp.SecureURL == "" {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("cloudinary returned empty URL, response error: %v", resp.Error)
	}

	return resp.SecureURL, tmpPath, nil
}

// Delete removes an audio asset from Cloudinary given its delivery URL.
func (s *StorageService) Delete(deliveryURL string) error {
	publicID := extractPublicID(deliveryURL)
	if publicID == "" {
		return nil // nothing to delete (e.g. old local paths)
	}

	ctx := context.Background()
	_, err := s.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: "video",
	})
	if err != nil {
		return fmt.Errorf("cloudinary delete failed: %w", err)
	}

	return nil
}

// extractPublicID parses the Cloudinary public_id from a secure delivery URL.
// Example: https://res.cloudinary.com/cloud/video/upload/v123/bard-audio/uuid.webm → bard-audio/uuid
func extractPublicID(rawURL string) string {
	uploadIdx := strings.Index(rawURL, "/upload/")
	if uploadIdx == -1 {
		return ""
	}
	after := rawURL[uploadIdx+len("/upload/"):]
	// Skip version segment "v1234567890/"
	if len(after) > 0 && after[0] == 'v' {
		if slashIdx := strings.Index(after, "/"); slashIdx != -1 {
			after = after[slashIdx+1:]
		}
	}
	// Strip file extension
	if dotIdx := strings.LastIndex(after, "."); dotIdx != -1 {
		after = after[:dotIdx]
	}
	return after
}
