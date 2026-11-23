package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"ledger-service/internal/domain"
)

type TransferResult struct {
	StatusCode int
	Body       []byte
}

type PostgresTransferService struct {
	store *PostgresStore
}

func NewPostgresTransferService(
	store *PostgresStore,
) *PostgresTransferService {
	return &PostgresTransferService{
		store: store,
	}
}

func (s *PostgresTransferService) Transfer(
	ctx context.Context,
	tenantID string,
	idempotencyKey string,
	req domain.TransferRequest,
) (TransferResult, error) {

	requestHash, err := domain.HashRequest(req)
	if err != nil {
		return TransferResult{}, fmt.Errorf(
			"hash transfer request: %w",
			err,
		)
	}

	tx, err := s.store.DB.Begin(ctx)
	if err != nil {
		return TransferResult{}, fmt.Errorf(
			"begin transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Lock the idempotency row if it already exists.
	var existingHash string
	var existingStatus string
	var existingResponseStatus int
	var existingResponseBody []byte

	err = tx.QueryRow(
		ctx,
		`
		SELECT
			request_hash,
			status,
			COALESCE(response_status, 0),
			COALESCE(response_body, '{}'::jsonb)
		FROM idempotency_keys
		WHERE tenant_id = $1
		  AND idempotency_key = $2
		FOR UPDATE
		`,
		tenantID,
		idempotencyKey,
	).Scan(
		&existingHash,
		&existingStatus,
		&existingResponseStatus,
		&existingResponseBody,
	)

	if err == nil {

		if existingHash != requestHash {
			return TransferResult{}, domain.ErrConflict
		}

		if existingStatus == "completed" {
			return TransferResult{
				StatusCode: existingResponseStatus,
				Body:       existingResponseBody,
			}, nil
		}

	} else if !errors.Is(err, pgx.ErrNoRows) {
		return TransferResult{}, fmt.Errorf(
			"check idempotency key: %w",
			err,
		)
	} else {
		// First request.
		_, err = tx.Exec(
			ctx,
			`
			INSERT INTO idempotency_keys (
				tenant_id,
				idempotency_key,
				request_hash,
				status
			)
			VALUES ($1, $2, $3, 'processing')
			`,
			tenantID,
			idempotencyKey,
			requestHash,
		)

		if err != nil {
			return TransferResult{}, fmt.Errorf(
				"create idempotency key: %w",
				err,
			)
		}
	}

	// Lock source account.
	from, err := lockAccount(
		ctx,
		tx,
		req.FromAccountID,
	)
	if err != nil {
		return TransferResult{}, err
	}

	// Lock destination account.
	to, err := lockAccount(
		ctx,
		tx,
		req.ToAccountID,
	)
	if err != nil {
		return TransferResult{}, err
	}

	if from.Balance < req.Amount {
		return TransferResult{}, domain.ErrInsufficientFunds
	}

	_, err = tx.Exec(
		ctx,
		`
		UPDATE accounts
		SET balance = balance - $1,
		    updated_at = NOW()
		WHERE id = $2
		`,
		req.Amount,
		from.ID,
	)
	if err != nil {
		return TransferResult{}, fmt.Errorf(
			"debit account: %w",
			err,
		)
	}

	_, err = tx.Exec(
		ctx,
		`
		UPDATE accounts
		SET balance = balance + $1,
		    updated_at = NOW()
		WHERE id = $2
		`,
		req.Amount,
		to.ID,
	)
	if err != nil {
		return TransferResult{}, fmt.Errorf(
			"credit account: %w",
			err,
		)
	}

	response := map[string]any{
		"status":  "completed",
		"message": "transfer completed",
	}

	body, err := json.Marshal(response)
	if err != nil {
		return TransferResult{}, fmt.Errorf(
			"marshal response: %w",
			err,
		)
	}

	_, err = tx.Exec(
		ctx,
		`
		UPDATE idempotency_keys
		SET status = 'completed',
		    response_status = 201,
		    response_body = $1
		WHERE tenant_id = $2
		  AND idempotency_key = $3
		`,
		body,
		tenantID,
		idempotencyKey,
	)
	if err != nil {
		return TransferResult{}, fmt.Errorf(
			"complete idempotency key: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return TransferResult{}, fmt.Errorf(
			"commit transfer: %w",
			err,
		)
	}

	return TransferResult{
		StatusCode: 201,
		Body:       body,
	}, nil
}
