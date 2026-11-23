package store

import (
	"context"
	"fmt"
	"sync"

	"ledger-service/internal/domain"
)

type MemoryIdempotencyRepository struct {
	mu      sync.RWMutex
	records map[string]domain.IdempotencyRecord
}

func NewMemoryIdempotencyRepository() *MemoryIdempotencyRepository {
	return &MemoryIdempotencyRepository{
		records: make(map[string]domain.IdempotencyRecord),
	}
}

func (r *MemoryIdempotencyRepository) makeKey(tenantID, key string) string {
	return tenantID + ":" + key
}

func (r *MemoryIdempotencyRepository) Get(
	ctx context.Context,
	tenantID string,
	key string,
) (*domain.IdempotencyRecord, error) {

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.records[r.makeKey(tenantID, key)]
	if !ok {
		return nil, nil
	}

	copyRec := rec
	return &copyRec, nil
}

func (r *MemoryIdempotencyRepository) Create(
	ctx context.Context,
	record domain.IdempotencyRecord,
) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	compositeKey := r.makeKey(record.TenantID, record.Key)
	if _, exists := r.records[compositeKey]; exists {
		return fmt.Errorf("duplicate idempotency key: %w", domain.ErrConflict)
	}

	r.records[compositeKey] = record
	return nil
}

func (r *MemoryIdempotencyRepository) Complete(
	ctx context.Context,
	tenantID string,
	key string,
	status int,
	body []byte,
) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	compositeKey := r.makeKey(tenantID, key)
	rec, ok := r.records[compositeKey]
	if !ok {
		return fmt.Errorf("idempotency record not found: %w", domain.ErrNotFound)
	}

	rec.Status = "completed"
	rec.ResponseStatus = status
	rec.ResponseBody = body
	r.records[compositeKey] = rec

	return nil
}
