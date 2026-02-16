package handlers

import (
	"githubEventsAggregator/internal/events"

	"github.com/gofiber/fiber/v2"
)

type HTTP struct {
	service events.Service
}

func NewHTTP(s events.Service) *HTTP {
	return &HTTP{
		service: s,
	}
}
func (h *HTTP) GetEventById(c *fiber.Ctx) error {
	id := c.Params("id")

	event, err := h.service.GetByID(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "event not found",
		})
	}

	return c.JSON(event)
}
func (h *HTTP) GetEvents(c *fiber.Ctx) error {
	event, err := h.service.GetAll()
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "event not found"})
	}
	return c.JSON(event)

}
