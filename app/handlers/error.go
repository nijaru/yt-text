package handlers

import (
	"log"
	"yt-text/errors"

	"github.com/gofiber/fiber/v2"
)

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*errors.AppError); ok {
		code = e.Code
		message = e.Message
	}

	log.Printf("Error: %v, RequestID: %s, Path: %s, Method: %s",
		err,
		c.Get("X-Request-ID"),
		c.Path(),
		c.Method(),
	)

	return c.Status(code).JSON(fiber.Map{
		"success":    false,
		"error":      message,
		"request_id": c.Get("X-Request-ID"),
	})
}
