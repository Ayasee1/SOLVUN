package handler

import "github.com/gofiber/fiber/v2"

// StoreConfigHandler exposes public storefront configuration. The Midtrans
// client key is public by design (it is embedded in the browser).
type StoreConfigHandler struct {
	midtransClientKey string
	midtransEnv       string
}

func NewStoreConfigHandler(clientKey, env string) *StoreConfigHandler {
	return &StoreConfigHandler{midtransClientKey: clientKey, midtransEnv: env}
}

func (h *StoreConfigHandler) Get(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"midtrans_client_key": h.midtransClientKey,
		"midtrans_env":        h.midtransEnv,
	})
}
