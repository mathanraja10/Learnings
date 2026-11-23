package domain

import "context"

type IdempotencyRepository interface {
	Get(
		ctx context.Context,
		tenantID string,
		key string,
	) (*IdempotencyRecord, error)

	Create(
		ctx context.Context,
		record IdempotencyRecord,
	) error

	Complete(
		ctx context.Context,
		tenantID string,
		key string,
		status int,
		body []byte,
	) error
}
