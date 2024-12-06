package handlers

import (
	"yt-text/errors"
	"yt-text/services/video"

	"github.com/gofiber/fiber/v2"
)

type VideoHandler struct {
	service video.Service
}

func NewVideoHandler(service video.Service) *VideoHandler {
	return &VideoHandler{service: service}
}

func (h *VideoHandler) Transcribe(c *fiber.Ctx) error {
	url := c.FormValue("url")
	if url == "" {
		return &errors.AppError{
			Code:    fiber.StatusBadRequest,
			Message: "URL is required",
		}
	}

	video, err := h.service.Transcribe(c.Context(), url)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    video,
	})
}

func (h *VideoHandler) GetTranscription(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return &errors.AppError{
			Code:    fiber.StatusBadRequest,
			Message: "ID is required",
		}
	}

	video, err := h.service.GetTranscription(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    video,
	})
}
