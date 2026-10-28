// Package http (infrastructure) = adapter HTTP untuk user-service.
// Tugasnya: terjemahkan request HTTP <-> usecase. TIDAK ada logika bisnis di sini.
// Bergantung ke inbound.IUserUsecase (interface), bukan usecase konkret.
package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/port/inbound"
	"github.com/refreshTechRusuh/user-service/pkg/apinizer"
)

type UserHandler struct {
	uc inbound.IUserUsecase
}

func NewUserHandler(uc inbound.IUserUsecase) *UserHandler {
	return &UserHandler{uc: uc}
}

// Register menangani POST /auth/register.
// Alur: decode body -> validasi -> usecase.Register -> balas 201.
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.FailedBadRequest(w, "body tidak valid")
		return
	}

	// Validasi ringan di boundary sebelum masuk usecase.
	if errs := req.Validate(); len(errs) > 0 {
		apinizer.FailedBadRequest(w, strings.Join(errs, "; "))
		return
	}

	res, err := h.uc.Register(r.Context(), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}

	// Register sukses = 201 Created (sesuai kontrak API).
	apinizer.Created(w, "akun berhasil dibuat", res)
}
