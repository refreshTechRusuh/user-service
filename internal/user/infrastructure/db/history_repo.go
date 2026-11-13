// Package db: read-only query riwayat booking milik user (lintas tabel BE-2).
package db

import (
	"context"
	"fmt"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
)

type HistoryRepository struct {
	pool Querier
}

func NewHistoryRepository(pool Querier) *HistoryRepository {
	return &HistoryRepository{pool: pool}
}

// ListByUser: join booking + event + agregat booking_item, lalu rincian tiket per booking.
// Hanya booking milik user (user_id = $1); booking guest (user_id NULL) tak muncul.
func (r *HistoryRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]dto.TicketHistoryItem, int, error) {
	// total dulu (untuk info paginasi).
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM booking WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count history: %w", err)
	}

	items, err := r.listBookings(ctx, userID, limit, offset)
	if err != nil || len(items) == 0 {
		return items, total, err
	}
	if err := r.attachTicketDetails(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// listBookings: satu baris per booking. Metode bayar = payment terbaru milik booking itu.
func (r *HistoryRepository) listBookings(ctx context.Context, userID string, limit, offset int) ([]dto.TicketHistoryItem, error) {
	const q = `
		SELECT b.id,
		       e.name,
		       e.poster_url,
		       b.status,
		       COALESCE(pm.method, '')               AS payment_method,
		       COALESCE(COUNT(bi.id), 0)             AS tickets,
		       COALESCE(SUM(bi.price_snapshot), 0)   AS total,
		       to_char(b.created_at, 'YYYY-MM-DD"T"HH24:MI:SSOF') AS created_at
		FROM booking b
		JOIN event e ON e.id = b.event_id
		LEFT JOIN booking_item bi ON bi.booking_id = b.id
		LEFT JOIN LATERAL (
		       SELECT p.method FROM payment p
		       WHERE p.booking_id = b.id
		       ORDER BY p.created_at DESC LIMIT 1
		) pm ON TRUE
		WHERE b.user_id = $1
		GROUP BY b.id, e.name, e.poster_url, b.status, pm.method, b.created_at
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	items := make([]dto.TicketHistoryItem, 0, limit)
	for rows.Next() {
		var it dto.TicketHistoryItem
		if err := rows.Scan(&it.BookingID, &it.EventName, &it.PosterURL, &it.Status,
			&it.PaymentMethod, &it.Tickets, &it.Total, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		it.TicketDetails = []dto.TicketDetail{}
		items = append(items, it)
	}
	return items, rows.Err()
}

// attachTicketDetails: satu query untuk semua booking di halaman ini (hindari N+1).
func (r *HistoryRepository) attachTicketDetails(ctx context.Context, items []dto.TicketHistoryItem) error {
	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for i, it := range items {
		ids[i] = it.BookingID
		index[it.BookingID] = i
	}

	const q = `
		SELECT bi.booking_id::text,
		       bi.id::text,
		       bi.nik,
		       COALESCE(tc.name, '') AS category_name,
		       bi.price_snapshot
		FROM booking_item bi
		LEFT JOIN ticket_category tc ON tc.id = bi.category_id
		WHERE bi.booking_id = ANY($1::uuid[])
		ORDER BY bi.booking_id, tc.name, bi.nik`

	rows, err := r.pool.Query(ctx, q, ids)
	if err != nil {
		return fmt.Errorf("query ticket details: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var bookingID string
		var d dto.TicketDetail
		if err := rows.Scan(&bookingID, &d.TicketID, &d.NIK, &d.CategoryName, &d.Price); err != nil {
			return fmt.Errorf("scan ticket details: %w", err)
		}
		if i, ok := index[bookingID]; ok {
			items[i].TicketDetails = append(items[i].TicketDetails, d)
		}
	}
	return rows.Err()
}
