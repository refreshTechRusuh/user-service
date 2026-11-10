package dto

import (
	"regexp"
	"strings"
)

var nikPattern = regexp.MustCompile(`^\d{16}$`)

// MemberRequest = body untuk create/update member (dicocokkan ke Dukcapil).
type MemberRequest struct {
	NIK      string `json:"nik"      validate:"required,len=16,numeric"`
	Nama     string `json:"nama"     validate:"required"`
	TglLahir string `json:"tglLahir" validate:"required"`
	NamaIbu  string `json:"namaIbu"  validate:"required"`
}

// Validate memeriksa bentuk request di boundary (sebelum ke Dukcapil).
func (r MemberRequest) Validate() []string {
	var errs []string
	if !nikPattern.MatchString(strings.TrimSpace(r.NIK)) {
		errs = append(errs, "NIK harus 16 digit angka")
	}
	if strings.TrimSpace(r.Nama) == "" {
		errs = append(errs, "nama wajib diisi")
	}
	if strings.TrimSpace(r.TglLahir) == "" {
		errs = append(errs, "tglLahir wajib diisi")
	}
	if strings.TrimSpace(r.NamaIbu) == "" {
		errs = append(errs, "namaIbu wajib diisi")
	}
	return errs
}

// VerifyNIKRequest = body POST /api/user/v1/nik/verify: cek pencocokan ke Dukcapil
// TANPA menyimpan apa pun. Publik (guest boleh), jadi dilindungi rate limit.
type VerifyNIKRequest struct {
	NIK      string `json:"nik"`
	Nama     string `json:"nama"`
	TglLahir string `json:"tglLahir"`
	NamaIbu  string `json:"namaIbu"`
}

// Validate memeriksa bentuk request (aturan sama dengan MemberRequest).
func (r VerifyNIKRequest) Validate() []string {
	return MemberRequest(r).Validate()
}

// VerifyNIKResponse = hasil pencocokan. Valid=false berarti data tidak cocok.
type VerifyNIKResponse struct {
	Valid bool `json:"valid"`
}

// MemberResponse = payload satu member (isi field data).
// TglLahir (YYYY-MM-DD) dan NamaIbu ikut dikembalikan karena FE memakainya untuk
// mengisi data penonton saat booking (dicocokkan ke Dukcapil). Hanya pemilik akun
// yang bisa membacanya (endpoint member butuh login dan di-scope per user).
type MemberResponse struct {
	MemberID string `json:"memberId"`
	NIK      string `json:"nik"`
	Nama     string `json:"nama"`
	TglLahir string `json:"tglLahir"`
	NamaIbu  string `json:"namaIbu"`
}

// MemberListResponse = daftar member milik user.
type MemberListResponse struct {
	Items []MemberResponse `json:"items"`
}
