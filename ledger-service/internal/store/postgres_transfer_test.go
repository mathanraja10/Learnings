package store

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"ledger-service/internal/domain"
)

func TestConcurrentIdempotentTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/ledger"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pgStore, err := NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Skipf("skipping integration test: postgres not reachable (%v)", err)
		return
	}
	defer pgStore.Close()

	// Ensure tables exist for integration testing
	_, err = pgStore.DB.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS accounts (
			id UUID PRIMARY KEY,
			tenant_id UUID NOT NULL,
			currency CHAR(3) NOT NULL,
			balance BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT accounts_balance_non_negative CHECK (balance >= 0)
		);
		CREATE TABLE IF NOT EXISTS idempotency_keys (
			tenant_id UUID NOT NULL,
			idempotency_key TEXT NOT NULL,
			request_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			response_status INT,
			response_body JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (tenant_id, idempotency_key)
		);
	`)
	if err != nil {
		t.Fatalf("failed to setup test tables: %v", err)
	}

	tenantID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	fromAccountID := "b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22"
	toAccountID := "c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a33"
	idempotencyKey := "idem_concurrent_test_key_001"

	// Clean up any previous test artifacts
	_, _ = pgStore.DB.Exec(ctx, "DELETE FROM idempotency_keys WHERE tenant_id = $1", tenantID)
	_, _ = pgStore.DB.Exec(ctx, "DELETE FROM accounts WHERE id IN ($1, $2)", fromAccountID, toAccountID)

	// Seed source and destination accounts
	_, err = pgStore.DB.Exec(ctx, `
		INSERT INTO accounts (id, tenant_id, currency, balance)
		VALUES ($1, $2, 'INR', 10000), ($3, $2, 'INR', 0)
	`, fromAccountID, tenantID, toAccountID)
	if err != nil {
		t.Fatalf("failed to seed test accounts: %v", err)
	}

	service := NewPostgresTransferService(pgStore)

	req := domain.TransferRequest{
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Amount:        1000,
	}

	const concurrency = 10
	var wg sync.WaitGroup
	results := make([]TransferResult, concurrency)
	errorsList := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, transferErr := service.Transfer(context.Background(), tenantID, idempotencyKey, req)
			results[idx] = res
			errorsList[idx] = transferErr
		}(i)
	}

	wg.Wait()

	// Verify all returned success
	for i, trErr := range errorsList {
		if trErr != nil {
			t.Errorf("worker %d returned error: %v", i, trErr)
		}
		if results[i].StatusCode != 201 {
			t.Errorf("worker %d returned unexpected status code: %d", i, results[i].StatusCode)
		}
		var respMap map[string]any
		if err := json.Unmarshal(results[i].Body, &respMap); err != nil {
			t.Errorf("worker %d returned invalid JSON: %v", i, err)
		}
	}

	// CRITICAL LEDGER VERIFICATION:
	// Verify that source balance was debited EXACTLY ONCE (10000 - 1000 = 9000), NOT 10 times!
	var finalFromBalance, finalToBalance int64
	err = pgStore.DB.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = $1", fromAccountID).Scan(&finalFromBalance)
	if err != nil {
		t.Fatalf("failed to query source balance: %v", err)
	}
	if finalFromBalance != 9000 {
		t.Fatalf("expected source balance 9000, got %d (RACE CONDITION DETECTED!)", finalFromBalance)
	}

	err = pgStore.DB.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = $1", toAccountID).Scan(&finalToBalance)
	if err != nil {
		t.Fatalf("failed to query destination balance: %v", err)
	}
	if finalToBalance != 1000 {
		t.Fatalf("expected destination balance 1000, got %d", finalToBalance)
	}
}
