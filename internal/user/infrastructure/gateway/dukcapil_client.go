// Package gateway (infrastructure) = adapter untuk memanggil service LAIN via HTTP.
// Di sini: client ke dukcapil-mock untuk validasi NIK. Mengimplementasikan
// outbound.IDukcapilClient. Satu-satunya tempat yang tahu URL & bentuk HTTP dukcapil.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// DukcapilClient memanggil POST {baseURL}/api/dukcapil/v1/verify.
type DukcapilClient struct {
	baseURL string
	http    *http.Client
}

// NewDukcapilClient membuat client dengan timeout (fail cepat bila dukcapil lambat).
func NewDukcapilClient(baseURL string, timeout time.Duration) *DukcapilClient {
	return &DukcapilClient{
		baseURL: baseURL,
		// otelhttp.NewTransport: membuat span klien & menyisipkan header traceparent,
		// sehingga trace user-service -> dukcapil-mock tersambung di Cloud Trace.
		// Tanpa tracer provider (OTEL_ENABLED=false) ia menjadi no-op.
		http: &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

// bentuk request/response dukcapil (lokal — tak bocor ke domain).
type verifyReq struct {
	NIK      string `json:"nik"`
	Nama     string `json:"nama"`
	TglLahir string `json:"tglLahir"`
	NamaIbu  string `json:"namaIbu"`
}

type verifyResp struct {
	Status string `json:"status"`
	Data   struct {
		Valid bool `json:"valid"`
	} `json:"data"`
}

// Verify memanggil dukcapil. Error teknis (timeout, dukcapil mati) dikembalikan
// sebagai error → usecase memutuskan (fail-closed: tolak simpan).
func (c *DukcapilClient) Verify(ctx context.Context, nik, nama, tglLahir, namaIbu string) (bool, error) {
	body, err := json.Marshal(verifyReq{NIK: nik, Nama: nama, TglLahir: tglLahir, NamaIbu: namaIbu})
	if err != nil {
		return false, fmt.Errorf("dukcapil marshal: %w", err)
	}

	url := c.baseURL + "/api/dukcapil/v1/verify"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("dukcapil new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("dukcapil call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("dukcapil status %d", resp.StatusCode)
	}

	var out verifyResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("dukcapil decode: %w", err)
	}
	return out.Data.Valid, nil
}
