package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_ShouldPostAllFourFields_AndReturnValid(t *testing.T) {
	var gotPath, gotCT string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotCT = r.URL.Path, r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"status":"00000","data":{"valid":true}}`))
	}))
	defer srv.Close()
	c := NewDukcapilClient(srv.URL, time.Second)

	valid, err := c.Verify(context.Background(), "3273000000000001", "Budi", "1990-01-01", "Siti")

	require.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, "/api/dukcapil/v1/verify", gotPath)
	assert.Equal(t, "application/json", gotCT)
	assert.Equal(t, map[string]string{
		"nik": "3273000000000001", "nama": "Budi", "tglLahir": "1990-01-01", "namaIbu": "Siti",
	}, gotBody)
}

func TestVerify_ShouldReturnFalse_WhenDukcapilSaysNotMatching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"00000","data":{"valid":false}}`))
	}))
	defer srv.Close()

	valid, err := NewDukcapilClient(srv.URL, time.Second).Verify(context.Background(), "n", "n", "t", "i")

	require.NoError(t, err)
	assert.False(t, valid)
}

func TestVerify_ShouldReturnError_WhenStatusNot200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := NewDukcapilClient(srv.URL, time.Second).Verify(context.Background(), "n", "n", "t", "i")

	assert.ErrorContains(t, err, "502")
}

func TestVerify_ShouldReturnError_WhenBodyNotJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>bukan json</html>`))
	}))
	defer srv.Close()

	_, err := NewDukcapilClient(srv.URL, time.Second).Verify(context.Background(), "n", "n", "t", "i")

	assert.ErrorContains(t, err, "decode")
}

func TestVerify_ShouldReturnError_WhenServerUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // port sudah tertutup

	_, err := NewDukcapilClient(url, time.Second).Verify(context.Background(), "n", "n", "t", "i")

	assert.ErrorContains(t, err, "dukcapil call")
}

func TestVerify_ShouldTimeout_WhenDukcapilTooSlow(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	_, err := NewDukcapilClient(srv.URL, 100*time.Millisecond).Verify(context.Background(), "n", "n", "t", "i")

	assert.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second, "harus gagal cepat, bukan menggantung")
}

func TestVerify_ShouldReturnError_WhenBaseURLInvalid(t *testing.T) {
	_, err := NewDukcapilClient("http://bad host\x7f", time.Second).Verify(context.Background(), "n", "n", "t", "i")

	assert.Error(t, err)
}

func TestVerify_ShouldStopEarly_WhenContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"valid":true}}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewDukcapilClient(srv.URL, time.Second).Verify(ctx, "n", "n", "t", "i")

	assert.Error(t, err)
}
