package dto

// TicketHistoryItem = satu baris riwayat booking milik user.
type TicketHistoryItem struct {
	BookingID string `json:"bookingId"`
	EventName string `json:"eventName"`
	Status    string `json:"status"`
	Tickets   int    `json:"tickets"`
	Total     int64  `json:"total"`
	CreatedAt string `json:"createdAt"`
}

// TicketHistoryResponse = daftar riwayat + info paginasi.
type TicketHistoryResponse struct {
	Items    []TicketHistoryItem `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Total    int                 `json:"total"`
}
