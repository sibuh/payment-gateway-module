-- name: CreatePayment :one
INSERT INTO payments (amount, currency, reference)
		VALUES ($1, $2, $3)
		RETURNING *;
-- name: GetPaymentByID :one
SELECT id, amount, currency, reference, status, created_at, updated_at FROM payments WHERE id = $1;

-- name: GetPaymentByReference :one
SELECT id, amount, currency, reference, status, created_at, updated_at FROM payments WHERE reference = $1;
-- name: GetPaymentByIDWithLock :one
SELECT id, amount, currency, reference, status, created_at, updated_at FROM payments WHERE id = $1 FOR UPDATE;
-- name: UpdatePaymentStatus :one
UPDATE payments SET status = $1, updated_at = $2 WHERE id = $3 RETURNING *;
