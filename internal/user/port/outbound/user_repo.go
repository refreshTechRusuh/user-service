// Package outbound berisi driven port: kontrak yang DIBUTUHKAN usecase dari
// dunia luar (penyimpanan data). Usecase bergantung ke interface ini, bukan
// implementasi konkret (pola event-service: prefix I, context, return error).
package outbound

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
)

// IUserRepository mengelola persistensi akun (tabel app_user).
type IUserRepository interface {
	// Create menyimpan user baru. Bila email sudah ada (pelanggaran UNIQUE),
	// implementasi mengembalikan apperror.EmailTaken().
	Create(ctx context.Context, u entity.User) error

	// FindByEmail mencari user by email.
	//   - ketemu        -> (user, true, nil)
	//   - tidak ada     -> (zero, false, nil)   [bukan error]
	//   - error teknis  -> (zero, false, err)
	// Dipakai untuk cek email sebelum register, dan untuk login nanti.
	FindByEmail(ctx context.Context, email string) (user entity.User, found bool, err error)
}
