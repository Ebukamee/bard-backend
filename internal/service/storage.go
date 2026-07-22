package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// StorageService handles audio file storage.
// In dev mode it saves to a local "uploads/" folder.
// In production you'd swap this for GCS — the interface stays the same.
type StorageService struct {
	uploadDir string // local directory for storing files
}

// NewStorageService creates a storage service that saves files locally.
func NewStorageService(uploadDir string) (*StorageService, error) {
	// Create the upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload dir: %w", err)
	}

	return &StorageService{uploadDir: uploadDir}, nil
}

// Upload saves a file and returns the storage path (used as the "GCS URL" for now).
// It generates a unique filename to avoid collisions.
func (s *StorageService) Upload(file io.Reader, originalFilename string) (storagePath string, err error) {
	// Generate a unique filename: uuid + original extension
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if ext == "" {
		ext = ".webm" // default for audio recordings
	}
	uniqueName := uuid.New().String() + ext

	fullPath := filepath.Join(s.uploadDir, uniqueName)

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fullPath, nil
}

// Delete removes a file from storage.
func (s *StorageService) Delete(storagePath string) error {
	if err := os.Remove(storagePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}
