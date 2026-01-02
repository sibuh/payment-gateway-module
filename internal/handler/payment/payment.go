package http

import (
	"net/http"

	"pgm/internal/domain"

	"github.com/labstack/echo/v4"
)

type PaymentHandler struct {
	svc domain.PaymentService
}

func NewPaymentHandler(g *echo.Group, uc domain.PaymentService) {
	handler := &PaymentHandler{
		svc: uc,
	}
	g.POST("/payments", handler.CreatePayment)
	g.GET("/payments/:id", handler.GetPaymentByID)
}

func (h *PaymentHandler) CreatePayment(c echo.Context) error {
	var p domain.Payment
	if err := c.Bind(&p); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if err := c.Validate(&p); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	res, err := h.svc.CreatePayment(c.Request().Context(), &p)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, res)
}

func (h *PaymentHandler) GetPaymentByID(c echo.Context) error {
	id := c.Param("id")
	res, err := h.svc.GetPaymentByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if res == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "payment not found"})
	}

	return c.JSON(http.StatusOK, res)
}
