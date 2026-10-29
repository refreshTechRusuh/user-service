package inbound

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
)

// IHistoryUsecase = riwayat tiket milik user login (paginasi).
type IHistoryUsecase interface {
	List(ctx context.Context, userID string, page, pageSize int) (dto.TicketHistoryResponse, error)
}
