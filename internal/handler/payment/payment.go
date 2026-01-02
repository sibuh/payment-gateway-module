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
	var pr domain.PaymentRequest
	if err := c.Bind(&pr); err != nil {
		return domain.Error{
			Code:        http.StatusBadRequest,
			Message:     "invalid request body",
			Description: "failed to bind request body",
			Err:         err,
		}
	}

	if err := pr.Validate(); err != nil {
		return domain.Error{
			Code:        http.StatusBadRequest,
			Message:     "validation failed",
			Description: "payment request validation failed",
			Args:        map[string]interface{}{"req": pr},
			Err:         err,
		}
	}

	res, err := h.svc.CreatePayment(c.Request().Context(), &pr)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, res)
}

func (h *PaymentHandler) GetPaymentByID(c echo.Context) error {
	id := c.Param("id")
	res, err := h.svc.GetPaymentByID(c.Request().Context(), id)
	if err != nil {
		return err
	}

	if res == nil {
		return domain.Error{
			Code:        http.StatusNotFound,
			Message:     "payment not found",
			Description: "payment not found",
			Args:        map[string]interface{}{"id": id},
			Err:         nil,
		}
	}

	return c.JSON(http.StatusOK, res)
}
