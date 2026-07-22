package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// WhisperService handles audio transcription via Groq's Whisper API.
type WhisperService struct {
	apiKey string
}

// NewWhisperService creates a Whisper service with the given Groq API key.
func NewWhisperService(apiKey string) *WhisperService {
	return &WhisperService{apiKey: apiKey}
}

// Transcribe sends an audio file to Groq's Whisper API and returns the transcript.
func (s *WhisperService) Transcribe(ctx context.Context, audioFilePath string) (string, error) {
	// Open the audio file
	file, err := os.Open(audioFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Build multipart form body
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write the form in a goroutine so we can stream it
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		defer writer.Close()

		// Add the audio file
		part, err := writer.CreateFormFile("file", filepath.Base(audioFilePath))
		if err != nil {
			errCh <- err
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			errCh <- err
			return
		}

		// Add the model field
		if err := writer.WriteField("model", "whisper-large-v3"); err != nil {
			errCh <- err
			return
		}

		// Add response format
		if err := writer.WriteField("response_format", "verbose_json"); err != nil {
			errCh <- err
			return
		}

		errCh <- nil
	}()

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/audio/transcriptions", pr)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for write errors
	if writeErr := <-errCh; writeErr != nil {
		return "", fmt.Errorf("failed to write multipart form: %w", writeErr)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("whisper API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var result struct {
		Text     string  `json:"text"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse whisper response: %w", err)
	}

	return result.Text, nil
}
