package routes

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"golearn/handler"
	"golearn/middleware"
	"golearn/session"
)

// Public limits. One window for every route keeps the table easy to reason
// about; the numbers reflect how often a real shopper can hit each endpoint.
const (
	rateWindow = time.Minute

	// Checkout is the expensive path: stock reservation plus a Midtrans call.
	orderLimit     = 5
	cancelLimit    = 5
	invoiceLimit   = 10
	shippingLimit  = 20
	trackingLimit  = 10
	orderViewLimit = 30
	pageLimit      = 60
	cartLimit      = 60
	catalogLimit   = 120
	// Webhooks are retried by the providers, so the limit is generous: it caps
	// amplification, not normal traffic.
	webhookLimit = 120
)

// SetupRoutes sets up the routes for the Fiber application
func SetupRoutes(app *fiber.App, ph *handler.ProductHandler, oh *handler.OrderHandler, sh *handler.ShippingHandler, sc *handler.StoreConfigHandler, adminAuth *handler.AdminAuthHandler, adminSessions *session.Store, rateLimitEnabled bool) {
	limits := middleware.RateLimits{Enabled: rateLimitEnabled}

	// health check for deploys and uptime probes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "golearn", "build": "preflight-watch-2"})
	})

	// public storefront
	app.Get("/store/config", limits.Handler(pageLimit, rateWindow), sc.Get)
	app.Get("/products", limits.Handler(catalogLimit, rateWindow), ph.GetProducts)
	app.Get("/products/:id", limits.Handler(catalogLimit, rateWindow), ph.GetProductByID)
	app.Post("/cart", limits.Handler(cartLimit, rateWindow), ph.AddToCart)

	app.Post("/shipping/rates", limits.Handler(shippingLimit, rateWindow), sh.Rates)

	// Providers call these; both handlers verify the state with the provider
	// before touching an order.
	app.Post("/shipping/notification", limits.Handler(webhookLimit, rateWindow), oh.ShippingNotification)
	app.Post("/orders/notification", limits.Handler(webhookLimit, rateWindow), oh.Notification)

	// customer-facing pages (HTML)
	app.Get("/order/:id", limits.Handler(pageLimit, rateWindow), oh.OrderPageView)
	app.Get("/track", limits.Handler(orderViewLimit, rateWindow), oh.TrackPageView)
	app.Post("/track", limits.Handler(trackingLimit, rateWindow), oh.TrackPageView)

	app.Post("/orders", limits.Handler(orderLimit, rateWindow), oh.CreateOrder)
	app.Get("/orders/:id", limits.Handler(orderViewLimit, rateWindow), oh.GetOrderByID)
	app.Get("/orders/:id/invoice.pdf", limits.Handler(invoiceLimit, rateWindow), oh.DownloadInvoice)
	app.Post("/orders/:id/cancel", limits.Handler(cancelLimit, rateWindow), oh.CancelOrder)

	// admin session endpoints are public; they are registered before the admin
	// group so the auth middleware never applies to them. Sign-in has its own
	// limiter inside the handler.
	app.Post("/admin/session", adminAuth.Login)
	app.Get("/admin/session", adminAuth.Session)
	app.Delete("/admin/session", adminAuth.Logout)

	// admin only
	admin := app.Group("/admin", middleware.AdminAuth(adminSessions))
	admin.Get("/orders", oh.GetOrders)
	admin.Get("/orders/:id", oh.GetOrderByID)
	admin.Post("/orders/:id/shipment", oh.RetryShipment)
	admin.Post("/orders/:id/refund", oh.RefundOrder)
	admin.Get("/products", ph.GetAllProducts)
	admin.Patch("/products/:id", ph.UpdateProduct)
	admin.Delete("/products/:id", ph.DeleteProduct)
}
