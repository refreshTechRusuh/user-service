package http

import (
	"net/http"
	"strconv"

	"github.com/refreshTechRusuh/user-service/internal/user/port/inbound"
	"github.com/refreshTechRusuh/user-service/pkg/apinizer"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
	"github.com/refreshTechRusuh/user-service/pkg/middleware"
)

type HistoryHandler struct {
	uc inbound.IHistoryUsecase
}

func NewHistoryHandler(uc inbound.IHistoryUsecase) *HistoryHandler {
	return &HistoryHandler{uc: uc}
}

// List — GET /api/user/v1/ticket-history?page=1&pageSize=20
func (h *HistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		apinizer.WriteError(w, apperror.Unauthorized("Sesi tidak valid"))
		return
	}
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	pageSize := atoiDefault(r.URL.Query().Get("pageSize"), 20)

	res, err := h.uc.List(r.Context(), userID, page, pageSize)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "OK", res)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
