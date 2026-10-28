package outbound

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
)

// IMemberRepository mengelola persistensi member_nik (scoped per user).
type IMemberRepository interface {
	// Create menyimpan member baru. NIK dobel untuk user sama (UNIQUE) -> error khusus.
	Create(ctx context.Context, m entity.MemberNIK) error

	// ListByUser mengembalikan semua member milik satu user.
	ListByUser(ctx context.Context, userID string) ([]entity.MemberNIK, error)

	// Update mengubah member milik user tertentu (scoping: id DAN user_id harus cocok).
	Update(ctx context.Context, m entity.MemberNIK) error

	// Delete menghapus member milik user tertentu. found=false bila bukan milik user.
	Delete(ctx context.Context, userID, id string) (found bool, err error)
}

// IDukcapilClient memanggil dukcapil-mock untuk validasi identitas.
type IDukcapilClient interface {
	// Verify mengembalikan true bila {nik,nama,tglLahir,namaIbu} cocok di Dukcapil.
	Verify(ctx context.Context, nik, nama, tglLahir, namaIbu string) (valid bool, err error)
}
