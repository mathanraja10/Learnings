package store

import (
	"context"
	"errors"
	"testing"

	"ledger-service/internal/domain"
)

func TestMemoryIdempotencyRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryIdempotencyRepository()

	// 1. Get non-existent
	rec, err := repo.Get(ctx, "tenant_1", "key_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec != nil {
		t.Fatalf("expected nil record, got %+v", rec)
	}

	// 2. Create
	newRec := domain.IdempotencyRecord{
		TenantID:    "tenant_1",
		Key:         "key_1",
		RequestHash: "hash123",
		Status:      "processing",
	}
	if err := repo.Create(ctx, newRec); err != nil {
		t.Fatalf("expected create success, got %v", err)
	}

	// 3. Create duplicate key -> ErrConflict
	if err := repo.Create(ctx, newRec); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate create, got %v", err)
	}

	// 4. Get created
	found, err := repo.Get(ctx, "tenant_1", "key_1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if found.Status != "processing" || found.RequestHash != "hash123" {
		t.Fatalf("unexpected found record: %+v", found)
	}

	// 5. Complete
	if err := repo.Complete(ctx, "tenant_1", "key_1", 200, []byte(`{"status":"ok"}`)); err != nil {
		t.Fatalf("expected complete success, got %v", err)
	}

	completed, err := repo.Get(ctx, "tenant_1", "key_1")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if completed.Status != "completed" || completed.ResponseStatus != 200 {
		t.Fatalf("unexpected completed record: %+v", completed)
	}

	// 6. Context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.Get(cancelCtx, "tenant_1", "key_1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
