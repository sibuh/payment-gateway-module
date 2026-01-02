package domain

import (
	"context"
	"fmt"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type PaymentStatus string

const (
	StatusPending PaymentStatus = "PENDING"
	StatusSuccess PaymentStatus = "SUCCESS"
	StatusFailed  PaymentStatus = "FAILED"
)

type Payment struct {
	ID        uuid.UUID     `json:"id"`
	Amount    float64       `json:"amount" validate:"required,gt=0"`
	Currency  string        `json:"currency" validate:"required,oneof=ETB USD"`
	Reference string        `json:"reference" validate:"required"`
	Status    PaymentStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}
type PaymentRequest struct {
	Amount    float64 `json:"amount" validate:"required,gt=0"`
	Currency  string  `json:"currency" validate:"required,oneof=ETB USD"`
	Reference string  `json:"reference" validate:"required"`
}

func (pr PaymentRequest) Validate() error {
	return validation.ValidateStruct(&pr,
		validation.Field(&pr.Amount, validation.Required.Error("payment amount is required"), validation.Min(0.01)),
		validation.Field(&pr.Currency, validation.Required.Error("currency is required"), validation.In("ETB", "USD")),
		validation.Field(&pr.Reference, validation.Required.Error("payment reference is required")))
}

type PaymentRepo interface {
	CreatePayment(ctx context.Context, payment *Payment) error
	GetPaymentByID(ctx context.Context, id string) (*Payment, error)
	GetPaymentByReference(ctx context.Context, reference string) (*Payment, error)
	UpdatePaymentStatus(ctx context.Context, id string, status PaymentStatus) error
	// For row-level locking and idempotency
	GetPaymentByIDWithLock(ctx context.Context, id string) (*Payment, error)
}

type PaymentService interface {
	CreatePayment(ctx context.Context, pr *PaymentRequest) (*Payment, error)
	GetPaymentByID(ctx context.Context, id string) (*Payment, error)
	ProcessPayment(ctx context.Context, id string) error
}

type MessagePublisher interface {
	PublishPaymentCreated(ctx context.Context, paymentID string) error
}

type Error struct {
	Code        int                    `json:"code"`
	Message     string                 `json:"message"`
	Description string                 `json:"description"`
	Args        map[string]interface{} `json:"params"`
	Err         error                  `json:"err"`
}

func (e Error) Error() string {
	return fmt.Sprintf("Message:%d: Description:%s Cause:%s", e.Code, e.Message, e.Err.Error())
}

func (e Error) ErrorCode() int {
	return e.Code
}

func (e Error) ErrorArgs() map[string]interface{} {
	return e.Args
}

func (e Error) Unwrap() error {
	return e.Err
}

type ErrorResponse struct {
	Code        int            `json:"code"`
	Message     string         `json:"message"`
	Description string         `json:"description,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

func ErrorHandler(err error, c echo.Context) {
	var (
		code        = http.StatusInternalServerError
		message     = "internal server error"
		description = ""
		params      map[string]interface{}
		internalErr error
	)

	switch e := err.(type) {

	// Your custom error
	case Error:
		code = e.Code
		message = e.Message
		description = e.Description
		params = e.Args
		internalErr = e.Err

	// Echo HTTP error
	case *echo.HTTPError:
		code = e.Code
		message = fmt.Sprint(e.Message)
		internalErr = e.Internal

	// Context timeout / cancel
	// case context.DeadlineExceeded:
	// 	code = http.StatusRequestTimeout
	// 	message = "request timeout"
	// 	internalErr = err

	default:
		internalErr = err
	}

	//LOG INTERNAL ERROR
	if internalErr != nil {
		c.Logger().Errorf(
			"error=%v code=%d message=%s params=%v",
			internalErr,
			code,
			message,
			params,
		)
	} else {
		c.Logger().Errorf(
			"error=%v code=%d message=%s params=%v",
			err,
			code,
			message,
			params,
		)
	}

	// Response already sent?
	if c.Response().Committed {
		return
	}

	// Write safe response
	_ = c.JSON(code, ErrorResponse{
		Code:        code,
		Message:     message,
		Description: description,
		Params:      params,
	})
}
