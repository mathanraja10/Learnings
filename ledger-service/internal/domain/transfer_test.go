package domain

import (
	"context"
	"errors"
	"testing"
)

type mockTransferRepository struct {
	accounts map[string]Account
}

func newMockTransferRepository(accounts map[string]Account) *mockTransferRepository {
	return &mockTransferRepository{
		accounts: accounts,
	}
}

func (m *mockTransferRepository) Transfer(ctx context.Context, req TransferRequest) error {
	from, ok := m.accounts[req.FromAccountID]
	if !ok {
		return ErrNotFound
	}

	to, ok := m.accounts[req.ToAccountID]
	if !ok {
		return ErrNotFound
	}

	if from.Balance < req.Amount {
		return ErrInsufficientFunds
	}

	from.Balance -= req.Amount
	to.Balance += req.Amount

	m.accounts[req.FromAccountID] = from
	m.accounts[req.ToAccountID] = to

	return nil
}

func TestTransferInvalidAmount(t *testing.T) {
	repo := newMockTransferRepository(map[string]Account{
		"acc_1": {ID: "acc_1", Balance: 5000},
		"acc_2": {ID: "acc_2", Balance: 1000},
	})
	service := NewTransferService(repo)

	err := service.Transfer(
		context.Background(),
		TransferRequest{
			FromAccountID: "acc_1",
			ToAccountID:   "acc_2",
			Amount:        0,
		},
	)
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}

	errNeg := service.Transfer(
		context.Background(),
		TransferRequest{
			FromAccountID: "acc_1",
			ToAccountID:   "acc_2",
			Amount:        -500,
		},
	)
	if errNeg == nil {
		t.Fatal("expected error for negative amount, got nil")
	}
}

func TestTransferSameAccountConflict(t *testing.T) {
	repo := newMockTransferRepository(map[string]Account{
		"acc_1": {ID: "acc_1", Balance: 5000},
	})
	service := NewTransferService(repo)

	err := service.Transfer(
		context.Background(),
		TransferRequest{
			FromAccountID: "acc_1",
			ToAccountID:   "acc_1",
			Amount:        1000,
		},
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for same account transfer, got %v", err)
	}
}

func TestTransferInsufficientFunds(t *testing.T) {
	repo := newMockTransferRepository(map[string]Account{
		"acc_1": {
			ID:      "acc_1",
			Balance: 5000,
		},
		"acc_2": {
			ID:      "acc_2",
			Balance: 1000,
		},
	})

	service := NewTransferService(repo)

	err := service.Transfer(
		context.Background(),
		TransferRequest{
			FromAccountID: "acc_1",
			ToAccountID:   "acc_2",
			Amount:        6000,
		},
	)

	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf(
			"expected ErrInsufficientFunds, got %v",
			err,
		)
	}
}

func TestTransferSuccess(t *testing.T) {
	repo := newMockTransferRepository(map[string]Account{
		"acc_1": {
			ID:      "acc_1",
			Balance: 5000,
		},
		"acc_2": {
			ID:      "acc_2",
			Balance: 1000,
		},
	})

	service := NewTransferService(repo)

	err := service.Transfer(
		context.Background(),
		TransferRequest{
			FromAccountID: "acc_1",
			ToAccountID:   "acc_2",
			Amount:        2000,
		},
	)

	if err != nil {
		t.Fatalf("expected transfer success, got %v", err)
	}

	if repo.accounts["acc_1"].Balance != 3000 {
		t.Fatalf("expected source balance 3000, got %d", repo.accounts["acc_1"].Balance)
	}

	if repo.accounts["acc_2"].Balance != 3000 {
		t.Fatalf("expected destination balance 3000, got %d", repo.accounts["acc_2"].Balance)
	}
}
