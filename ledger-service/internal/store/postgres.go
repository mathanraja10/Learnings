package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	DB *pgxpool.Pool
}

func NewPostgresStore(
	ctx context.Context,
	databaseURL string,
) (*PostgresStore, error) {

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresStore{
		DB: db,
	}, nil
}

func (s *PostgresStore) Close() {
	s.DB.Close()
}
