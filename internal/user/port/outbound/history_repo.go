package outbound

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
)

// IHistoryRepository = read-only query riwayat booking milik user.
// CATATAN: membaca tabel booking/booking_item/event milik BE-2 (DB bersama).
// Sementara via query langsung; bisa diganti endpoint internal Booking nanti.
type IHistoryRepository interface {
	// ListByUser mengembalikan item riwayat (paginasi) + total jumlah booking user.
	ListByUser(ctx context.Context, userID string, limit, offset int) (items []dto.TicketHistoryItem, total int, err error)
}
