package usecase

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/port/outbound"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type HistoryUsecase struct {
	repo outbound.IHistoryRepository
}

func NewHistoryUsecase(repo outbound.IHistoryRepository) *HistoryUsecase {
	return &HistoryUsecase{repo: repo}
}

// List menghitung limit/offset dari page/pageSize lalu memanggil repo.
func (u *HistoryUsecase) List(ctx context.Context, userID string, page, pageSize int) (dto.TicketHistoryResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset := (page - 1) * pageSize

	items, total, err := u.repo.ListByUser(ctx, userID, pageSize, offset)
	if err != nil {
		return dto.TicketHistoryResponse{}, apperror.Internal(err)
	}
	return dto.TicketHistoryResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}
