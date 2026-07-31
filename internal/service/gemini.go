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

// PracticeResult holds the output of speech practice analysis.
// Includes filler word counts so the frontend can highlight them in red.
type PracticeResult struct {
	CleanedTranscript string         `json:"cleaned_transcript"`
	Summary           string         `json:"summary"`
	FillerWordCounts  map[string]int `json:"filler_word_counts"` // e.g. {"um": 5, "like": 3}
}

// DiaryResult holds the output of diary entry processing.
type DiaryResult struct {
	CleanedTranscript string `json:"cleaned_transcript"`
	Summary           string `json:"summary"`
}

// CleanAndSummarize takes a raw transcript and returns a cleaned version
// (no filler words) and a summary. Used for the normal "transcription" type.
func (s *GeminiService) CleanAndSummarize(ctx context.Context, transcript string) (*CleanAndSummarizeResult, error) {
	prompt := fmt.Sprintf(`You are a transcript editor and summarizer.

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
- Return ONLY valid JSON, no markdown code blocks, no extra text`, transcript)

	var result CleanAndSummarizeResult
	if err := s.callGemini(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AnalyzePractice processes a speech practice recording. It identifies and counts
// the specific filler words the user wants to track, so the frontend can render
// those words in red and show how many times each was said.
func (s *GeminiService) AnalyzePractice(ctx context.Context, transcript string, fillerWords []string) (*PracticeResult, error) {
	fillerList := strings.Join(fillerWords, ", ")

	prompt := fmt.Sprintf(`You are a speech coach analyzing a practice session.

The user is trying to eliminate these specific filler words from their speech: [%s]

Here is the raw transcript of their practice session:
---
%s
---

Return a JSON response with exactly these fields:

{
  "cleaned_transcript": "The exact same transcript but with the user's specified filler words wrapped in double asterisks like **um** so they can be highlighted. Keep ALL other words exactly as spoken. Only wrap the specific filler words listed above, not other words.",
  "summary": "A brief coaching summary: how the speaker did overall, which filler words appeared most, and a tip for improvement. Keep it encouraging and actionable. Write in second person (you).",
  "filler_word_counts": {"word1": count, "word2": count}
}

IMPORTANT:
- filler_word_counts must ONLY contain the specific words the user listed: [%s]
- If a listed word was not used at all, include it with count 0
- The cleaned_transcript must wrap EVERY occurrence of the filler words in **double asterisks**
- Count is case-insensitive ("Um" and "um" both count)
- Only count a word as filler when it's used as filler, not when it has real meaning (e.g. "like" in "I like pizza" is NOT filler)
- Return ONLY valid JSON, no markdown code blocks, no extra text`, fillerList, transcript, fillerList)

	var result PracticeResult
	if err := s.callGemini(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ProcessDiary processes a daily diary audio entry. It cleans up the transcript
// and creates a polished journal-style summary.
func (s *GeminiService) ProcessDiary(ctx context.Context, transcript string) (*DiaryResult, error) {
	prompt := fmt.Sprintf(`You are a personal journal assistant.

The user recorded a daily diary entry by speaking. Here is the raw transcript:
---
%s
---

Return a JSON response with exactly these fields:

{
  "cleaned_transcript": "A cleaned-up, well-structured version of what the user said. Reorganize the content CHRONOLOGICALLY — if the user mentions something later in the recording that happened earlier in the day, move it to its correct place in the timeline. Remove filler words (um, uh, like, you know, etc.), fix grammar issues from speech-to-text, group related topics together, and make it read naturally. Keep the first-person voice and all the content intact.",
  "summary": "A concise, structured diary-style summary of the user's day. Write in first person as if it's their journal entry. Organize by time of day or topic — whichever flows better. Capture key events, thoughts, and feelings. Keep it warm and personal."
}

IMPORTANT:
- CHRONOLOGICAL ORDER: If the user says "oh and this morning I also..." halfway through talking about their evening, put that morning event in the correct chronological spot
- GROUP related topics together — don't scatter the same subject across multiple paragraphs
- Keep the user's voice and personality in both fields
- The cleaned_transcript should feel natural, like a well-written version of what they said, not just a direct transcription
- The summary should read like a polished diary entry, not a clinical report
- Use paragraph breaks to separate different parts of the day or different topics
- Return ONLY valid JSON, no markdown code blocks, no extra text`, transcript)

	var result DiaryResult
	if err := s.callGemini(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// callGemini is the shared helper that sends a prompt to Gemini and unmarshals the response.
func (s *GeminiService) callGemini(ctx context.Context, prompt string, result interface{}) error {
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
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
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", s.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gemini API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(body))
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
		return fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("empty response from gemini")
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

	if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
		log.Printf("Gemini JSON parse error: %v", err)
		log.Printf("Raw response length: %d", len(jsonStr))

		decoder := json.NewDecoder(strings.NewReader(jsonStr))
		if decErr := decoder.Decode(result); decErr != nil {
			return fmt.Errorf("failed to parse gemini JSON: %w (raw truncated: %.500s)", err, jsonStr)
		}
	}

	return nil
}
