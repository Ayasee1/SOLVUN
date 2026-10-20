package routes

import (
	"github.com/gofiber/fiber/v2"

	"golearn/handler"
	"golearn/middleware"
)

// SetupRoutes sets up the routes for the Fiber application
func SetupRoutes(app *fiber.App, ph *handler.ProductHandler, oh *handler.OrderHandler, sh *handler.ShippingHandler, sc *handler.StoreConfigHandler, adminAPIKey string) {
	// health check for deploys and uptime probes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "golearn", "build": "preflight-watch"})
	})

	// public storefront
	app.Get("/store/config", sc.Get)
	app.Get("/products", ph.GetProducts)
	app.Get("/products/:id", ph.GetProductByID)
	app.Post("/cart", ph.AddToCart)

	app.Post("/shipping/rates", sh.Rates)
	app.Post("/shipping/notification", oh.ShippingNotification)

	// customer-facing pages (HTML)
	app.Get("/order/:id", oh.OrderPageView)
	app.Get("/track", oh.TrackPageView)
	app.Post("/track", oh.TrackPageView)

	app.Post("/orders", oh.CreateOrder)
	app.Get("/orders/:id", oh.GetOrderByID)
	app.Get("/orders/:id/invoice.pdf", oh.DownloadInvoice)
	app.Post("/orders/:id/cancel", oh.CancelOrder)
	app.Post("/orders/notification", oh.Notification)

	// admin only
	admin := app.Group("/admin", middleware.AdminAuth(adminAPIKey))
	admin.Get("/orders", oh.GetOrders)
	admin.Get("/orders/:id", oh.GetOrderByID)
	admin.Get("/products", ph.GetAllProducts)
	admin.Patch("/products/:id", ph.UpdateProduct)
	admin.Delete("/products/:id", ph.DeleteProduct)
}
