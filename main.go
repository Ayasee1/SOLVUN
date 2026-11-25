package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // embed tzdata so TZ works in minimal containers

	// fiber
	"github.com/gofiber/fiber/v2"
	// cors
	"github.com/gofiber/fiber/v2/middleware/cors"
	// panic recovery
	"github.com/gofiber/fiber/v2/middleware/recover"
	// env
	"github.com/joho/godotenv"
	// catalog
	"golearn/catalog"
	// alerts
	"golearn/alerts"
	// config
	"golearn/config"
	// db
	"golearn/db"
	// handler
	"golearn/handler"
	// model
	"golearn/model"
	// notification
	"golearn/notification"
	// payment
	"golearn/payment"
	// repository
	"golearn/repository"
	// routes
	"golearn/routes"
	// session
	"golearn/session"
	// shipping
	"golearn/shipping"
	// sweeper
	"golearn/sweeper"
)

func main() {
	// Load .env if present (ignored in production where real env vars are used)
	_ = godotenv.Load()

	// cancel on SIGINT/SIGTERM so the sweeper stops and in-flight requests drain
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	var productRepo repository.ProductRepository
	var orderRepo repository.OrderRepository

	if cfg.DatabaseURL == "" {
		// no database configured -> in-memory (dev/tests)
		log.Println("DATABASE_URL is empty, using in-memory repositories")
		memoryProducts := repository.NewMemoryProductRepository()
		memoryOrders := repository.NewMemoryOrderRepository()
		memoryOrders.SetProducts(memoryProducts)
		productRepo = memoryProducts
		orderRepo = memoryOrders
	} else {
		pool, err := db.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("database connection failed: %v", err)
		}
		defer pool.Close()

		if err := db.Migrate(ctx, pool); err != nil {
			log.Fatalf("database migration failed: %v", err)
		}

		// product content comes from the catalog files; fall back to the
		// built-in seed when the directory is missing.
		catalogs := model.Catalogs
		if loaded, err := catalog.Load(cfg.CatalogDir); err != nil {
			log.Printf("could not load catalog dir %q (%v); using built-in seed", cfg.CatalogDir, err)
		} else if len(loaded) > 0 {
			catalogs = loaded
			log.Printf("loaded %d catalogs from %q", len(loaded), cfg.CatalogDir)
		}

		if err := db.Seed(ctx, pool, catalogs); err != nil {
			log.Fatalf("database seed failed: %v", err)
		}

		log.Println("connected to Postgres")
		productRepo = repository.NewPostgresProductRepository(pool)
		orderRepo = repository.NewPostgresOrderRepository(pool)
	}

	paymentGateway := payment.NewMidtransGateway(cfg.MidtransServerKey, cfg.MidtransEnv, cfg.MidtransExpiryMinutes, cfg.StoreBaseURL)

	var mailer notification.Sender
	switch {
	case cfg.ResendAPIKey != "":
		mailer = notification.NewResendSender(cfg.ResendAPIKey, cfg.MailFrom)
		log.Printf("email enabled via Resend (from=%s)", cfg.MailFrom)
	case cfg.SMTPHost != "":
		mailer = notification.SMTPSender{
			Host: cfg.SMTPHost,
			Port: cfg.SMTPPort,
			User: cfg.SMTPUser,
			Pass: cfg.SMTPPass,
			From: cfg.MailFrom,
		}
		log.Printf("email enabled via SMTP %s:%s", cfg.SMTPHost, cfg.SMTPPort)
	default:
		log.Println("no email provider configured, emails will only be logged")
		mailer = notification.LogSender{}
	}

	shippingProvider, err := buildShippingProvider(cfg)
	if err != nil {
		log.Fatalf("shipping configuration failed: %v", err)
	}

	productHandler := handler.NewProductHandler(productRepo)
	orderHandler := handler.NewOrderHandler(productRepo, orderRepo, paymentGateway, mailer, shippingProvider, cfg.StoreBaseURL)
	orderHandler.SetStoreOrigin(cfg.StoreOriginLat, cfg.StoreOriginLon, cfg.StoreOriginAddress)
	// Enables Midtrans signature verification on the payment webhook.
	orderHandler.SetPaymentServerKey(cfg.MidtransServerKey)
	shippingHandler := handler.NewShippingHandler(shippingProvider, productRepo)
	storeConfigHandler := handler.NewStoreConfigHandler(cfg.MidtransClientKey, cfg.MidtransEnv)

	// Admin sessions live in memory and are revoked on restart.
	adminSessions := session.NewStore(time.Duration(cfg.AdminSessionTTLMinutes) * time.Minute)
	go adminSessions.RunCleanup(ctx, 10*time.Minute)
	adminAuthHandler := handler.NewAdminAuthHandler(cfg, adminSessions)
	if cfg.AdminEmail == "" || cfg.AdminPasswordHash == "" {
		log.Println("admin sign-in is disabled: set ADMIN_EMAIL and ADMIN_PASSWORD_HASH")
	}

	// Expire unpaid orders so their reserved stock returns to the catalog even
	// when the payment webhook never arrives.
	orderSweeper := sweeper.New(orderRepo, productRepo, orderHandler,
		time.Duration(cfg.OrderPendingTTLMinutes)*time.Minute,
		time.Duration(cfg.OrderSweepIntervalSeconds)*time.Second)
	go orderSweeper.Run(ctx)

	// Email a digest of orders that need a human: refunds to issue and paid
	// orders whose shipment was never created. Silence means everything is fine.
	if cfg.OpsEmail != "" {
		notifier := alerts.New(orderRepo, mailer, cfg.OpsEmail,
			time.Duration(cfg.OpsAlertIntervalHours)*time.Hour,
			time.Duration(cfg.OpsAlertGraceMinutes)*time.Minute)
		go notifier.Run(ctx)
	} else {
		log.Println("ops alerts disabled: set OPS_EMAIL to receive the refund/shipment digest")
	}

	// Create a new Fiber instance
	fiberConfig := fiber.Config{}
	if cfg.TrustProxy {
		// nginx runs on the same host, so only loopback may claim a client IP.
		// Trusting X-Forwarded-For from anywhere else would let a caller spoof
		// the address used by the admin sign-in limiter.
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedFor
		fiberConfig.EnableTrustedProxyCheck = true
		fiberConfig.TrustedProxies = []string{"127.0.0.1", "::1"}
		fiberConfig.EnableIPValidation = true
	}
	app := fiber.New(fiberConfig)

	// Recover panics so one bad request cannot take the process down.
	app.Use(recover.New())

	// CORS: restrict to configured origins in production
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Join(cfg.AllowedOrigins(), ","),
	}))

	// Setup routes
	routes.SetupRoutes(app, productHandler, orderHandler, shippingHandler, storeConfigHandler, adminAuthHandler, adminSessions, cfg.RateLimitEnabled)

	// Start the server, then wait for a shutdown signal.
	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Printf("server stopped: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Println("shutting down server...")
	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// buildShippingProvider picks the provider from config, so switching between
// providers (flat, biteship, disabled) never touches the handlers.
func buildShippingProvider(cfg config.Config) (shipping.Provider, error) {
	mode := cfg.ShippingProvider
	if mode == "" {
		// auto: Biteship when a key exists, flat rates when configured, else disabled
		switch {
		case cfg.BiteshipAPIKey != "":
			mode = "biteship"
		case cfg.ShippingFlatRates != "":
			mode = "flat"
		default:
			mode = "disabled"
		}
	}

	switch mode {
	case "biteship":
		if cfg.BiteshipAPIKey == "" {
			return nil, fmt.Errorf("SHIPPING_PROVIDER=biteship requires BITESHIP_API_KEY")
		}
		log.Printf("shipping enabled via Biteship (origin postal code %q)", cfg.StoreOriginPostalCode)
		return shipping.NewBiteship(cfg.BiteshipAPIKey, cfg.BiteshipBaseURL, shipping.Origin{
			PostalCode:       cfg.StoreOriginPostalCode,
			Lat:              cfg.StoreOriginLat,
			Lon:              cfg.StoreOriginLon,
			ContactName:      cfg.StoreOriginContact,
			ContactPhone:     cfg.StoreOriginPhone,
			Address:          cfg.StoreOriginAddress,
			CollectionMethod: cfg.StoreCollectionMethod,
		}), nil

	case "flat":
		provider, err := shipping.NewFlatRate(cfg.ShippingFlatRates)
		if err != nil {
			return nil, err
		}
		log.Println("shipping enabled via flat rate (development provider)")
		return provider, nil

	default:
		log.Println("shipping is disabled")
		return shipping.Disabled{}, nil
	}
}
