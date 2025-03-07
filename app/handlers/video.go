package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	ytError "yt-text/errors"
	"yt-text/logger"
	"yt-text/models"
	"yt-text/services/video"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type VideoHandler struct {
	service video.Service
}

func NewVideoHandler(service video.Service) *VideoHandler {
	return &VideoHandler{
		service: service,
	}
}

// RegisterRoutes registers the routes for the video handlers
func (h *VideoHandler) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api")
	
	// REST endpoints
	api.Post("/transcribe", h.TranscribeVideo)
	api.Get("/transcribe/:id", h.GetTranscription)
	api.Delete("/transcribe/:id", h.CancelTranscription) // New endpoint for job cancellation
	
	// WebSocket endpoint
	app.Use("/ws/transcribe", websocket.New(h.TranscribeWebSocket))
}

// TranscribeVideo handles video transcription requests
func (h *VideoHandler) TranscribeVideo(c *fiber.Ctx) error {
	// Support both form and JSON input
	var url string
	
	// Check if content type is JSON
	if c.Get("Content-Type") == "application/json" {
		var req struct {
			URL string `json:"url"`
		}
		
		if err := c.BodyParser(&req); err != nil {
			return &errors.AppError{
				Code:    fiber.StatusBadRequest,
				Message: "Invalid request body",
			}
		}
		url = req.URL
	} else {
		// Fallback to form value
		url = c.FormValue("url")
	}
	
	if url == "" {
		return &errors.AppError{
			Code:    fiber.StatusBadRequest,
			Message: "URL is required",
		}
	}
	
	// Start transcription process
	video, err := h.service.Transcribe(c.Context(), url)
	if err != nil {
		return err
	}
	
	// Return response with video ID
	return c.JSON(fiber.Map{
		"success": true,
		"data":    models.NewVideoResponse(video),
	})
}

// GetTranscription retrieves a transcription by ID
func (h *VideoHandler) GetTranscription(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return &errors.AppError{
			Code:    fiber.StatusBadRequest,
			Message: "ID is required",
		}
	}
	
	// Get the video record
	video, err := h.service.GetTranscription(c.Context(), id)
	if err != nil {
		return err
	}
	
	// If the video has file-based storage, fetch the text
	if video.TranscriptionPath != "" && video.Transcription == "" {
		text, err := h.service.GetTranscriptionText(c.Context(), video)
		if err != nil {
			return err
		}
		
		// Temporarily set transcription for response
		// This doesn't modify the database record
		video.Transcription = text
	}
	
	return c.JSON(fiber.Map{
		"success": true,
		"data":    models.NewVideoResponse(video),
	})
}

// WebSocket message types
type webSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type transcribeRequest struct {
	URL string `json:"url"`
}

type cancelRequest struct {
	ID string `json:"id"`
}

// Error codes for WebSocket communication
const (
	ErrorCodeGeneral     = "ERR_GENERAL"
	ErrorCodeInvalidURL  = "ERR_INVALID_URL"
	ErrorCodeJobNotFound = "ERR_JOB_NOT_FOUND"
	ErrorCodeTimeout     = "ERR_TIMEOUT"
	ErrorCodeCancelled   = "ERR_CANCELLED"
)

type progressUpdate struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress,omitempty"`
	Message  string  `json:"message,omitempty"`
	Stage    string  `json:"stage,omitempty"`    // download, process, complete
	Substage string  `json:"substage,omitempty"` // detailed substage information
	ETA      int     `json:"eta,omitempty"`      // estimated seconds remaining
}

