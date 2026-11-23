package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ledger-service/internal/domain"
)

type PostgresAccountRepository struct {
	db *pgxpool.Pool
}

func NewPostgresAccountRepository(
	db *pgxpool.Pool,
) *PostgresAccountRepository {
	return &PostgresAccountRepository{
		db: db,
	}
}

func (r *PostgresAccountRepository) GetByID(
	ctx context.Context,
	accountID string,
) (domain.Account, error) {

	const query = `
		SELECT id, tenant_id, currency, balance
		FROM accounts
		WHERE id = $1
	`

	var account domain.Account

	err := r.db.QueryRow(
		ctx,
		query,
		accountID,
	).Scan(
		&account.ID,
		&account.TenantID,
		&account.Currency,
		&account.Balance,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrNotFound
		}

		return domain.Account{}, fmt.Errorf(
			"get account: %w",
			err,
		)
	}

	return account, nil
}
