package inbound

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
)

// INIKVerifier = kontrak cek pencocokan NIK ke Dukcapil (tanpa menyimpan data).
// Dipanggil FE lewat user-service supaya FE tidak langsung ke dukcapil-mock.
type INIKVerifier interface {
	// VerifyNIK mengembalikan Valid=true/false bila Dukcapil menjawab. Bila Dukcapil
	// tak terjangkau → error DUKCAPIL_UNAVAILABLE (BUKAN Valid=false).
	VerifyNIK(ctx context.Context, req dto.VerifyNIKRequest) (dto.VerifyNIKResponse, error)
}
