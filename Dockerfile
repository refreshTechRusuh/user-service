# ---- Tahap 1: builder ----
# Pakai image Go lengkap (ada compiler) untuk meng-compile binary.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy manifest dulu + download deps → layer ini ter-cache selama go.mod/go.sum
# tidak berubah, jadi build ulang lebih cepat saat cuma kode yang berubah.
COPY go.mod go.sum ./
RUN go mod download

# Copy sisa source lalu compile.
# CGO_ENABLED=0 → binary statis (tak butuh libc), cocok untuk image distroless.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/api ./cmd

# ---- Tahap 2: runtime ----
# distroless/static = image super kecil, cuma berisi binary kita (tanpa shell,
# tanpa package manager) → permukaan serang kecil. :nonroot = jalan sbg user biasa.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /app/bin/api .
EXPOSE 8086
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
