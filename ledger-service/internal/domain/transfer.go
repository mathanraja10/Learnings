package domain

import (
	"context"
	"errors"
	"fmt"
)

type TransferRequest struct {
	TenantID       string `json:"tenant_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	FromAccountID  string `json:"from_account_id"`
	ToAccountID    string `json:"to_account_id"`
	Amount         int64  `json:"amount"`
}

type TransferRepository interface {
	Transfer(ctx context.Context, req TransferRequest) error
}

type TransferService struct {
	repo        TransferRepository
	idempotency IdempotencyRepository
	notifier    Notifier
}

func NewTransferService(
	repo TransferRepository,
) *TransferService {
	return &TransferService{
		repo: repo,
	}
}

func (s *TransferService) WithIdempotency(
	idempotency IdempotencyRepository,
) *TransferService {
	s.idempotency = idempotency
	return s
}

func (s *TransferService) WithNotifier(
	notifier Notifier,
) *TransferService {
	s.notifier = notifier
	return s
}

func (s *TransferService) Transfer(
	ctx context.Context,
	req TransferRequest,
) error {

	if req.Amount <= 0 {
		return fmt.Errorf(
			"transfer amount must be positive: %d",
			req.Amount,
		)
	}

	if req.FromAccountID == req.ToAccountID {
		return fmt.Errorf(
			"source and destination accounts must be different: %w",
			ErrConflict,
		)
	}

	// If idempotency is enabled and a key is provided
	if s.idempotency != nil && req.IdempotencyKey != "" {
		return s.transferWithIdempotency(ctx, req)
	}

	if err := s.repo.Transfer(ctx, req); err != nil {
		return err
	}

	if s.notifier != nil {
		if err := s.notifier.Notify(
			ctx,
			fmt.Sprintf(
				"transfer completed: %s -> %s amount=%d",
				req.FromAccountID,
				req.ToAccountID,
				req.Amount,
			),
		); err != nil {
			return fmt.Errorf("notify transfer: %w", err)
		}
	}

	return nil
}

func (s *TransferService) transferWithIdempotency(
	ctx context.Context,
	req TransferRequest,
) error {

	type transferPayload struct {
		FromAccountID string `json:"from_account_id"`
		ToAccountID   string `json:"to_account_id"`
		Amount        int64  `json:"amount"`
	}

	reqHash, err := HashRequest(transferPayload{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
	})
	if err != nil {
		return fmt.Errorf("hash transfer request: %w", err)
	}

	existing, err := s.idempotency.Get(ctx, req.TenantID, req.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("get idempotency record: %w", err)
	}

	if existing != nil {
		// Same key + different request = 409 Conflict
		if existing.RequestHash != reqHash {
			return fmt.Errorf(
				"idempotency key %s reused with different request payload: %w",
				req.IdempotencyKey,
				ErrConflict,
			)
		}

		// Completed -> return saved response (idempotent replay)
		if existing.Status == "completed" {
			return nil
		}

		// Currently processing
		return fmt.Errorf(
			"transfer with idempotency key %s is already processing: %w",
			req.IdempotencyKey,
			ErrConflict,
		)
	}

	// Record initial "processing" status
	err = s.idempotency.Create(ctx, IdempotencyRecord{
		TenantID:    req.TenantID,
		Key:         req.IdempotencyKey,
		RequestHash: reqHash,
		Status:      "processing",
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return fmt.Errorf("concurrent request with same idempotency key: %w", ErrConflict)
		}
		return fmt.Errorf("create idempotency record: %w", err)
	}

	// Perform actual transfer transaction
	if err := s.repo.Transfer(ctx, req); err != nil {
		return err
	}

	// Mark completed
	_ = s.idempotency.Complete(
		ctx,
		req.TenantID,
		req.IdempotencyKey,
		200,
		[]byte(`{"status":"ok"}`),
	)

	return nil
}
