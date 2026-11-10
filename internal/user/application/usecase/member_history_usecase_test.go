package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// stubMemberRepo = IMemberRepository dengan perilaku bisa diatur per test.
type stubMemberRepo struct {
	createErr error
	updateErr error
	listErr   error
	list      []entity.MemberNIK
	deleted   bool
	deleteErr error
	created   []entity.MemberNIK
	updated   []entity.MemberNIK
}

func (s *stubMemberRepo) Create(_ context.Context, m entity.MemberNIK) error {
	s.created = append(s.created, m)
	return s.createErr
}

func (s *stubMemberRepo) ListByUser(context.Context, string) ([]entity.MemberNIK, error) {
	return s.list, s.listErr
}

func (s *stubMemberRepo) Update(_ context.Context, m entity.MemberNIK) error {
	s.updated = append(s.updated, m)
	return s.updateErr
}

func (s *stubMemberRepo) Delete(context.Context, string, string) (bool, error) {
	return s.deleted, s.deleteErr
}

var goodReq = dto.MemberRequest{NIK: "3273000000000001", Nama: "Budi", TglLahir: "1990-01-01", NamaIbu: "Siti"}

// ------------------------------------------------------------------ Create

func TestCreate_ShouldNotSave_WhenDukcapilSaysInvalid(t *testing.T) {
	repo := &stubMemberRepo{}
	uc := NewMemberUsecase(repo, &fakeDukcapil{valid: false})

	_, err := uc.Create(context.Background(), "u1", goodReq)

	assert.Equal(t, apperror.ErrNIKInvalid, codeOf(t, err))
	assert.Empty(t, repo.created, "data yang tidak lolos Dukcapil tidak boleh tersimpan")
}

func TestCreate_ShouldFailClosed_WhenDukcapilDown(t *testing.T) {
	repo := &stubMemberRepo{}
	uc := NewMemberUsecase(repo, &fakeDukcapil{err: errors.New("timeout")})

	_, err := uc.Create(context.Background(), "u1", goodReq)

	assert.Equal(t, apperror.ErrNIKInvalid, codeOf(t, err))
	assert.Empty(t, repo.created)
}

func TestCreate_ShouldSaveWithUserIDAndGeneratedID_WhenValid(t *testing.T) {
	repo := &stubMemberRepo{}
	uc := NewMemberUsecase(repo, &fakeDukcapil{valid: true})

	res, err := uc.Create(context.Background(), "u1", goodReq)

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	assert.Equal(t, "u1", repo.created[0].UserID)
	assert.NotEmpty(t, repo.created[0].ID)
	assert.Equal(t, repo.created[0].ID, res.MemberID)
	assert.False(t, repo.created[0].CreatedAt.IsZero())
}

func TestCreate_ShouldPropagateDuplicate_WhenRepoRejects(t *testing.T) {
	uc := NewMemberUsecase(&stubMemberRepo{createErr: apperror.MemberDuplicate()}, &fakeDukcapil{valid: true})

	_, err := uc.Create(context.Background(), "u1", goodReq)

	assert.Equal(t, apperror.ErrMemberDuplicate, codeOf(t, err))
}

// -------------------------------------------------------------------- List

func TestList_ShouldReturnEmptyItems_WhenNoMembers(t *testing.T) {
	uc := NewMemberUsecase(&stubMemberRepo{}, &fakeDukcapil{})

	res, err := uc.List(context.Background(), "u1")

	require.NoError(t, err)
	assert.NotNil(t, res.Items, "harus [] bukan null di JSON")
	assert.Empty(t, res.Items)
}

func TestList_ShouldReturnInternal_WhenRepoFails(t *testing.T) {
	uc := NewMemberUsecase(&stubMemberRepo{listErr: errors.New("db")}, &fakeDukcapil{})

	_, err := uc.List(context.Background(), "u1")

	assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
}

// ------------------------------------------------------------------ Update

