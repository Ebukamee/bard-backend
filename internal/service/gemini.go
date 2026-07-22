package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// GeminiService handles AI-powered filler removal and summarization.
type GeminiService struct {
	apiKey string
}

// NewGeminiService creates a Gemini service with the given API key.
func NewGeminiService(apiKey string) *GeminiService {
	return &GeminiService{apiKey: apiKey}
}

// CleanAndSummarizeResult holds the output of filler removal and summarization.
type CleanAndSummarizeResult struct {
	CleanedTranscript string `json:"cleaned_transcript"`
	Summary           string `json:"summary"`
}

// CleanAndSummarize takes a raw transcript and returns a cleaned version
// (no filler words) and a summary.
func (s *GeminiService) CleanAndSummarize(ctx context.Context, transcript string) (*CleanAndSummarizeResult, error) {
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": fmt.Sprintf(`You are a transcript editor and summarizer.

Given this raw transcript:
---
%s
---

Return a JSON response with exactly these fields:

{
  "cleaned_transcript": "The same transcript but with ALL filler words removed. Remove: um, uh, hmm, like (when used as filler), you know, I mean, sort of, kind of, basically, actually, literally, right, so (when used as filler at start of sentences), well (when used as filler), er, ah, oh, let me think, how do I say this, what's the word",
  "summary": "A straightforward summary of what was actually said in the transcript. Write it in first/second person as appropriate, NOT third person. Do NOT add your own interpretation or meaning — just summarize what was said."
}

IMPORTANT:
- The cleaned_transcript should still be readable and make grammatical sense after removing fillers
- The summary should be a direct, plain summary of what was said — not an analysis or interpretation
- Do NOT use phrases like "The speaker expresses" or "The speaker asserts" — just summarize the content directly
- Return ONLY valid JSON, no markdown code blocks, no extra text`, transcript),
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":      0.1,
			"responseMimeType": "application/json",
			"thinkingConfig": map[string]interface{}{
				"thinkingBudget": 0,
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", s.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	// Concatenate all parts (Gemini sometimes splits across multiple parts)
	var sb strings.Builder
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	jsonStr := strings.TrimSpace(sb.String())
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimPrefix(jsonStr, "```")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)

	var result CleanAndSummarizeResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Try to salvage by extracting fields manually
		log.Printf("Gemini JSON parse error: %v", err)
		log.Printf("Raw response length: %d", len(jsonStr))

		// Attempt a more lenient decode using json.Decoder
		decoder := json.NewDecoder(strings.NewReader(jsonStr))
		decoder.DisallowUnknownFields()
		if decErr := decoder.Decode(&result); decErr != nil {
			return nil, fmt.Errorf("failed to parse gemini JSON: %w (raw truncated: %.500s)", err, jsonStr)
		}
	}

	return &result, nil
}
