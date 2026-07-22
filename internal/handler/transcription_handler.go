package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/bard/bard-backend/internal/domain"
	"github.com/bard/bard-backend/internal/repository"
	"github.com/bard/bard-backend/internal/service"
	"github.com/bard/bard-backend/internal/worker"
	"github.com/gin-gonic/gin"
)

type TranscriptionHandler struct {
	transcriptionRepo *repository.TranscriptionRepository
	storage           *service.StorageService
	processor         *worker.Processor
}

func NewTranscriptionHandler(
	transcriptionRepo *repository.TranscriptionRepository,
	storage *service.StorageService,
	processor *worker.Processor,
) *TranscriptionHandler {
	return &TranscriptionHandler{
		transcriptionRepo: transcriptionRepo,
		storage:           storage,
		processor:         processor,
	}
}

// Upload handles audio file upload and creates a transcription record.
// POST /transcriptions/upload
// Expects multipart form with an "audio" file field.
func (h *TranscriptionHandler) Upload(c *gin.Context) {
	userID := c.GetString("user_id")

	// Get the uploaded file from the multipart form
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "audio file is required"})
		return
	}
	defer file.Close()

	// Validate file size (max 50MB)
	if header.Size > 50*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large, max 50MB"})
		return
	}

	// Save the file to storage
	storagePath, err := h.storage.Upload(file, header.Filename)
	if err != nil {
		log.Printf("Storage upload error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload file"})
		return
	}

	// Create a transcription record in the database
	transcription, err := h.transcriptionRepo.Create(
		c.Request.Context(),
		userID,
		storagePath,
		header.Filename,
	)
	if err != nil {
		log.Printf("Create transcription error: %v", err)
		// Clean up the uploaded file if DB insert fails
		_ = h.storage.Delete(storagePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transcription"})
		return
	}

	// Submit the job for background processing
	h.processor.Submit(worker.Job{
		TranscriptionID: transcription.ID,
		AudioFilePath:   storagePath,
	})

	c.JSON(http.StatusCreated, transcription)
}

// List returns all transcriptions for the authenticated user.
// GET /transcriptions?limit=20&offset=0
func (h *TranscriptionHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Clamp limit to reasonable bounds
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	transcriptions, err := h.transcriptionRepo.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		log.Printf("List transcriptions error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list transcriptions"})
		return
	}

	// Return empty array instead of null when there are no results
	if transcriptions == nil {
		transcriptions = []domain.Transcription{}
	}

	c.JSON(http.StatusOK, gin.H{
		"transcriptions": transcriptions,
		"limit":          limit,
		"offset":         offset,
	})
}

// Get returns a single transcription by ID.
// GET /transcriptions/:id
func (h *TranscriptionHandler) Get(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	transcription, err := h.transcriptionRepo.GetByID(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrTranscriptionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "transcription not found"})
			return
		}
		log.Printf("Get transcription error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	c.JSON(http.StatusOK, transcription)
}

// Delete removes a transcription and its audio file.
// DELETE /transcriptions/:id
func (h *TranscriptionHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	// Get the transcription first so we can delete the file
	transcription, err := h.transcriptionRepo.GetByID(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrTranscriptionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "transcription not found"})
			return
		}
		log.Printf("Get transcription for delete error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	// Delete from database
	if err := h.transcriptionRepo.Delete(c.Request.Context(), id, userID); err != nil {
		log.Printf("Delete transcription error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete transcription"})
		return
	}

	// Delete the audio file from storage (best effort)
	_ = h.storage.Delete(transcription.AudioGCSURL)

	c.JSON(http.StatusOK, gin.H{"message": "transcription deleted"})
}
