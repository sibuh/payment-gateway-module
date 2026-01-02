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

	exists, err := u.queries.CheckExistence(ctx, p.Reference)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewError(
				500,
				"Failed to check for existing payment",
				"An unexpected error occurred while checking for existing payment", err,
				map[string]interface{}{"PaymentRequest": p})
		}
	}
	if exists {
		return nil, domain.NewError(
			409,
			"Payment with this reference already exists",
			"A payment with the same reference has already been created",
			fmt.Errorf("payment with reference %s already exists", p.Reference),
			map[string]interface{}{"PaymentRequest": p},
		)
	}

	payment, err := u.queries.CreatePayment(ctx, db.CreatePaymentParams{
		Amount:    decimal.NewFromFloat(p.Amount),
		Currency:  p.Currency,
		Reference: p.Reference,
	})
	if err != nil {
		return nil, domain.NewError(
			500,
			"Failed to create payment",
			err.Error(),
			err,
			map[string]interface{}{"PaymentRequest": p},
		)
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
		return nil, domain.NewError(
			400,
			"Invalid payment ID format",
			"The provided payment ID is not a valid UUID format",
			err,
			map[string]interface{}{"PaymentID": id},
		)
	}

	payment, err := u.queries.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, domain.NewError(
			500,
			"Failed to fetch payment",
			"Error occurred while retrieving payment information",
			err,
			map[string]interface{}{"PaymentID": id},
		)
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
		return domain.NewError(
			400,
			"Invalid payment ID format",
			"The provided payment ID is not a valid UUID format",
			err,
			map[string]interface{}{"PaymentID": id},
		)
	}

	// Use row-level locking to prevent race conditions
	p, err := u.queries.GetPaymentByIDWithLock(ctx, paymentID)
	if err != nil {
		return domain.NewError(
			500,
			"Failed to fetch payment",
			"Error occurred while retrieving payment information",
			err,
			map[string]interface{}{"PaymentID": id},
		)
	}
	if p.ID == uuid.Nil {
		return domain.NewError(
			404,
			"Payment not found",
			"The specified payment could not be found",
			nil,
			map[string]interface{}{"PaymentID": id},
		)
	}

	// Idempotency check: only process if PENDING
	if string(p.Status) != string(domain.StatusPending) {
		fmt.Printf("payment %s already processed with status %s\n", id, p.Status)
		return domain.NewError(
			409,
			"Payment already processed",
			"This payment has already been processed with status "+string(p.Status),
			nil,
			map[string]interface{}{"status": p.Status},
		)
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
		return domain.NewError(
			500,
			"Failed to update payment status",
			"Error occurred while updating payment status in the database",
			err,
			map[string]interface{}{"PaymentID": id, "NewStatus": newStatus, "OldStatus": p.Status, "Payment": p},
		)
	}

	fmt.Printf("payment %s processed with status %s\n", id, newStatus)
	return nil
}
