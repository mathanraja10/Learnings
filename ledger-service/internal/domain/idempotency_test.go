package domain

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type inMemoryIdempotencyRepo struct {
	mu      sync.Mutex
	records map[string]IdempotencyRecord
}

func newInMemoryIdempotencyRepo() *inMemoryIdempotencyRepo {
	return &inMemoryIdempotencyRepo{
		records: make(map[string]IdempotencyRecord),
	}
}

func (r *inMemoryIdempotencyRepo) Get(ctx context.Context, tenantID, key string) (*IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[tenantID+":"+key]
	if !ok {
		return nil, nil
	}
	cp := rec
	return &cp, nil
}

func (r *inMemoryIdempotencyRepo) Create(ctx context.Context, record IdempotencyRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := record.TenantID + ":" + record.Key
	if _, exists := r.records[k]; exists {
		return ErrConflict
	}
	r.records[k] = record
	return nil
}

func (r *inMemoryIdempotencyRepo) Complete(ctx context.Context, tenantID, key string, status int, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := tenantID + ":" + key
	rec, ok := r.records[k]
	if !ok {
		return ErrNotFound
	}
	rec.Status = "completed"
	rec.ResponseStatus = status
	rec.ResponseBody = body
	r.records[k] = rec
	return nil
}

func TestHashRequest(t *testing.T) {
	req1 := map[string]any{"amount": 1000, "currency": "INR"}
	req2 := map[string]any{"amount": 1000, "currency": "INR"}
	req3 := map[string]any{"amount": 2000, "currency": "INR"}

	hash1, err := HashRequest(req1)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	hash2, err := HashRequest(req2)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	hash3, err := HashRequest(req3)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("expected identical hashes for identical payloads, got %s and %s", hash1, hash2)
	}
	if hash1 == hash3 {
		t.Fatalf("expected different hashes for different payloads")
	}
}

func TestIdempotentTransferReplay(t *testing.T) {
	transferRepo := newMockTransferRepository(map[string]Account{
		"acc_1": {ID: "acc_1", Balance: 5000},
		"acc_2": {ID: "acc_2", Balance: 1000},
	})
	idemRepo := newInMemoryIdempotencyRepo()

	service := NewTransferService(transferRepo).WithIdempotency(idemRepo)

	req := TransferRequest{
		TenantID:       "tenant_1",
		IdempotencyKey: "key_123",
		FromAccountID:  "acc_1",
		ToAccountID:    "acc_2",
		Amount:         1000,
	}

	// 1st request -> executes transfer
	err := service.Transfer(context.Background(), req)
	if err != nil {
		t.Fatalf("expected 1st transfer success, got %v", err)
	}
	if transferRepo.accounts["acc_1"].Balance != 4000 {
		t.Fatalf("expected balance 4000 after 1st transfer, got %d", transferRepo.accounts["acc_1"].Balance)
	}

	// 2nd request with SAME idempotency key -> returns success (replay), NO SECOND DEBIT!
	err = service.Transfer(context.Background(), req)
	if err != nil {
		t.Fatalf("expected 2nd transfer success (idempotent replay), got %v", err)
	}
	if transferRepo.accounts["acc_1"].Balance != 4000 {
		t.Fatalf("balance was debited twice! Expected 4000, got %d", transferRepo.accounts["acc_1"].Balance)
	}
}

func TestIdempotentTransferDifferentPayloadConflict(t *testing.T) {
	transferRepo := newMockTransferRepository(map[string]Account{
		"acc_1": {ID: "acc_1", Balance: 5000},
		"acc_2": {ID: "acc_2", Balance: 1000},
	})
	idemRepo := newInMemoryIdempotencyRepo()

	service := NewTransferService(transferRepo).WithIdempotency(idemRepo)

	// First request: transfer 1000
	req1 := TransferRequest{
		TenantID:       "tenant_1",
		IdempotencyKey: "key_xyz",
		FromAccountID:  "acc_1",
		ToAccountID:    "acc_2",
		Amount:         1000,
	}
	if err := service.Transfer(context.Background(), req1); err != nil {
		t.Fatalf("expected req1 success, got %v", err)
	}

	// Second request: same key, but different amount 2000 -> must return ErrConflict!
	req2 := TransferRequest{
		TenantID:       "tenant_1",
		IdempotencyKey: "key_xyz",
		FromAccountID:  "acc_1",
		ToAccountID:    "acc_2",
		Amount:         2000,
	}
	err := service.Transfer(context.Background(), req2)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for different payload with same key, got %v", err)
	}
}

func TestConcurrentDuplicateRequests(t *testing.T) {
	transferRepo := newMockTransferRepository(map[string]Account{
		"acc_1": {ID: "acc_1", Balance: 5000},
		"acc_2": {ID: "acc_2", Balance: 1000},
	})
	idemRepo := newInMemoryIdempotencyRepo()

	service := NewTransferService(transferRepo).WithIdempotency(idemRepo)

	req := TransferRequest{
		TenantID:       "tenant_1",
		IdempotencyKey: "concurrent_key_999",
		FromAccountID:  "acc_1",
		ToAccountID:    "acc_2",
		Amount:         1000,
	}

	const concurrency = 10
	var wg sync.WaitGroup
	var successfulTransfers int
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := service.Transfer(context.Background(), req)
			if err == nil {
				mu.Lock()
				successfulTransfers++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Crucial assertion: exactly 1000 debited, balance is exactly 4000, NEVER debited 10,000!
	if transferRepo.accounts["acc_1"].Balance != 4000 {
		t.Fatalf("expected final balance 4000 (exactly one transfer executed), got %d", transferRepo.accounts["acc_1"].Balance)
	}
	if transferRepo.accounts["acc_2"].Balance != 2000 {
		t.Fatalf("expected destination balance 2000, got %d", transferRepo.accounts["acc_2"].Balance)
	}
}
