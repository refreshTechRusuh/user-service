package entity

import "time"

// MemberNIK = satu NIK yang disimpan user login (mis. anggota keluarga).
// Dicocokkan ke Dukcapil saat disimpan.
type MemberNIK struct {
	ID        string
	UserID    string
	NIK       string
	Nama      string
	TglLahir  string // YYYY-MM-DD
	NamaIbu   string
	CreatedAt time.Time
}
