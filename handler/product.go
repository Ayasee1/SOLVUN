package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"golearn/model"
	"golearn/repository"
)

const (
	maxProductImages = 8
	maxImageBytes    = 5 << 20
)

type ProductHandler struct {
	repo repository.ProductRepository
	// mediaDir is where uploaded product images are written. Empty disables
	// uploads.
	mediaDir string
}

func NewProductHandler(repo repository.ProductRepository) *ProductHandler {
	return &ProductHandler{repo: repo}
}

// SetMediaDir enables image uploads into dir/products.
func (h *ProductHandler) SetMediaDir(dir string) {
	h.mediaDir = dir
}

func (h *ProductHandler) GetProducts(c *fiber.Ctx) error {
	name := c.Query("name")

	all, err := h.repo.GetAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load products",
		})
	}

	var products []model.Product
	for _, product := range all {
		if !product.IsAvailable {
			continue
		}
		if name == "" || strings.Contains(strings.ToLower(product.Name), strings.ToLower(name)) {
			products = append(products, product)
		}
	}

	if len(products) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No products found",
		})
	}

	return c.JSON(products)
}

// GetAllProducts lists every product (admin), including hidden ones.
func (h *ProductHandler) GetAllProducts(c *fiber.Ctx) error {
	products, err := h.repo.GetAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load products",
		})
	}

	return c.JSON(products)
}

func (h *ProductHandler) GetProductByID(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid ID",
		})
	}

	product, err := h.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Product not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load product",
		})
	}

	return c.JSON(product)
}

// cart handler
func (h *ProductHandler) AddToCart(c *fiber.Ctx) error {
	var item model.Item

	if err := c.BodyParser(&item); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	item.TotalPrice = item.Price * item.Qty

	return c.Status(fiber.StatusCreated).JSON(item)
}

// delete handler
// CreateProduct adds a product from the dashboard. Content and runtime fields
// are both stored in the database now that it owns the catalog.
func (h *ProductHandler) CreateProduct(c *fiber.Ctx) error {
	var input model.ProductInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	normalizeProductInput(&input)
	if err := validateProductInput(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	product, err := h.repo.Create(input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create product",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(product)
}

// UploadProductImage stores one image and returns the URL the storefront can
// use. Files are written outside the web build, so deploys never touch them.
func (h *ProductHandler) UploadProductImage(c *fiber.Ctx) error {
	if h.mediaDir == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Image uploads are not configured",
		})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "A file is required",
		})
	}
	if file.Size <= 0 || file.Size > maxImageBytes {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Image must be at most 5 MB",
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not read the upload",
		})
	}
	defer src.Close()

	header := make([]byte, 512)
	n, _ := io.ReadFull(src, header)
	extension := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}[http.DetectContentType(header[:n])]
	if extension == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Only JPEG, PNG or WebP images are accepted",
		})
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not read the upload",
		})
	}

	dir := filepath.Join(h.mediaDir, "products")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not store the image",
		})
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not store the image",
		})
	}
	name := hex.EncodeToString(random) + extension

	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not store the image",
		})
	}
	tmpName := tmp.Name()

	// nginx reads the file as a different user than the API process.
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not store the image",
		})
	}

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not store the image",
		})
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not store the image",
		})
	}
	if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		os.Remove(tmpName)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not store the image",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"url": "/media/products/" + name,
	})
}

func normalizeProductInput(input *model.ProductInput) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	for i := range input.Images {
		input.Images[i] = strings.TrimSpace(input.Images[i])
	}
}

func validateProductInput(input model.ProductInput) error {
	if len(input.Name) == 0 || len(input.Name) > 200 {
		return errors.New("name must be 1-200 characters")
	}
	if len(input.Description) > 5000 {
		return errors.New("description must be at most 5000 characters")
	}
	if input.Price < 0 || input.Price > 1_000_000_000 {
		return errors.New("price must be between 0 and 1,000,000,000")
	}
	if input.Stock < 0 || input.Stock > 1_000_000 {
		return errors.New("stock must be between 0 and 1,000,000")
	}
	if input.WeightGram < 1 || input.WeightGram > 100_000 {
		return errors.New("weight must be between 1 and 100,000 grams")
	}
	if len(input.Images) > maxProductImages {
		return fmt.Errorf("at most %d images", maxProductImages)
	}
	for _, image := range input.Images {
		if !validImageURL(image) {
			return errors.New("each image must be a /path or an http(s) URL")
		}
	}
	return nil
}

func validImageURL(value string) bool {
	if value == "" || len(value) > 500 {
		return false
	}
	return strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://")
}

func (h *ProductHandler) DeleteProduct(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid ID",
		})
	}

	product, err := h.repo.Delete(id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Product not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete product",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Product deleted successfully",
		"product": product,
	})
}

// update handler (admin): partial update of a product
func (h *ProductHandler) UpdateProduct(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid ID",
		})
	}

	var update model.ProductUpdate
	if err := c.BodyParser(&update); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if update.Name != nil {
		trimmed := strings.TrimSpace(*update.Name)
		if trimmed == "" || len(trimmed) > 200 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "name must be 1-200 characters",
			})
		}
		update.Name = &trimmed
	}
	if update.Description != nil {
		trimmed := strings.TrimSpace(*update.Description)
		if len(trimmed) > 5000 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "description must be at most 5000 characters",
			})
		}
		update.Description = &trimmed
	}
	if update.Images != nil {
		if len(*update.Images) > maxProductImages {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("at most %d images", maxProductImages),
			})
		}
		images := make([]string, len(*update.Images))
		for i, image := range *update.Images {
			image = strings.TrimSpace(image)
			if !validImageURL(image) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "each image must be a /path or an http(s) URL",
				})
			}
			images[i] = image
		}
		update.Images = &images
	}
	if update.Price != nil && (*update.Price < 0 || *update.Price > 1_000_000_000) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "price must be between 0 and 1,000,000,000",
		})
	}
	if update.Stock != nil && (*update.Stock < 0 || *update.Stock > 1_000_000) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "stock must be between 0 and 1,000,000",
		})
	}
	if update.WeightGram != nil && (*update.WeightGram < 1 || *update.WeightGram > 100_000) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "weight must be between 1 and 100,000 grams",
		})
	}

	product, err := h.repo.UpdateProduct(id, update)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Product not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	return c.JSON(product)
}
