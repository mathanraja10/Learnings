package http

import (
	"encoding/json"
	"net/http"

	"ledger-service/internal/domain"
)

type Handler struct {
	transferService *domain.TransferService
}

func NewHandler(
	transferService *domain.TransferService,
) *Handler {
	return &Handler{
		transferService: transferService,
	}
}

type healthResponse struct {
	Status string `json:"status"`
}

func (h *Handler) Health(
	w http.ResponseWriter,
	r *http.Request,
) {
	_ = r.Context()

	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(
		healthResponse{
			Status: "ok",
		},
	)
}
