package worker

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/bard/bard-backend/internal/domain"
	"github.com/bard/bard-backend/internal/repository"
	"github.com/bard/bard-backend/internal/service"
)

// Processor handles background audio processing jobs.
type Processor struct {
	transcriptionRepo *repository.TranscriptionRepository
	whisper           *service.WhisperService
	gemini            *service.GeminiService
	jobs              chan Job
}

// Job represents a single transcription processing task.
type Job struct {
	TranscriptionID string
	AudioFilePath   string
	Type            string   // "transcription", "practice", "diary", "music"
	FillerWords     []string // only used for "practice" type
}

// NewProcessor creates a worker that listens for processing jobs.
func NewProcessor(
	transcriptionRepo *repository.TranscriptionRepository,
	whisper *service.WhisperService,
	gemini *service.GeminiService,
) *Processor {
	return &Processor{
		transcriptionRepo: transcriptionRepo,
		whisper:           whisper,
		gemini:            gemini,
		jobs:              make(chan Job, 100),
	}
}

// Submit adds a new job to the processing queue.
func (p *Processor) Submit(job Job) {
	p.jobs <- job
}

// Start begins listening for jobs. Call this in a goroutine.
func (p *Processor) Start(ctx context.Context) {
	log.Println("Background processor started")

	for {
		select {
		case job := <-p.jobs:
			p.processJob(ctx, job)
		case <-ctx.Done():
			log.Println("Background processor shutting down")
			return
		}
	}
}

// processJob routes the job to the right handler based on type.
func (p *Processor) processJob(ctx context.Context, job Job) {
	log.Printf("Processing %s job %s", job.Type, job.TranscriptionID)

	switch job.Type {
	case domain.TypePractice:
		p.processPractice(ctx, job)
	case domain.TypeDiary:
		p.processDiary(ctx, job)
	default:
		p.processTranscription(ctx, job)
	}
}

// processTranscription handles normal transcription jobs.
// Pipeline: Whisper (transcription) → Gemini (filler removal + summary)
func (p *Processor) processTranscription(ctx context.Context, job Job) {
	_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusProcessing, nil)

	transcript, err := p.whisper.Transcribe(ctx, job.AudioFilePath)
	if err != nil {
		log.Printf("Whisper transcription failed for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	result, err := p.gemini.CleanAndSummarize(ctx, transcript)
	if err != nil {
		log.Printf("Gemini processing failed for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	err = p.transcriptionRepo.UpdateResult(ctx, job.TranscriptionID, 0, transcript, result.CleanedTranscript, result.Summary)
	if err != nil {
		log.Printf("Failed to save results for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	log.Printf("Completed transcription %s", job.TranscriptionID)
}

// processPractice handles speech practice jobs.
// Pipeline: Whisper (transcription) → Gemini (filler word analysis)
// The AI identifies how many times each filler word was used and wraps them in **asterisks**
// so the frontend can render them in red.
func (p *Processor) processPractice(ctx context.Context, job Job) {
	_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusProcessing, nil)

	transcript, err := p.whisper.Transcribe(ctx, job.AudioFilePath)
	if err != nil {
		log.Printf("Whisper transcription failed for practice %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	result, err := p.gemini.AnalyzePractice(ctx, transcript, job.FillerWords)
	if err != nil {
		log.Printf("Gemini practice analysis failed for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	// Serialize the filler word counts to JSON for storage
	countsJSON, _ := json.Marshal(result.FillerWordCounts)

	err = p.transcriptionRepo.UpdatePracticeResult(
		ctx,
		job.TranscriptionID,
		0,
		transcript,
		result.CleanedTranscript,
		result.Summary,
		string(countsJSON),
	)
	if err != nil {
		log.Printf("Failed to save practice results for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	log.Printf("Completed practice analysis %s (fillers: %s)", job.TranscriptionID, strings.Join(job.FillerWords, ", "))
}

// processDiary handles daily diary entry jobs.
// Pipeline: Whisper (transcription) → Gemini (cleanup + journal summary)
func (p *Processor) processDiary(ctx context.Context, job Job) {
	_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusProcessing, nil)

	transcript, err := p.whisper.Transcribe(ctx, job.AudioFilePath)
	if err != nil {
		log.Printf("Whisper transcription failed for diary %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	result, err := p.gemini.ProcessDiary(ctx, transcript)
	if err != nil {
		log.Printf("Gemini diary processing failed for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	err = p.transcriptionRepo.UpdateResult(ctx, job.TranscriptionID, 0, transcript, result.CleanedTranscript, result.Summary)
	if err != nil {
		log.Printf("Failed to save diary results for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	log.Printf("Completed diary entry %s", job.TranscriptionID)
}
