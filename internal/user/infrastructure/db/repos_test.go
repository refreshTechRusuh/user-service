package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

var (
	errDB     = errors.New("db down")
	uniqueErr = &pgconn.PgError{Code: uniqueViolation}
)

func codeOf(t *testing.T, err error) apperror.ErrorCode {
	t.Helper()
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	return ae.Code
}

// ------------------------------------------------------------- UserRepository

func TestUserRepo_FindByEmail(t *testing.T) {
	t.Run("ketemu", func(t *testing.T) {
		f := newFake(t).expectQueryRow("FROM app_user", []any{"a@b.com"},
			"u1", "a@b.com", "hash", entity.RoleAdmin, time.Now())

		u, found, err := NewUserRepository(f).FindByEmail(context.Background(), "a@b.com")

		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "u1", u.ID)
		assert.Equal(t, entity.RoleAdmin, u.Role)
	})
	t.Run("tidak ada → found=false tanpa error", func(t *testing.T) {
		f := newFake(t).expectQueryRowErr("FROM app_user", []any{"x"}, pgx.ErrNoRows)

		_, found, err := NewUserRepository(f).FindByEmail(context.Background(), "x")

		assert.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("error teknis", func(t *testing.T) {
		f := newFake(t).expectQueryRowErr("FROM app_user", []any{"x"}, errDB)

		_, found, err := NewUserRepository(f).FindByEmail(context.Background(), "x")

		assert.ErrorIs(t, err, errDB)
		assert.False(t, found)
	})
}

func TestUserRepo_FindByID(t *testing.T) {
	t.Run("ketemu", func(t *testing.T) {
		f := newFake(t).expectQueryRow("WHERE id", []any{"u1"},
			"u1", "a@b.com", "h", entity.RoleCustomer, time.Now())

		u, found, err := NewUserRepository(f).FindByID(context.Background(), "u1")

		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "a@b.com", u.Email)
	})
	t.Run("tidak ada", func(t *testing.T) {
		f := newFake(t).expectQueryRowErr("WHERE id", []any{"u1"}, pgx.ErrNoRows)

		_, found, err := NewUserRepository(f).FindByID(context.Background(), "u1")

		assert.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("error teknis", func(t *testing.T) {
		f := newFake(t).expectQueryRowErr("WHERE id", []any{"u1"}, errDB)

		_, _, err := NewUserRepository(f).FindByID(context.Background(), "u1")

		assert.ErrorIs(t, err, errDB)
	})
}

func TestUserRepo_Create(t *testing.T) {
	u := entity.User{ID: "u1", Email: "a@b.com", PasswordHash: "h", Role: entity.RoleCustomer, CreatedAt: time.Now()}
	args := []any{u.ID, u.Email, u.PasswordHash, u.Role, u.CreatedAt}

	t.Run("sukses", func(t *testing.T) {
		f := newFake(t).expectExec("INSERT INTO app_user", args, 1)
		assert.NoError(t, NewUserRepository(f).Create(context.Background(), u))
	})
	t.Run("email dobel (23505) → EMAIL_TAKEN", func(t *testing.T) {
		f := newFake(t).expectExecErr("INSERT INTO app_user", args, uniqueErr)
		assert.Equal(t, apperror.ErrEmailTaken, codeOf(t, NewUserRepository(f).Create(context.Background(), u)))
	})
	t.Run("error lain → dibungkus, bukan AppError", func(t *testing.T) {
		f := newFake(t).expectExecErr("INSERT INTO app_user", args, errDB)
		err := NewUserRepository(f).Create(context.Background(), u)
		assert.ErrorIs(t, err, errDB)
		var ae *apperror.AppError
		assert.False(t, errors.As(err, &ae))
	})
}

// ----------------------------------------------------------- MemberRepository

var member = entity.MemberNIK{
	ID: "m1", UserID: "u1", NIK: "3273000000000001", Nama: "Budi",
	TglLahir: "1990-01-01", NamaIbu: "Siti", CreatedAt: time.Now(),
}

func TestMemberRepo_Create(t *testing.T) {
	args := []any{member.ID, member.UserID, member.NIK, member.Nama, member.TglLahir, member.NamaIbu, member.CreatedAt}

	t.Run("sukses", func(t *testing.T) {
		f := newFake(t).expectExec("INSERT INTO member_nik", args, 1)
		assert.NoError(t, NewMemberRepository(f).Create(context.Background(), member))
	})
	t.Run("NIK dobel → MEMBER_DUPLICATE", func(t *testing.T) {
		f := newFake(t).expectExecErr("INSERT INTO member_nik", args, uniqueErr)
		assert.Equal(t, apperror.ErrMemberDuplicate, codeOf(t, NewMemberRepository(f).Create(context.Background(), member)))
	})
	t.Run("error lain", func(t *testing.T) {
		f := newFake(t).expectExecErr("INSERT INTO member_nik", args, errDB)
		assert.ErrorIs(t, NewMemberRepository(f).Create(context.Background(), member), errDB)
	})
}

func TestMemberRepo_ListByUser(t *testing.T) {
	row := func(id, nik, nama, tgl, ibu string) []any {
		return []any{id, "u1", nik, nama, tgl, ibu, time.Now()}
	}

	t.Run("mengembalikan semua kolom termasuk tglLahir & namaIbu", func(t *testing.T) {
		f := newFake(t).expectQuery("FROM member_nik", []any{"u1"}, [][]any{
			row("m1", "3273000000000001", "Budi", "1990-01-01", "Siti"),
			row("m2", "3273000000000002", "Ani", "1992-02-02", "Ina"),
		})

		got, err := NewMemberRepository(f).ListByUser(context.Background(), "u1")

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "1990-01-01", got[0].TglLahir)
		assert.Equal(t, "Ina", got[1].NamaIbu)
	})
	t.Run("kosong", func(t *testing.T) {
		f := newFake(t).expectQuery("FROM member_nik", []any{"u1"}, nil)

		got, err := NewMemberRepository(f).ListByUser(context.Background(), "u1")

		assert.NoError(t, err)
		assert.Empty(t, got)
	})
	t.Run("query gagal", func(t *testing.T) {
		f := newFake(t).expectQueryErr("FROM member_nik", []any{"u1"}, errDB)

		_, err := NewMemberRepository(f).ListByUser(context.Background(), "u1")

		assert.ErrorIs(t, err, errDB)
	})
	t.Run("scan gagal (tipe kolom salah)", func(t *testing.T) {
		bad := row("m1", "n", "b", "t", "i")
		bad[6] = "bukan-waktu"
		f := newFake(t).expectQuery("FROM member_nik", []any{"u1"}, [][]any{bad})

		_, err := NewMemberRepository(f).ListByUser(context.Background(), "u1")

		assert.Error(t, err)
	})
	t.Run("error saat iterasi baris", func(t *testing.T) {
		f := newFake(t).expectQueryRowErrAfter("FROM member_nik", []any{"u1"}, nil, errDB)

		_, err := NewMemberRepository(f).ListByUser(context.Background(), "u1")

		assert.ErrorIs(t, err, errDB)
	})
}

func TestMemberRepo_Update(t *testing.T) {
	args := []any{member.ID, member.UserID, member.NIK, member.Nama, member.TglLahir, member.NamaIbu}

	t.Run("sukses", func(t *testing.T) {
		f := newFake(t).expectExec("UPDATE member_nik", args, 1)
		assert.NoError(t, NewMemberRepository(f).Update(context.Background(), member))
	})
	t.Run("0 baris (bukan milik user) → NOT_FOUND", func(t *testing.T) {
		f := newFake(t).expectExec("UPDATE member_nik", args, 0)
		assert.Equal(t, apperror.ErrNotFound, codeOf(t, NewMemberRepository(f).Update(context.Background(), member)))
	})
	t.Run("NIK dobel → MEMBER_DUPLICATE", func(t *testing.T) {
		f := newFake(t).expectExecErr("UPDATE member_nik", args, uniqueErr)
		assert.Equal(t, apperror.ErrMemberDuplicate, codeOf(t, NewMemberRepository(f).Update(context.Background(), member)))
	})
	t.Run("error lain", func(t *testing.T) {
		f := newFake(t).expectExecErr("UPDATE member_nik", args, errDB)
		assert.ErrorIs(t, NewMemberRepository(f).Update(context.Background(), member), errDB)
	})
}

func TestMemberRepo_Delete(t *testing.T) {
	t.Run("terhapus — id & user_id dipakai sebagai filter", func(t *testing.T) {
		f := newFake(t).expectExec("DELETE FROM member_nik", []any{"m1", "u1"}, 1)
		found, err := NewMemberRepository(f).Delete(context.Background(), "u1", "m1")
		assert.NoError(t, err)
		assert.True(t, found)
	})
	t.Run("bukan milik user → found=false", func(t *testing.T) {
		f := newFake(t).expectExec("DELETE FROM member_nik", []any{"m1", "u1"}, 0)
		found, err := NewMemberRepository(f).Delete(context.Background(), "u1", "m1")
		assert.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("error", func(t *testing.T) {
		f := newFake(t).expectExecErr("DELETE FROM member_nik", []any{"m1", "u1"}, errDB)
		_, err := NewMemberRepository(f).Delete(context.Background(), "u1", "m1")
		assert.ErrorIs(t, err, errDB)
	})
}

// ---------------------------------------------------------- HistoryRepository

func TestHistoryRepo_ListByUser(t *testing.T) {
	t.Run("total + item", func(t *testing.T) {
		f := newFake(t).
			expectQueryRow("SELECT COUNT", []any{"u1"}, 5).
			expectQuery("FROM booking b", []any{"u1", 2, 0}, [][]any{
				{"b1", "Konser A", "PAID", 2, int64(3000000), "2026-11-01T10:00:00+07"},
			})

		items, total, err := NewHistoryRepository(f).ListByUser(context.Background(), "u1", 2, 0)

		require.NoError(t, err)
		assert.Equal(t, 5, total)
		require.Len(t, items, 1)
		assert.Equal(t, "Konser A", items[0].EventName)
		assert.Equal(t, int64(3000000), items[0].Total)
	})
	t.Run("count gagal", func(t *testing.T) {
		f := newFake(t).expectQueryRowErr("SELECT COUNT", []any{"u1"}, errDB)
		_, _, err := NewHistoryRepository(f).ListByUser(context.Background(), "u1", 2, 0)
		assert.ErrorIs(t, err, errDB)
	})
	t.Run("query item gagal", func(t *testing.T) {
		f := newFake(t).
			expectQueryRow("SELECT COUNT", []any{"u1"}, 1).
			expectQueryErr("FROM booking b", []any{"u1", 2, 0}, errDB)
		_, _, err := NewHistoryRepository(f).ListByUser(context.Background(), "u1", 2, 0)
		assert.ErrorIs(t, err, errDB)
	})
	t.Run("scan gagal", func(t *testing.T) {
		f := newFake(t).
			expectQueryRow("SELECT COUNT", []any{"u1"}, 1).
			expectQuery("FROM booking b", []any{"u1", 2, 0}, [][]any{
				{"b1", "K", "PAID", "bukan-angka", int64(1), "t"},
			})
		_, _, err := NewHistoryRepository(f).ListByUser(context.Background(), "u1", 2, 0)
		assert.Error(t, err)
	})
}
