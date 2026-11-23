package domain

import "context"

type NoopNotifier struct{}

func (NoopNotifier) Notify(
	ctx context.Context,
	message string,
) error {
	return nil
}
