package domain

import (
	"context"
	"testing"
)

type mockNotifier struct {
	messages []string
}

func (m *mockNotifier) Notify(
	ctx context.Context,
	message string,
) error {
	m.messages = append(m.messages, message)
	return nil
}

func TestTransferWithNotifier(t *testing.T) {
	transferRepo := newMockTransferRepository(map[string]Account{
		"acc_1": {ID: "acc_1", Balance: 5000},
		"acc_2": {ID: "acc_2", Balance: 1000},
	})
	notifier := &mockNotifier{}

	service := NewTransferService(transferRepo).WithNotifier(notifier)

	req := TransferRequest{
		FromAccountID: "acc_1",
		ToAccountID:   "acc_2",
		Amount:        1000,
	}

	err := service.Transfer(context.Background(), req)
	if err != nil {
		t.Fatalf("expected transfer success, got %v", err)
	}

	if len(notifier.messages) != 1 {
		t.Fatalf("expected 1 notification message, got %d", len(notifier.messages))
	}

	expected := "transfer completed: acc_1 -> acc_2 amount=1000"
	if notifier.messages[0] != expected {
		t.Fatalf("expected notification message %q, got %q", expected, notifier.messages[0])
	}
}
