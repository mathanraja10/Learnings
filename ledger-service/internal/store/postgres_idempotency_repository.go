package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ledger-service/internal/domain"
)

type PostgresIdempotencyRepository struct {
	store *PostgresStore
}

func NewPostgresIdempotencyRepository(
	store *PostgresStore,
) *PostgresIdempotencyRepository {
	return &PostgresIdempotencyRepository{
		store: store,
	}
}

func (r *PostgresIdempotencyRepository) Get(
	ctx context.Context,
	tenantID string,
	key string,
) (*domain.IdempotencyRecord, error) {

	const query = `
		SELECT
			tenant_id,
			idempotency_key,
			request_hash,
			status,
			COALESCE(response_status, 0),
			COALESCE(response_body, '{}'::jsonb)
		FROM idempotency_keys
		WHERE tenant_id = $1
		  AND idempotency_key = $2
	`

	var record domain.IdempotencyRecord

	err := r.store.DB.QueryRow(
		ctx,
		query,
		tenantID,
		key,
	).Scan(
		&record.TenantID,
		&record.Key,
		&record.RequestHash,
		&record.Status,
		&record.ResponseStatus,
		&record.ResponseBody,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf(
			"get idempotency record: %w",
			err,
		)
	}

	return &record, nil
}

func (r *PostgresIdempotencyRepository) Create(
	ctx context.Context,
	record domain.IdempotencyRecord,
) error {

	const query = `
		INSERT INTO idempotency_keys (
			tenant_id,
			idempotency_key,
			request_hash,
			status
		)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.store.DB.Exec(
		ctx,
		query,
		record.TenantID,
		record.Key,
		record.RequestHash,
		record.Status,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("duplicate idempotency key: %w", domain.ErrConflict)
		}

		return fmt.Errorf(
			"create idempotency record: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresIdempotencyRepository) Complete(
	ctx context.Context,
	tenantID string,
	key string,
	status int,
	body []byte,
) error {

	const query = `
		UPDATE idempotency_keys
		SET status = 'completed',
		    response_status = $1,
		    response_body = $2
		WHERE tenant_id = $3
		  AND idempotency_key = $4
	`

	_, err := r.store.DB.Exec(
		ctx,
		query,
		status,
		body,
		tenantID,
		key,
	)

	if err != nil {
		return fmt.Errorf(
			"complete idempotency record: %w",
			err,
		)
	}

	return nil
}
