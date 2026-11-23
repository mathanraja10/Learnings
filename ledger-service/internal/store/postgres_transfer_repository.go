package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"ledger-service/internal/domain"
)

type PostgresTransferRepository struct {
	store *PostgresStore
}

func NewPostgresTransferRepository(
	store *PostgresStore,
) *PostgresTransferRepository {
	return &PostgresTransferRepository{
		store: store,
	}
}

func (r *PostgresTransferRepository) Transfer(
	ctx context.Context,
	req domain.TransferRequest,
) error {

	tx, err := r.store.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transfer transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Lock source account.
	from, err := lockAccount(
		ctx,
		tx,
		req.FromAccountID,
	)
	if err != nil {
		return fmt.Errorf("lock source account: %w", err)
	}

	// Lock destination account.
	to, err := lockAccount(
		ctx,
		tx,
		req.ToAccountID,
	)
	if err != nil {
		return fmt.Errorf("lock destination account: %w", err)
	}

	if from.Balance < req.Amount {
		return fmt.Errorf(
			"balance %d is less than amount %d: %w",
			from.Balance,
			req.Amount,
			domain.ErrInsufficientFunds,
		)
	}

	newFromBalance := from.Balance - req.Amount
	newToBalance := to.Balance + req.Amount

	_, err = tx.Exec(
		ctx,
		`
		UPDATE accounts
		SET balance = $1,
		    updated_at = NOW()
		WHERE id = $2
		`,
		newFromBalance,
		from.ID,
	)
	if err != nil {
		return fmt.Errorf("debit source account: %w", err)
	}

	_, err = tx.Exec(
		ctx,
		`
		UPDATE accounts
		SET balance = $1,
		    updated_at = NOW()
		WHERE id = $2
		`,
		newToBalance,
		to.ID,
	)
	if err != nil {
		return fmt.Errorf("credit destination account: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transfer: %w", err)
	}

	return nil
}

func lockAccount(
	ctx context.Context,
	tx pgx.Tx,
	accountID string,
) (domain.Account, error) {

	const query = `
		SELECT id, tenant_id, currency, balance
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`

	var account domain.Account

	err := tx.QueryRow(
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
			"query account: %w",
			err,
		)
	}

	return account, nil
}
