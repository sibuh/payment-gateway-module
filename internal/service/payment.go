package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"pgm/internal/domain"
	"pgm/internal/repo/db"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PaymentService struct {
	queries   *db.Queries
	publisher domain.MessagePublisher
}

func NewPaymentService(q *db.Queries, publisher domain.MessagePublisher) domain.PaymentService {
	return &PaymentService{
		queries:   q,
		publisher: publisher,
	}
}

func (u *PaymentService) CreatePayment(ctx context.Context, p *domain.PaymentRequest) (*domain.Payment, error) {
	// Check if reference already exists

	existing, err := u.queries.GetPaymentByReference(ctx, p.Reference)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Error{
				Code:        500,
				Message:     "Failed to check for existing payment",
				Description: err.Error(),
				Err:         err,
			}
		}
	}
	if existing.ID != uuid.Nil {
		return nil, domain.Error{
			Code:        409,
			Message:     "Payment with this reference already exists",
			Description: "A payment with the same reference has already been created",
		}
	}

	payment, err := u.queries.CreatePayment(ctx, db.CreatePaymentParams{
		Amount:    decimal.NewFromFloat(p.Amount),
		Currency:  p.Currency,
		Reference: p.Reference,
	})
	if err != nil {
		return nil, domain.Error{
			Code:        500,
			Message:     "Failed to create payment",
			Description: err.Error(),
			Err:         err,
		}
	}

	// Publish to RabbitMQ
	err = u.publisher.PublishPaymentCreated(ctx, payment.ID.String())
	if err != nil {
		// In a real-world scenario, we might want to use an outbox pattern here
		// to ensure the message is eventually published.
		fmt.Printf("failed to publish message: %v\n", err)
	}

	return &domain.Payment{
		ID:        payment.ID,
		Amount:    payment.Amount.InexactFloat64(),
		Currency:  payment.Currency,
		Reference: payment.Reference,
		Status:    domain.PaymentStatus(payment.Status),
		CreatedAt: payment.CreatedAt.Time,
		UpdatedAt: payment.UpdatedAt.Time,
	}, nil
}

func (u *PaymentService) GetPaymentByID(ctx context.Context, id string) (*domain.Payment, error) {
	// parse payment id
	paymentID, err := uuid.Parse(id)
	if err != nil {
		return nil, domain.Error{
			Code:        400,
			Message:     "Invalid payment ID format",
			Description: "The provided payment ID is not a valid UUID format",
			Err:         err,
		}
	}

	payment, err := u.queries.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, domain.Error{
			Code:        500,
			Message:     "Failed to fetch payment",
			Description: "Error occurred while retrieving payment information",
			Err:         err,
		}
	}

	return &domain.Payment{
		ID:        payment.ID,
		Amount:    payment.Amount.InexactFloat64(),
		Currency:  payment.Currency,
		Reference: payment.Reference,
		Status:    domain.PaymentStatus(payment.Status),
		CreatedAt: payment.CreatedAt.Time,
		UpdatedAt: payment.UpdatedAt.Time,
	}, nil
}

func (u *PaymentService) ProcessPayment(ctx context.Context, id string) error {
	// Parse payment id
	paymentID, err := uuid.Parse(id)
	if err != nil {
		return domain.Error{
			Code:        400,
			Message:     "Invalid payment ID format",
			Description: "The provided payment ID is not a valid UUID format",
			Err:         err,
		}
	}

	// Use row-level locking to prevent race conditions
	p, err := u.queries.GetPaymentByIDWithLock(ctx, paymentID)
	if err != nil {
		return domain.Error{
			Code:        500,
			Message:     "Failed to fetch payment",
			Description: "Error occurred while retrieving payment information",
			Err:         err,
		}
	}
	if p.ID == uuid.Nil {
		return domain.Error{
			Code:        404,
			Message:     "Payment not found",
			Description: "The specified payment could not be found",
		}
	}

	// Idempotency check: only process if PENDING
	if string(p.Status) != string(domain.StatusPending) {
		fmt.Printf("payment %s already processed with status %s\n", id, p.Status)
		return nil
	}

	// Simulate processing
	time.Sleep(2 * time.Second)

	newStatus := domain.StatusSuccess
	if rand.Float32() < 0.3 { // 30% failure rate for simulation
		newStatus = domain.StatusFailed
	}

	_, err = u.queries.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
		ID:     uuid.MustParse(id),
		Status: db.Paymentstatus(newStatus),
	})
	if err != nil {
		return domain.Error{
			Code:        500,
			Message:     "Failed to update payment status",
			Description: "Error occurred while updating payment status in the database",
			Err:         err,
		}
	}

	fmt.Printf("payment %s processed with status %s\n", id, newStatus)
	return nil
}
