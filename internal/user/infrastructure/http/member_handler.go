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

// MemberHandler menangani endpoint member/NIK (semua butuh authToken).
type MemberHandler struct {
	uc inbound.IMemberUsecase
}

func NewMemberHandler(uc inbound.IMemberUsecase) *MemberHandler {
	return &MemberHandler{uc: uc}
}

// Create — POST /api/user/v1/member/create
func (h *MemberHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		apinizer.WriteError(w, apperror.Unauthorized(msgInvalidSession))
		return
	}
	var req dto.MemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.FailedBadRequest(w, msgInvalidBody)
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		apinizer.FailedBadRequest(w, strings.Join(errs, "; "))
		return
	}
	res, err := h.uc.Create(r.Context(), userID, req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "member tersimpan", res)
}

// List — GET /api/user/v1/member/list
func (h *MemberHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		apinizer.WriteError(w, apperror.Unauthorized(msgInvalidSession))
		return
	}
	res, err := h.uc.List(r.Context(), userID)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "OK", res)
}

// Update — PUT /api/user/v1/member/update/{id}
func (h *MemberHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		apinizer.WriteError(w, apperror.Unauthorized(msgInvalidSession))
		return
	}
	memberID := r.PathValue("id")
	var req dto.MemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.FailedBadRequest(w, msgInvalidBody)
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		apinizer.FailedBadRequest(w, strings.Join(errs, "; "))
		return
	}
	res, err := h.uc.Update(r.Context(), userID, memberID, req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "member diperbarui", res)
}

// Delete — DELETE /api/user/v1/member/delete/{id}
func (h *MemberHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		apinizer.WriteError(w, apperror.Unauthorized(msgInvalidSession))
		return
	}
	memberID := r.PathValue("id")
	if err := h.uc.Delete(r.Context(), userID, memberID); err != nil {
		apinizer.WriteError(w, err)
		return
	}
	apinizer.Success(w, "member dihapus", map[string]bool{"ok": true})
}
