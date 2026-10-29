package inbound

import (
	"context"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
)

// IMemberUsecase = kontrak member/NIK yang dipanggil handler. Semua scoped per user.
type IMemberUsecase interface {
	Create(ctx context.Context, userID string, req dto.MemberRequest) (dto.MemberResponse, error)
	List(ctx context.Context, userID string) (dto.MemberListResponse, error)
	Update(ctx context.Context, userID, memberID string, req dto.MemberRequest) (dto.MemberResponse, error)
	Delete(ctx context.Context, userID, memberID string) error
}
