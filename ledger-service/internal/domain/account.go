package domain

import (
	"context"
	"fmt"
)

type Account struct {
	ID       string
	TenantID string
	Currency string
	Balance  int64
}

type AccountService struct {
	accounts map[string]Account
}

func NewAccountService() *AccountService {
	return &AccountService{
		accounts: make(map[string]Account),
	}
}

func (s *AccountService) GetAccount(
	ctx context.Context,
	accountID string,
) (Account, error) {

	select {
	case <-ctx.Done():
		return Account{}, ctx.Err()

	default:
	}

	account, ok := s.accounts[accountID]
	if !ok {
		return Account{}, fmt.Errorf(
			"get account %s: %w",
			accountID,
			ErrNotFound,
		)
	}

	return account, nil
}

func (s *AccountService) CreateAccount(
	ctx context.Context,
	account Account,
) error {

	select {
	case <-ctx.Done():
		return ctx.Err()

	default:
	}

	if _, exists := s.accounts[account.ID]; exists {
		return fmt.Errorf(
			"create account %s: %w",
			account.ID,
			ErrConflict,
		)
	}

	s.accounts[account.ID] = account

	return nil
}
