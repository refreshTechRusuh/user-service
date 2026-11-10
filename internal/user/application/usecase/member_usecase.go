// Package usecase: logika member/NIK. Setiap simpan/ubah dicocokkan ke Dukcapil.
package usecase

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/internal/user/port/outbound"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// MemberUsecase mengelola member/NIK; butuh repo + client Dukcapil.
type MemberUsecase struct {
	repo     outbound.IMemberRepository
	dukcapil outbound.IDukcapilClient
}

func NewMemberUsecase(repo outbound.IMemberRepository, dukcapil outbound.IDukcapilClient) *MemberUsecase {
	return &MemberUsecase{repo: repo, dukcapil: dukcapil}
}

// verify memanggil Dukcapil; gagal teknis = fail-closed (anggap tidak valid → tolak).
func (u *MemberUsecase) verify(ctx context.Context, req dto.MemberRequest) error {
	valid, err := u.dukcapil.Verify(ctx, req.NIK, req.Nama, req.TglLahir, req.NamaIbu)
	if err != nil {
		// Dukcapil mati/timeout → jangan loloskan data yang belum tervalidasi.
		return apperror.NIKInvalid()
	}
	if !valid {
		return apperror.NIKInvalid()
	}
	return nil
}

// VerifyNIK mencocokkan {nik,nama,tglLahir,namaIbu} ke Dukcapil tanpa menyimpan apa pun.
//
// Beda dengan verify() di atas: di sini kegagalan teknis TIDAK dianggap "NIK tidak valid".
// FE perlu bisa membedakan "data salah" (Valid=false) dari "layanan sedang bermasalah"
// (error 503) supaya user tidak dituduh salah isi saat Dukcapil yang mati.
func (u *MemberUsecase) VerifyNIK(ctx context.Context, req dto.VerifyNIKRequest) (dto.VerifyNIKResponse, error) {
	valid, err := u.dukcapil.Verify(ctx,
		strings.TrimSpace(req.NIK), strings.TrimSpace(req.Nama),
		strings.TrimSpace(req.TglLahir), strings.TrimSpace(req.NamaIbu))
	if err != nil {
		// Jangan log NIK/nama (data pribadi); cukup penyebab teknisnya.
		slog.WarnContext(ctx, "verifikasi dukcapil gagal", "err", err)
		return dto.VerifyNIKResponse{}, apperror.DukcapilUnavailable(err)
	}
	return dto.VerifyNIKResponse{Valid: valid}, nil
}

func (u *MemberUsecase) Create(ctx context.Context, userID string, req dto.MemberRequest) (dto.MemberResponse, error) {
	if err := u.verify(ctx, req); err != nil {
		return dto.MemberResponse{}, err
	}
	m := entity.MemberNIK{
		ID:        uuid.NewString(),
		UserID:    userID,
		NIK:       req.NIK,
		Nama:      req.Nama,
		TglLahir:  req.TglLahir,
		NamaIbu:   req.NamaIbu,
		CreatedAt: time.Now().UTC(),
	}
	if err := u.repo.Create(ctx, m); err != nil {
		return dto.MemberResponse{}, err // MemberDuplicate / Internal dari repo
	}
	return toMemberResponse(m), nil
}

// toMemberResponse memetakan entity ke payload response (satu tempat agar
// list/create/update selalu seragam).
func toMemberResponse(m entity.MemberNIK) dto.MemberResponse {
	return dto.MemberResponse{
		MemberID: m.ID,
		NIK:      m.NIK,
		Nama:     m.Nama,
		TglLahir: m.TglLahir,
		NamaIbu:  m.NamaIbu,
	}
}

func (u *MemberUsecase) List(ctx context.Context, userID string) (dto.MemberListResponse, error) {
	members, err := u.repo.ListByUser(ctx, userID)
	if err != nil {
		return dto.MemberListResponse{}, apperror.Internal(err)
	}
	items := make([]dto.MemberResponse, 0, len(members))
	for _, m := range members {
		items = append(items, toMemberResponse(m))
	}
	return dto.MemberListResponse{Items: items}, nil
}

func (u *MemberUsecase) Update(ctx context.Context, userID, memberID string, req dto.MemberRequest) (dto.MemberResponse, error) {
	if err := u.verify(ctx, req); err != nil {
		return dto.MemberResponse{}, err
	}
	m := entity.MemberNIK{
		ID:       memberID,
		UserID:   userID,
		NIK:      req.NIK,
		Nama:     req.Nama,
		TglLahir: req.TglLahir,
		NamaIbu:  req.NamaIbu,
	}
	if err := u.repo.Update(ctx, m); err != nil {
		return dto.MemberResponse{}, err // NotFound / MemberDuplicate / Internal
	}
	return toMemberResponse(m), nil
}

func (u *MemberUsecase) Delete(ctx context.Context, userID, memberID string) error {
	found, err := u.repo.Delete(ctx, userID, memberID)
	if err != nil {
		return apperror.Internal(err)
	}
	if !found {
		return apperror.NotFound("member")
	}
	return nil
}
