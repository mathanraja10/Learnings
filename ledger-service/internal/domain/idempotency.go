package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type IdempotencyRecord struct {
	TenantID       string
	Key            string
	RequestHash    string
	Status         string
	ResponseStatus int
	ResponseBody   []byte
}

func HashRequest(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:]), nil
}
