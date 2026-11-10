package usecase

import (
	"context"
	"testing"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
)

// fakeMemberRepo = IMemberRepository di memori untuk test.
type fakeMemberRepo struct {
	members []entity.MemberNIK
}

func (f *fakeMemberRepo) Create(_ context.Context, m entity.MemberNIK) error {
	f.members = append(f.members, m)
	return nil
}

func (f *fakeMemberRepo) ListByUser(_ context.Context, _ string) ([]entity.MemberNIK, error) {
	return f.members, nil
}

func (f *fakeMemberRepo) Update(_ context.Context, _ entity.MemberNIK) error { return nil }

func (f *fakeMemberRepo) Delete(_ context.Context, _, _ string) (bool, error) { return true, nil }

var wantMember = dto.MemberResponse{
	MemberID: "m1",
	NIK:      "3273000000000001",
	Nama:     "Budi",
	TglLahir: "1990-01-01",
	NamaIbu:  "Siti",
}

func TestList_ShouldIncludeTglLahirAndNamaIbu_WhenMembersExist(t *testing.T) {
	repo := &fakeMemberRepo{members: []entity.MemberNIK{{
		ID: "m1", UserID: "u1", NIK: "3273000000000001", Nama: "Budi",
		TglLahir: "1990-01-01", NamaIbu: "Siti",
	}}}
	uc := NewMemberUsecase(repo, &fakeDukcapil{valid: true})

	res, err := uc.List(context.Background(), "u1")

	if err != nil || len(res.Items) != 1 {
		t.Fatalf("res=%+v err=%v; mau 1 item tanpa error", res, err)
	}
	if res.Items[0] != wantMember {
		t.Errorf("item = %+v; mau %+v", res.Items[0], wantMember)
	}
}

func TestCreate_ShouldReturnTglLahirAndNamaIbu_WhenDukcapilValid(t *testing.T) {
	uc := NewMemberUsecase(&fakeMemberRepo{}, &fakeDukcapil{valid: true})

	res, err := uc.Create(context.Background(), "u1", dto.MemberRequest{
		NIK: "3273000000000001", Nama: "Budi", TglLahir: "1990-01-01", NamaIbu: "Siti",
	})

	if err != nil {
		t.Fatalf("err=%v", err)
	}
	// MemberID dibuat server (UUID); sisanya harus sama dengan input.
	res.MemberID = "m1"
	if res != wantMember {
		t.Errorf("res = %+v; mau %+v", res, wantMember)
	}
}

func TestUpdate_ShouldReturnTglLahirAndNamaIbu_WhenDukcapilValid(t *testing.T) {
	uc := NewMemberUsecase(&fakeMemberRepo{}, &fakeDukcapil{valid: true})

	res, err := uc.Update(context.Background(), "u1", "m1", dto.MemberRequest{
		NIK: "3273000000000001", Nama: "Budi", TglLahir: "1990-01-01", NamaIbu: "Siti",
	})

	if err != nil || res != wantMember {
		t.Fatalf("res=%+v err=%v; mau %+v", res, err, wantMember)
	}
}
