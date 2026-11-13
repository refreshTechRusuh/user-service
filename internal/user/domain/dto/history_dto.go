package dto

// TicketHistoryItem = satu baris riwayat booking milik user.
type TicketHistoryItem struct {
	BookingID     string `json:"bookingId"`
	EventName     string `json:"eventName"`
	PosterURL     string `json:"posterUrl"`
	Status        string `json:"status"`
	PaymentMethod string `json:"paymentMethod"` // payment_type Midtrans; "" bila belum bayar
	Tickets       int    `json:"tickets"`       // jumlah tiket (dipertahankan untuk kompatibilitas FE)
	Total         int64  `json:"total"`
	CreatedAt     string `json:"createdAt"`
	// TicketDetails = rincian per tiket (1 tiket = 1 NIK penonton), urut kategori lalu NIK.
	TicketDetails []TicketDetail `json:"ticketDetails"`
}

// TicketDetail = satu tiket di dalam booking.
type TicketDetail struct {
	TicketID     string `json:"ticketId"` // id booking_item = nomor e-tiket
	NIK          string `json:"nik"`
	CategoryName string `json:"categoryName"`
	Price        int64  `json:"price"`
}

// TicketHistoryResponse = daftar riwayat + info paginasi.
type TicketHistoryResponse struct {
	Items    []TicketHistoryItem `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Total    int                 `json:"total"`
}
