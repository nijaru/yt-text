package handlers

import (
	"yt-text/errors"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*errors.AppError); ok {
		code = e.Code
		message = e.Message
	}

	log.Error().
		Str("request_id", c.Get("X-Request-ID")).
		Str("path", c.Path()).
		Str("method", c.Method()).
		Int("status", code).
		Err(err).
		Msg("Request error")

	return c.Status(code).JSON(fiber.Map{
		"success":    false,
		"error":      message,
		"request_id": c.Get("X-Request-ID"),
	})
}
