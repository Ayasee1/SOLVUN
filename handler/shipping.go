package handler

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"

	"golearn/repository"
	"golearn/shipping"
)

type ShippingHandler struct {
	provider shipping.Provider
	repo     repository.ProductRepository
}

func NewShippingHandler(provider shipping.Provider, repo repository.ProductRepository) *ShippingHandler {
	return &ShippingHandler{provider: provider, repo: repo}
}

type ratesRequest struct {
	DestinationPostalCode string  `json:"destination_postal_code"`
	DestinationLat        float64 `json:"destination_lat"`
	DestinationLon        float64 `json:"destination_lon"`
	Items                 []struct {
		ProductID int `json:"product_id"`
		Qty       int `json:"qty"`
	} `json:"items"`
}

// Rates returns courier options for the given destination and cart.
// The weight is computed server-side from the product catalog, never trusted
// from the client.
func (h *ShippingHandler) Rates(c *fiber.Ctx) error {
	var req ratesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(req.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Items are required",
		})
	}
	if req.DestinationPostalCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Destination postal code is required",
		})
	}

	weight := 0
	items := make([]shipping.ShipmentItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.Qty <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Item quantity must be greater than zero",
			})
		}

		product, err := h.repo.FindByID(item.ProductID)
		if err != nil {
			if errors.Is(err, repository.ErrProductNotFound) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Unknown product in cart",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to load product",
			})
		}

		weight += product.WeightGram * item.Qty
		items = append(items, shipping.ShipmentItem{
			Name:       product.Name,
			Value:      product.Price,
			Quantity:   item.Qty,
			WeightGram: product.WeightGram,
		})
	}

	if weight == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Product weight is not configured",
		})
	}

	rates, err := h.provider.Rates(shipping.RateRequest{
		DestinationPostalCode: req.DestinationPostalCode,
		DestinationLat:        req.DestinationLat,
		DestinationLon:        req.DestinationLon,
		WeightGram:            weight,
		Items:                 items,
	})
	if err != nil {
		if errors.Is(err, shipping.ErrNotConfigured) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Shipping is not configured",
			})
		}
		log.Printf("shipping rates failed for %s: %v", req.DestinationPostalCode, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Failed to load shipping rates",
		})
	}

	return c.JSON(fiber.Map{
		"weight_gram": weight,
		"rates":       rates,
	})
}
