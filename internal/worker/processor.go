package worker

import (
	"context"
	"log"

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

// processJob handles a single transcription job.
// Pipeline: Whisper (transcription) → Gemini (filler removal + summary)
func (p *Processor) processJob(ctx context.Context, job Job) {
	log.Printf("Processing transcription %s", job.TranscriptionID)

	_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusProcessing, nil)

	// Step 1: Transcribe with Whisper
	log.Printf("Step 1: Transcribing with Whisper for %s", job.TranscriptionID)
	transcript, err := p.whisper.Transcribe(ctx, job.AudioFilePath)
	if err != nil {
		log.Printf("Whisper transcription failed for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	// Step 2: Clean and summarize with Gemini
	log.Printf("Step 2: Cleaning and summarizing with Gemini for %s", job.TranscriptionID)
	result, err := p.gemini.CleanAndSummarize(ctx, transcript)
	if err != nil {
		log.Printf("Gemini processing failed for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	// Save results
	err = p.transcriptionRepo.UpdateResult(
		ctx,
		job.TranscriptionID,
		0,
		transcript,
		result.CleanedTranscript,
		result.Summary,
	)
	if err != nil {
		log.Printf("Failed to save results for %s: %v", job.TranscriptionID, err)
		errMsg := err.Error()
		_ = p.transcriptionRepo.UpdateStatus(ctx, job.TranscriptionID, domain.StatusFailed, &errMsg)
		return
	}

	log.Printf("Completed transcription %s", job.TranscriptionID)
}
