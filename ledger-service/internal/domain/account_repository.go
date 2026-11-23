package domain

import "context"

type AccountRepository interface {
	GetByID(ctx context.Context, accountID string) (Account, error)

	Create(ctx context.Context, account Account) error

	UpdateBalance(
		ctx context.Context,
		accountID string,
		newBalance int64,
	) error
}
