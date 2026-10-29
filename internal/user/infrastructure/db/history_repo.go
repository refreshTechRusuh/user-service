// Package db: read-only query riwayat booking milik user (lintas tabel BE-2).
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
)

type HistoryRepository struct {
	pool *pgxpool.Pool
}

func NewHistoryRepository(pool *pgxpool.Pool) *HistoryRepository {
	return &HistoryRepository{pool: pool}
}

// ListByUser: join booking + event + agregat booking_item.
// Hanya booking milik user (user_id = $1); booking guest (user_id NULL) tak muncul.
func (r *HistoryRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]dto.TicketHistoryItem, int, error) {
	// total dulu (untuk info paginasi).
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM booking WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count history: %w", err)
	}

	const q = `
		SELECT b.id,
		       e.name,
		       b.status,
		       COALESCE(COUNT(bi.id), 0)             AS tickets,
		       COALESCE(SUM(bi.price_snapshot), 0)   AS total,
		       to_char(b.created_at, 'YYYY-MM-DD"T"HH24:MI:SSOF') AS created_at
		FROM booking b
		JOIN event e ON e.id = b.event_id
		LEFT JOIN booking_item bi ON bi.booking_id = b.id
		WHERE b.user_id = $1
		GROUP BY b.id, e.name, b.status, b.created_at
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	items := make([]dto.TicketHistoryItem, 0, limit)
	for rows.Next() {
		var it dto.TicketHistoryItem
		if err := rows.Scan(&it.BookingID, &it.EventName, &it.Status, &it.Tickets, &it.Total, &it.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan history: %w", err)
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}