// TranscribeWebSocket handles WebSocket connections for real-time transcription updates
func (h *VideoHandler) TranscribeWebSocket(c *websocket.Conn) {
	// Create a unique connection ID
	connID := uuid.New().String()
	logger.Info("WebSocket connection established", "conn_id", connID)
	
	// Set up context with timeout for the entire WebSocket session
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	
	// Subscribe to transcription updates channel
	updateChannel := make(chan *progressUpdate, 10)
	errorChannel := make(chan error, 1)
	
	// Handle client messages
	go func() {
		for {
			messageType, msg, err := c.ReadMessage()
			if err != nil {
				logger.Error("WebSocket read error", "error", err, "conn_id", connID)
				errorChannel <- err
				return
			}
			
			// Only process text messages
			if messageType != websocket.TextMessage {
				continue
			}
			
			// Parse message
			var wsMsg webSocketMessage
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				logger.Error("WebSocket message parse error", "error", err, "conn_id", connID)
				sendErrorMessage(c, "Invalid message format", ErrorCodeGeneral)
				continue
			}
			
			// Generate request ID for tracking
			requestID := uuid.New().String()
			
			// Handle message by type
			switch wsMsg.Type {
			case "transcribe":
				// Handle transcription request
				var req transcribeRequest
				if err := json.Unmarshal(wsMsg.Payload, &req); err != nil {
					logger.Error("Invalid transcribe payload", "error", err, "conn_id", connID, "request_id", requestID)
					sendErrorMessage(c, "Invalid transcribe request format", ErrorCodeGeneral, requestID)
					continue
				}
				
				// Validate URL
				if req.URL == "" {
					sendErrorMessage(c, "URL is required", ErrorCodeInvalidURL, requestID)
					continue
				}
				
				// Start transcription with request ID for tracking
				go startTranscription(ctx, h.service, req.URL, updateChannel, errorChannel, requestID)
				
			case "cancel":
				// Handle cancellation request
				var req cancelRequest
				if err := json.Unmarshal(wsMsg.Payload, &req); err != nil {
					logger.Error("Invalid cancel payload", "error", err, "conn_id", connID, "request_id", requestID)
					sendErrorMessage(c, "Invalid cancel request format", ErrorCodeGeneral, requestID)
					continue
				}
				
				// Validate job ID
				if req.ID == "" {
					sendErrorMessage(c, "Job ID is required", ErrorCodeJobNotFound, requestID)
					continue
				}
				
				// Process cancellation in a goroutine
				go func() {
					// First check if job exists and is in processing state
					video, err := h.service.GetTranscription(ctx, req.ID)
					if err != nil {
						logger.Error("Job not found for cancellation", "error", err, "job_id", req.ID, "request_id", requestID)
						errorChannel <- fmt.Errorf("%s: %w", ErrorCodeJobNotFound, err)
						return
					}
					
					// Check if job is in processing state
					if video.Status != models.StatusProcessing {
						logger.Warn("Attempted to cancel non-processing job", "job_id", req.ID, "status", video.Status, "request_id", requestID)
						errorChannel <- fmt.Errorf("%s: only in-progress jobs can be canceled", ErrorCodeGeneral)
						return
					}
					
					// Attempt to cancel the job
					success := h.service.CancelJob(req.ID)
					
					if success {
						// Update the video status in the database
						video.Status = models.StatusFailed
						video.Error = "Canceled by user"
						video.UpdatedAt = time.Now()
						
						// Save the canceled status
						saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer saveCancel()
						
						if saveErr := h.service.GetRepository().Save(saveCtx, video); saveErr != nil {
							logger.Error("Failed to save canceled status", "error", saveErr, "job_id", req.ID)
						}
						
						// Send cancellation success update
						updateChannel <- &progressUpdate{
							ID:       req.ID,
							Status:   string(models.StatusFailed),
							Progress: 1.0,
							Message:  "Job canceled by user",
							Stage:    "complete",
						}
					} else {
						// Cancellation failed
						logger.Error("Failed to cancel job", "job_id", req.ID, "request_id", requestID)
						errorChannel <- fmt.Errorf("%s: unable to cancel job - job may be already completed", ErrorCodeGeneral)
					}
				}()
			
			case "ping":
				// Handle ping messages
				sendMessage(c, "pong", nil)

			case "status":
				// Handle status request for a job
				var req struct {
					ID string `json:"id"`
				}
				
				if err := json.Unmarshal(wsMsg.Payload, &req); err != nil {
					logger.Error("Invalid status request", "error", err, "conn_id", connID, "request_id", requestID)
					sendErrorMessage(c, "Invalid status request format", ErrorCodeGeneral, requestID)
					continue
				}
				
				// Validate job ID
				if req.ID == "" {
					sendErrorMessage(c, "Job ID is required", ErrorCodeJobNotFound, requestID)
					continue
				}
				
				// Retrieve job status and send it as a progress update
				go func() {
					// Get the video record
					video, err := h.service.GetTranscription(ctx, req.ID)
					if err != nil {
						logger.Error("Job not found for status request", "error", err, "job_id", req.ID, "request_id", requestID)
						errorChannel <- fmt.Errorf("%s: %w", ErrorCodeJobNotFound, err)
						return
					}
					
					// Prepare a status update based on its current state
					var progress float64 = 0
					var message string
					var stage string
					var substage string
					var eta int
					
					switch video.Status {
					case models.StatusCompleted:
						progress = 1.0
						stage = "complete"
						substage = "done"
						eta = 0
						message = "Transcription complete"
						
						if video.Source == "youtube_api" {
							message = "Retrieved official captions"
						} else {
							message = "Generated transcription complete"
						}
						
					case models.StatusFailed:
						progress = 1.0
						stage = "complete"
						substage = "failed"
						eta = 0
						message = "Transcription failed: " + video.Error
						
					case models.StatusProcessing:
						// Calculate an estimate based on time elapsed
						elapsed := time.Since(video.CreatedAt)
						totalEstimatedDuration := 10 * time.Minute
						
						// ETA calculation
						remainingTime := totalEstimatedDuration - elapsed
						if remainingTime < 0 {
							remainingTime = 10 * time.Second
						}
						eta = int(remainingTime.Seconds())
						
						// For long-running jobs, use a simpler progress estimation
						if elapsed < 30*time.Second {
							progress = 0.1
							stage = "download"
							substage = "video"
							message = "Downloading video"
						} else if elapsed < 2*time.Minute {
							progress = 0.3
							stage = "process"
							substage = "analyzing"
							message = "Analyzing audio"
						} else {
							// Calculate progress as a function of elapsed time
							progress = math.Min(0.9, float64(elapsed)/float64(totalEstimatedDuration))
							stage = "process"
							substage = "transcribing"
							message = "Generating transcription"
						}
					}
					
					// Send the status update
					updateChannel <- &progressUpdate{
						ID:       video.ID,
						Status:   string(video.Status),
						Progress: progress,
						Message:  message,
						Stage:    stage,
						Substage: substage,
						ETA:      eta,
					}
				}()
				
			default:
				logger.Warn("Unknown message type received", "type", wsMsg.Type, "conn_id", connID, "request_id", requestID)
				sendErrorMessage(c, "Unknown message type", ErrorCodeGeneral, requestID)
			}
		}
	}()
	
	// Send updates to client
	for {
		select {
		case update := <-updateChannel:
			// Send progress update
			if err := sendMessage(c, "progress", update); err != nil {
				logger.Error("Failed to send progress update", "error", err, "conn_id", connID)
				return
			}
			
		case err := <-errorChannel:
			// Parse error code from error message if available
			errMsg := err.Error()
			errCode := ErrorCodeGeneral
			
			// Check if the error starts with an error code
			for _, code := range []string{
				ErrorCodeGeneral, ErrorCodeInvalidURL, 
				ErrorCodeJobNotFound, ErrorCodeTimeout, 
				ErrorCodeCancelled,
			} {
				if strings.HasPrefix(errMsg, code+":") {
					errCode = code
					errMsg = strings.TrimPrefix(errMsg, code+": ")
					break
				}
			}
			
			// Send error and close connection
			logger.Error("Transcription error", "error", errMsg, "code", errCode, "conn_id", connID)
			sendErrorMessage(c, "Transcription error: "+errMsg, errCode)
			return
			
		case <-ctx.Done():
			// Context timeout or cancellation
			logger.Warn("WebSocket context done", "error", ctx.Err(), "conn_id", connID)
			sendErrorMessage(c, "Connection timeout", ErrorCodeTimeout)
			return
		}
	}
}

