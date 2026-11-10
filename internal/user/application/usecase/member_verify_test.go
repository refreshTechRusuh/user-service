package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// fakeDukcapil = IDukcapilClient di memori; merekam argumen terakhir.
type fakeDukcapil struct {
	valid bool
	err   error
	got   [4]string
}

func (f *fakeDukcapil) Verify(_ context.Context, nik, nama, tglLahir, namaIbu string) (bool, error) {
	f.got = [4]string{nik, nama, tglLahir, namaIbu}
	return f.valid, f.err
}

func TestVerifyNIK_ShouldReturnValidTrue_WhenDukcapilMatches(t *testing.T) {
	d := &fakeDukcapil{valid: true}
	uc := NewMemberUsecase(nil, d)

	res, err := uc.VerifyNIK(context.Background(), dto.VerifyNIKRequest{
		NIK: " 3273000000000001 ", Nama: " Budi ", TglLahir: "1990-01-01", NamaIbu: "Siti",
	})

	if err != nil || !res.Valid {
		t.Fatalf("res=%+v err=%v; mau Valid=true tanpa error", res, err)
	}
	if d.got != [4]string{"3273000000000001", "Budi", "1990-01-01", "Siti"} {
		t.Errorf("argumen ke dukcapil = %v; spasi harus dipangkas", d.got)
	}
}

func TestVerifyNIK_ShouldReturnValidFalse_WhenDukcapilDoesNotMatch(t *testing.T) {
	uc := NewMemberUsecase(nil, &fakeDukcapil{valid: false})

	res, err := uc.VerifyNIK(context.Background(), dto.VerifyNIKRequest{NIK: "3273000000000001"})

	if err != nil || res.Valid {
		t.Fatalf("res=%+v err=%v; mau Valid=false tanpa error", res, err)
	}
}

func TestVerifyNIK_ShouldReturnUnavailable_WhenDukcapilFails(t *testing.T) {
	uc := NewMemberUsecase(nil, &fakeDukcapil{err: errors.New("timeout")})

	_, err := uc.VerifyNIK(context.Background(), dto.VerifyNIKRequest{NIK: "3273000000000001"})

	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperror.ErrDukcapilUnavailable {
		t.Fatalf("err=%v; mau DUKCAPIL_UNAVAILABLE (bukan NIK_INVALID)", err)
	}
}
