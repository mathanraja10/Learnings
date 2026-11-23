package domain

import (
	"context"
	"time"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Health(ctx context.Context) error {
	select {
	case <-time.After(10 * time.Millisecond):
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}
