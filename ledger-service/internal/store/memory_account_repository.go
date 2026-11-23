package store

import (
	"context"
	"fmt"
	"sync"

	"ledger-service/internal/domain"
)

type MemoryAccountRepository struct {
	mu       sync.RWMutex
	accounts map[string]domain.Account
}

func NewMemoryAccountRepository() *MemoryAccountRepository {
	return &MemoryAccountRepository{
		accounts: make(map[string]domain.Account),
	}
}

func (r *MemoryAccountRepository) GetByID(
	ctx context.Context,
	accountID string,
) (domain.Account, error) {

	select {
	case <-ctx.Done():
		return domain.Account{}, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	account, ok := r.accounts[accountID]

	if !ok {
		return domain.Account{}, domain.ErrNotFound
	}

	return account, nil
}

func (r *MemoryAccountRepository) Create(
	ctx context.Context,
	account domain.Account,
) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.accounts[account.ID]; exists {
		return domain.ErrConflict
	}

	r.accounts[account.ID] = account

	return nil
}

func (r *MemoryAccountRepository) UpdateBalance(
	ctx context.Context,
	accountID string,
	newBalance int64,
) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	account, ok := r.accounts[accountID]

	if !ok {
		return domain.ErrNotFound
	}

	account.Balance = newBalance
	r.accounts[accountID] = account

	return nil
}

func (r *MemoryAccountRepository) Transfer(
	ctx context.Context,
	req domain.TransferRequest,
) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	from, ok := r.accounts[req.FromAccountID]
	if !ok {
		return fmt.Errorf("source account not found: %w", domain.ErrNotFound)
	}

	to, ok := r.accounts[req.ToAccountID]
	if !ok {
		return fmt.Errorf("destination account not found: %w", domain.ErrNotFound)
	}

	if from.Balance < req.Amount {
		return fmt.Errorf(
			"source balance %d is less than requested %d: %w",
			from.Balance,
			req.Amount,
			domain.ErrInsufficientFunds,
		)
	}

	from.Balance -= req.Amount
	to.Balance += req.Amount

	r.accounts[req.FromAccountID] = from
	r.accounts[req.ToAccountID] = to

	return nil
}
