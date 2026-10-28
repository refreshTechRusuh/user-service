// Package db (infrastructure) mengimplementasikan outbound.IUserRepository
// memakai PostgreSQL (pgxpool). Satu-satunya tempat yang tahu SQL.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// uniqueViolation = kode error PostgreSQL untuk pelanggaran UNIQUE constraint.
const uniqueViolation = "23505"

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// FindByEmail mencari satu user berdasarkan email.
//   - ketemu        -> (user, true, nil)
//   - tidak ada     -> (zero, false, nil)
//   - error teknis  -> (zero, false, err)
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (entity.User, bool, error) {
	const q = `
		SELECT id, email, password_hash, role, created_at
		FROM app_user
		WHERE email = $1`

	var res entity.User
	err := r.pool.QueryRow(ctx, q, email).
		Scan(&res.ID, &res.Email, &res.PasswordHash, &res.Role, &res.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, false, nil
		}
		return entity.User{}, false, fmt.Errorf("query app_user: %w", err)
	}
	return res, true, nil
}

// Create menyimpan user baru.
// Bila email sudah dipakai (UNIQUE violation 23505) -> apperror.EmailTaken()
// sebagai benteng terakhir, walau usecase sudah cek lewat FindByEmail.
func (r *UserRepository) Create(ctx context.Context, u entity.User) error {
	const q = `
		INSERT INTO app_user (id, email, password_hash, role, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.pool.Exec(ctx, q, u.ID, u.Email, u.PasswordHash, u.Role, u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return apperror.EmailTaken()
		}
		return fmt.Errorf("insert app_user: %w", err)
	}
	return nil
}