// startTranscription begins a transcription job and sends updates
func startTranscription(
	ctx context.Context, 
	service video.Service, 
	url string, 
	updates chan<- *progressUpdate,
	errors chan<- error,
	requestID string,
) {
	// Start transcription
	video, err := service.Transcribe(ctx, url)
	if err != nil {
		logger.Error("Transcription start error", "error", err, "url", url, "request_id", requestID)
		errors <- fmt.Errorf("%s: %w", ErrorCodeGeneral, err)
		return
	}
	
	// Send initial update
	updates <- &progressUpdate{
		ID:       video.ID,
		Status:   string(video.Status),
		Progress: 0.0,
		Message:  "Transcription started",
		Stage:    "download",
		Substage: "preparing",
		ETA:      600, // Initial estimate: 10 minutes
	}
	
	// Poll for updates until complete
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check status
			current, err := service.GetTranscription(ctx, video.ID)
			if err != nil {
				errors <- err
				return
			}
			
			// Calculate progress and detailed message
			var progress float64 = 0
			var message string
			var stage string
			var substage string
			var eta int
			
			switch current.Status {
			case models.StatusProcessing:
				// Check how far along we are in the process
				elapsed := time.Since(current.CreatedAt)
				totalEstimatedDuration := 10 * time.Minute // Assuming ~10 min max
				
				// Calculate estimated time remaining
				remainingTime := totalEstimatedDuration - elapsed
				if remainingTime < 0 {
					remainingTime = 10 * time.Second // Minimum ETA
				}
				eta = int(remainingTime.Seconds())
				
				if elapsed < 15*time.Second {
					// Initial preparation phase
					progress = float64(elapsed) / float64(15*time.Second) * 0.05
					stage = "download"
					substage = "preparing"
					message = "Preparing download"
				} else if elapsed < 30*time.Second {
					// Initial downloading phase
					progress = 0.05 + (float64(elapsed-15*time.Second) / 
								  float64(15*time.Second) * 0.1)
					stage = "download"
					substage = "video"
					message = "Downloading video"
				} else if elapsed < 45*time.Second {
					// Audio extraction phase
					progress = 0.15 + (float64(elapsed-30*time.Second) / 
								   float64(15*time.Second) * 0.1)
					stage = "download"
					substage = "audio"
					message = "Extracting audio"
				} else if elapsed < 2*time.Minute {
					// Early processing phase
					progress = 0.25 + (float64(elapsed-45*time.Second) / 
								  float64(75*time.Second) * 0.15)
					stage = "process"
					substage = "analyzing"
					message = "Analyzing audio"
				} else if elapsed < 5*time.Minute {
					// Mid processing phase
					progress = 0.4 + (float64(elapsed-2*time.Minute) / 
								  float64(3*time.Minute) * 0.3)
					stage = "process"
					substage = "transcribing"
					message = "Transcribing audio"
				} else {
					// Final processing phase
					baseProgress := 0.7
					remainingProgress := 0.25 // Leave room for final 5%
					elapsedSinceProcessingBegan := elapsed - 5*time.Minute
					remainingEstimatedTime := totalEstimatedDuration - 5*time.Minute
					
					if remainingEstimatedTime <= 0 {
						remainingEstimatedTime = 10 * time.Second
					}
					
					additionalProgress := float64(elapsedSinceProcessingBegan) / 
									 float64(remainingEstimatedTime) * remainingProgress
					
					progress = baseProgress + additionalProgress
					if progress > 0.95 {
						progress = 0.95 // Cap at 95% until complete
					}
					
					stage = "process"
					substage = "finalizing"
					message = "Finalizing transcription"
				}
				
			case models.StatusCompleted:
				progress = 1.0
				stage = "complete"
				substage = "done"
				eta = 0
				message = "Transcription complete"
				
				// Get the source for a more descriptive message
				if current.Source == "youtube_api" {
					message = "Retrieved official captions"
				} else {
					message = "Generated transcription complete"
				}
				
				// Send final update
				updates <- &progressUpdate{
					ID:       current.ID,
					Status:   string(current.Status),
					Progress: progress,
					Message:  message,
					Stage:    stage,
					Substage: substage,
					ETA:      eta,
				}
				return
				
			case models.StatusFailed:
				progress = 1.0
				stage = "complete" // We're done, even though it failed
				substage = "failed"
				eta = 0
				message = "Transcription failed: " + current.Error
				
				// Send final update
				updates <- &progressUpdate{
					ID:       current.ID,
					Status:   string(current.Status),
					Progress: progress,
					Message:  message,
					Stage:    stage,
					Substage: substage,
					ETA:      eta,
				}
				return
			}
			
			// Send update
			updates <- &progressUpdate{
				ID:       current.ID,
				Status:   string(current.Status),
				Progress: progress,
				Message:  message,
				Stage:    stage,
				Substage: substage,
				ETA:      eta,
			}
			
		case <-ctx.Done():
			// Context canceled
			errors <- ctx.Err()
			return
		}
	}
}

