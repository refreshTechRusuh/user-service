// Package db: adapter PostgreSQL untuk member_nik (implementasi IMemberRepository).
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

type MemberRepository struct {
	pool *pgxpool.Pool
}

func NewMemberRepository(pool *pgxpool.Pool) *MemberRepository {
	return &MemberRepository{pool: pool}
}

// Create menyimpan member. NIK dobel untuk user sama (UNIQUE 23505) -> MemberDuplicate.
func (r *MemberRepository) Create(ctx context.Context, m entity.MemberNIK) error {
	const q = `
		INSERT INTO member_nik (id, user_id, nik, nama, tgl_lahir, nama_ibu, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.pool.Exec(ctx, q, m.ID, m.UserID, m.NIK, m.Nama, m.TglLahir, m.NamaIbu, m.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return apperror.MemberDuplicate()
		}
		return fmt.Errorf("insert member_nik: %w", err)
	}
	return nil
}

// ListByUser mengembalikan semua member milik user (terbaru dulu).
func (r *MemberRepository) ListByUser(ctx context.Context, userID string) ([]entity.MemberNIK, error) {
	const q = `
		SELECT id, user_id, nik, nama, to_char(tgl_lahir,'YYYY-MM-DD'), nama_ibu, created_at
		FROM member_nik
		WHERE user_id = $1
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list member_nik: %w", err)
	}
	defer rows.Close()

	var result []entity.MemberNIK
	for rows.Next() {
		var m entity.MemberNIK
		if err := rows.Scan(&m.ID, &m.UserID, &m.NIK, &m.Nama, &m.TglLahir, &m.NamaIbu, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member_nik: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// Update mengubah member HANYA bila id DAN user_id cocok (scoping antar-user).
// found=false bila tak ada baris yang cocok (bukan milik user / id salah).
func (r *MemberRepository) Update(ctx context.Context, m entity.MemberNIK) error {
	const q = `
		UPDATE member_nik
		SET nik = $3, nama = $4, tgl_lahir = $5, nama_ibu = $6
		WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, q, m.ID, m.UserID, m.NIK, m.Nama, m.TglLahir, m.NamaIbu)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return apperror.MemberDuplicate()
		}
		return fmt.Errorf("update member_nik: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("member")
	}
	return nil
}

// Delete menghapus member milik user. found=false bila bukan milik user / tak ada.
func (r *MemberRepository) Delete(ctx context.Context, userID, id string) (bool, error) {
	const q = `DELETE FROM member_nik WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, q, id, userID)
	if err != nil {
		return false, fmt.Errorf("delete member_nik: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
