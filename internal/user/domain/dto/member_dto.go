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

// MemberResponse = payload satu member (isi field data).
type MemberResponse struct {
	MemberID string `json:"memberId"`
	NIK      string `json:"nik"`
	Nama     string `json:"nama"`
}

// MemberListResponse = daftar member milik user.
type MemberListResponse struct {
	Items []MemberResponse `json:"items"`
}
