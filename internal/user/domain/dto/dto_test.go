package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterRequest_Validate(t *testing.T) {
	cases := []struct {
		name     string
		req      RegisterRequest
		wantErrs int
	}{
		{"valid", RegisterRequest{Email: "a@b.com", Password: "12345678"}, 0},
		{"email kosong", RegisterRequest{Email: "  ", Password: "12345678"}, 1},
		{"email tanpa @", RegisterRequest{Email: "ab.com", Password: "12345678"}, 1},
		{"email tanpa titik", RegisterRequest{Email: "a@bcom", Password: "12345678"}, 1},
		{"password pendek", RegisterRequest{Email: "a@b.com", Password: "1234567"}, 1},
		{"semua salah", RegisterRequest{}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Len(t, tc.req.Validate(), tc.wantErrs)
		})
	}
}

func TestLoginRequest_Validate(t *testing.T) {
	assert.Empty(t, LoginRequest{Email: "a@b.com", Password: "x"}.Validate())
	assert.Len(t, LoginRequest{Email: " ", Password: "x"}.Validate(), 1)
	assert.Len(t, LoginRequest{Email: "a@b.com"}.Validate(), 1)
	assert.Len(t, LoginRequest{}.Validate(), 2)
}

func TestMemberRequest_Validate(t *testing.T) {
	ok := MemberRequest{NIK: "3273000000000001", Nama: "Budi", TglLahir: "1990-01-01", NamaIbu: "Siti"}
	cases := []struct {
		name     string
		mutate   func(r *MemberRequest)
		wantErrs int
	}{
		{"valid", func(r *MemberRequest) {}, 0},
		{"nik 15 digit", func(r *MemberRequest) { r.NIK = "327300000000001" }, 1},
		{"nik 17 digit", func(r *MemberRequest) { r.NIK = "32730000000000011" }, 1},
		{"nik huruf", func(r *MemberRequest) { r.NIK = "abcdefghijklmnop" }, 1},
		{"nik dengan spasi di tepi masih valid", func(r *MemberRequest) { r.NIK = " 3273000000000001 " }, 0},
		{"nama kosong", func(r *MemberRequest) { r.Nama = " " }, 1},
		{"tglLahir kosong", func(r *MemberRequest) { r.TglLahir = "" }, 1},
		{"namaIbu kosong", func(r *MemberRequest) { r.NamaIbu = "" }, 1},
		{"semua kosong", func(r *MemberRequest) { *r = MemberRequest{} }, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := ok
			tc.mutate(&req)
			assert.Len(t, req.Validate(), tc.wantErrs)
		})
	}
}

func TestVerifyNIKRequest_Validate_ShouldFollowMemberRules(t *testing.T) {
	assert.Empty(t, VerifyNIKRequest{NIK: "3273000000000001", Nama: "B", TglLahir: "1990-01-01", NamaIbu: "S"}.Validate())
	assert.Len(t, VerifyNIKRequest{}.Validate(), 4)
}
