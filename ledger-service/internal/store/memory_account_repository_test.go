package store

import (
	"context"
	"errors"
	"testing"

	"ledger-service/internal/domain"
)

func TestMemoryAccountRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryAccountRepository()

	// 1. Get non-existent account
	_, err := repo.GetByID(ctx, "acc_unknown")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// 2. Create account
	acc := domain.Account{
		ID:      "acc_1",
		Balance: 1000,
	}
	if err := repo.Create(ctx, acc); err != nil {
		t.Fatalf("expected create success, got %v", err)
	}

	// 3. Create duplicate account
	if err := repo.Create(ctx, acc); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// 4. Get created account
	found, err := repo.GetByID(ctx, "acc_1")
	if err != nil {
		t.Fatalf("expected get success, got %v", err)
	}
	if found.Balance != 1000 {
		t.Fatalf("expected balance 1000, got %d", found.Balance)
	}

	// 5. Update balance
	if err := repo.UpdateBalance(ctx, "acc_1", 2500); err != nil {
		t.Fatalf("expected update balance success, got %v", err)
	}

	updated, err := repo.GetByID(ctx, "acc_1")
	if err != nil {
		t.Fatalf("expected get success, got %v", err)
	}
	if updated.Balance != 2500 {
		t.Fatalf("expected balance 2500, got %d", updated.Balance)
	}

	// 6. Transfer tests
	acc2 := domain.Account{
		ID:      "acc_2",
		Balance: 500,
	}
	_ = repo.Create(ctx, acc2)

	// Transfer insufficient funds
	err = repo.Transfer(ctx, domain.TransferRequest{
		FromAccountID: "acc_1",
		ToAccountID:   "acc_2",
		Amount:        5000,
	})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}

	// Transfer success
	err = repo.Transfer(ctx, domain.TransferRequest{
		FromAccountID: "acc_1",
		ToAccountID:   "acc_2",
		Amount:        1000,
	})
	if err != nil {
		t.Fatalf("expected transfer success, got %v", err)
	}

	fromAfter, _ := repo.GetByID(ctx, "acc_1")
	if fromAfter.Balance != 1500 {
		t.Fatalf("expected acc_1 balance 1500, got %d", fromAfter.Balance)
	}

	toAfter, _ := repo.GetByID(ctx, "acc_2")
	if toAfter.Balance != 1500 {
		t.Fatalf("expected acc_2 balance 1500, got %d", toAfter.Balance)
	}

	// 7. Context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.GetByID(cancelCtx, "acc_1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