// sendMessage sends a typed message through the WebSocket
func sendMessage(c *websocket.Conn, msgType string, payload interface{}) error {
	var payloadBytes []byte
	var err error
	
	if payload != nil {
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	} else {
		payloadBytes = []byte("{}")
	}
	
	msg := webSocketMessage{
		Type:    msgType,
		Payload: payloadBytes,
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	return c.WriteMessage(websocket.TextMessage, data)
}

// Enhanced error payload with code
type errorPayload struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"` // To track which request failed
}

// sendErrorMessage sends an enhanced error message to the client
func sendErrorMessage(c *websocket.Conn, errorMsg string, errorCode string, requestID ...string) {
	reqID := ""
	if len(requestID) > 0 {
		reqID = requestID[0]
	}
	
	sendMessage(c, "error", errorPayload{
		Error:     errorMsg,
		Code:      errorCode,
		RequestID: reqID,
	})
}

// CancelTranscription cancels an in-progress transcription job
func (h *VideoHandler) CancelTranscription(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return &ytError.AppError{
			Code:    fiber.StatusBadRequest,
			Message: "ID is required",
		}
	}
	
	// First, check if the video exists in the database
	video, err := h.service.GetTranscription(c.Context(), id)
	if err != nil {
		return err // This will handle not found errors
	}
	
	// Only allow cancellation for videos that are in processing state
	if video.Status != models.StatusProcessing {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "Only in-progress transcriptions can be canceled",
		})
	}
	
	// Attempt to cancel the job
	success := h.service.CancelJob(id)
	
	if success {
		// Update the video status in the database to canceled
		video.Status = models.StatusFailed
		video.Error = "Canceled by user"
		video.UpdatedAt = time.Now()
		
		// Try to save the canceled status
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		
		// The error is logged but we still return success to the user
		// since the job was successfully canceled
		if saveErr := h.service.GetRepository().Save(saveCtx, video); saveErr != nil {
			logger.Error("Failed to save canceled status", "error", saveErr, "video_id", id)
		}
		
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Transcription job canceled",
		})
	} else {
		// Job couldn't be canceled
		return c.JSON(fiber.Map{
			"success": false,
			"message": "Unable to cancel job - job may be already completed",
		})
	}
}