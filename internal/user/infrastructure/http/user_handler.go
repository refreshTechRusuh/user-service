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
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
	"github.com/refreshTechRusuh/user-service/pkg/middleware"
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
		apinizer.FailedBadRequest(w, msgInvalidBody)
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

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.FailedBadRequest(w, msgInvalidBody)
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		apinizer.FailedBadRequest(w, strings.Join(errs, "; "))
		return
	}

	res, err := h.uc.Login(r.Context(), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	// Login = 200 OK (bukan 201 — tidak membuat resource baru).
	apinizer.Success(w, "login berhasil", res)
}

// Me menangani GET /auth/me. userID diambil dari context (di-set RequireAuth).
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		apinizer.WriteError(w, apperror.Unauthorized(msgInvalidSession))
		return
	}
	res, err := h.uc.Me(r.Context(), userID)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "OK", res)
}

// Logout menangani POST /auth/logout. Wajib authToken (RequireAuth); sesi yang
// dicabut diambil dari klaim sid di token. Tanpa body. Idempoten.
func (h *UserHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID, _ := middleware.GetSessionID(r.Context())
	if err := h.uc.Logout(r.Context(), sessionID); err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "logout berhasil", map[string]bool{"ok": true})
}

// Refresh menangani POST /auth/refresh. Tanpa authToken (justru dipakai saat
// authToken sudah habis); kredensialnya adalah refreshToken di body.
func (h *UserHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.FailedBadRequest(w, msgInvalidBody)
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		apinizer.FailedBadRequest(w, strings.Join(errs, "; "))
		return
	}
	res, err := h.uc.Refresh(r.Context(), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "token diperbarui", res)
}
