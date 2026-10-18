package handler

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"golearn/model"
	"golearn/repository"
)

type ProductHandler struct {
	repo repository.ProductRepository
}

func NewProductHandler(repo repository.ProductRepository) *ProductHandler {
	return &ProductHandler{repo: repo}
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
