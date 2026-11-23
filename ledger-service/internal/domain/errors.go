package domain

import "errors"

var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("resource conflict")
	ErrInsufficientFunds = errors.New("insufficient funds")
)