func TestUpdate_ShouldNotSave_WhenDukcapilSaysInvalid(t *testing.T) {
	repo := &stubMemberRepo{}
	uc := NewMemberUsecase(repo, &fakeDukcapil{valid: false})

	_, err := uc.Update(context.Background(), "u1", "m1", goodReq)

	assert.Equal(t, apperror.ErrNIKInvalid, codeOf(t, err))
	assert.Empty(t, repo.updated)
}

func TestUpdate_ShouldScopeToUserAndMember_WhenValid(t *testing.T) {
	repo := &stubMemberRepo{}
	uc := NewMemberUsecase(repo, &fakeDukcapil{valid: true})

	_, err := uc.Update(context.Background(), "u1", "m1", goodReq)

	require.NoError(t, err)
	require.Len(t, repo.updated, 1)
	assert.Equal(t, "m1", repo.updated[0].ID)
	assert.Equal(t, "u1", repo.updated[0].UserID)
}

func TestUpdate_ShouldPropagateNotFound_WhenMemberNotOwned(t *testing.T) {
	uc := NewMemberUsecase(&stubMemberRepo{updateErr: apperror.NotFound("member")}, &fakeDukcapil{valid: true})

	_, err := uc.Update(context.Background(), "u1", "m1", goodReq)

	assert.Equal(t, apperror.ErrNotFound, codeOf(t, err))
}

// ------------------------------------------------------------------ Delete

func TestDelete(t *testing.T) {
	t.Run("ada → sukses", func(t *testing.T) {
		uc := NewMemberUsecase(&stubMemberRepo{deleted: true}, &fakeDukcapil{})
		assert.NoError(t, uc.Delete(context.Background(), "u1", "m1"))
	})
	t.Run("bukan milik user / tidak ada → NOT_FOUND", func(t *testing.T) {
		uc := NewMemberUsecase(&stubMemberRepo{deleted: false}, &fakeDukcapil{})
		assert.Equal(t, apperror.ErrNotFound, codeOf(t, uc.Delete(context.Background(), "u1", "m1")))
	})
	t.Run("repo error → INTERNAL", func(t *testing.T) {
		uc := NewMemberUsecase(&stubMemberRepo{deleteErr: errors.New("db")}, &fakeDukcapil{})
		assert.Equal(t, apperror.ErrInternal, codeOf(t, uc.Delete(context.Background(), "u1", "m1")))
	})
}

// ----------------------------------------------------------------- History

type stubHistoryRepo struct {
	items     []dto.TicketHistoryItem
	total     int
	err       error
	gotLimit  int
	gotOffset int
}

func (s *stubHistoryRepo) ListByUser(_ context.Context, _ string, limit, offset int) ([]dto.TicketHistoryItem, int, error) {
	s.gotLimit, s.gotOffset = limit, offset
	return s.items, s.total, s.err
}

func TestHistoryList_ShouldNormalisePaging(t *testing.T) {
	cases := []struct {
		name                        string
		page, size                  int
		wantPage, wantSize, wantOff int
	}{
		{"normal", 2, 10, 2, 10, 10},
		{"page < 1 → 1", 0, 10, 1, 10, 0},
		{"page negatif → 1", -5, 10, 1, 10, 0},
		{"size < 1 → default 20", 1, 0, 1, 20, 0},
		{"size > max → 100", 1, 500, 1, 100, 0},
		{"offset halaman 3 size 20", 3, 20, 3, 20, 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubHistoryRepo{total: 7}
			uc := NewHistoryUsecase(repo)

			res, err := uc.List(context.Background(), "u1", tc.page, tc.size)

			require.NoError(t, err)
			assert.Equal(t, tc.wantPage, res.Page)
			assert.Equal(t, tc.wantSize, res.PageSize)
			assert.Equal(t, tc.wantSize, repo.gotLimit)
			assert.Equal(t, tc.wantOff, repo.gotOffset)
			assert.Equal(t, 7, res.Total)
		})
	}
}

func TestHistoryList_ShouldReturnInternal_WhenRepoFails(t *testing.T) {
	uc := NewHistoryUsecase(&stubHistoryRepo{err: errors.New("db")})

	_, err := uc.List(context.Background(), "u1", 1, 10)

	assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
}
