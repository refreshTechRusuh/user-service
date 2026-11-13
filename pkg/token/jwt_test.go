package token

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthToken_ShouldCarryRole_AndVerifyWithSameSecret(t *testing.T) {
	g := NewGenerator("user-secret", "refresh-secret", time.Hour, 24*time.Hour)
	for _, role := range []string{"customer", "admin"} {
		tok, err := g.GenerateAuthToken("user-1", role, "sid-1")
		if err != nil {
			t.Fatal(err)
		}
		c, err := g.VerifyAuthToken(tok)
		if err != nil {
			t.Fatalf("verify %s: %v", role, err)
		}
		if c.Subject != "user-1" || c.Role != role || c.SessionID != "sid-1" {
			t.Errorf("claim = sub %q role %q sid %q; mau user-1 %s sid-1", c.Subject, c.Role, c.SessionID, role)
		}
	}
}

func TestRefreshToken_ShouldVerifyOnlyWithRefreshSecret(t *testing.T) {
	g := NewGenerator("user-secret", "refresh-secret", time.Hour, 24*time.Hour)
	refresh, err := g.GenerateRefreshToken("user-1", "sid-1")
	if err != nil {
		t.Fatal(err)
	}

	c, err := g.VerifyRefreshToken(refresh)
	if err != nil || c.Subject != "user-1" || c.Role != "" || c.SessionID != "sid-1" {
		t.Fatalf("claims=%+v err=%v; mau sub user-1 tanpa role", c, err)
	}
	if _, err := g.VerifyAuthToken(refresh); err == nil {
		t.Error("refresh token tidak boleh lolos sebagai authToken")
	}
}

func TestAccessTTLSeconds_ShouldReturnConfiguredTTL(t *testing.T) {
	g := NewGenerator("a", "b", 90*time.Minute, time.Hour)

	if got := g.AccessTTLSeconds(); got != 5400 {
		t.Errorf("AccessTTLSeconds = %d; mau 5400", got)
	}
}

// Token yang ditandatangani secret lain (mis. Access Token antrian dengan
// ACCESS_TOKEN_SECRET) harus DITOLAK — inilah gunanya memisahkan secret.
func TestVerifyAuthToken_ShouldReject_WhenSignedWithOtherSecret(t *testing.T) {
	g := NewGenerator("user-secret", "refresh-secret", time.Hour, 24*time.Hour)

	queueLike := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "q_123", "role": "admin", "exp": time.Now().Add(time.Hour).Unix(),
	})
	forged, _ := queueLike.SignedString([]byte("queue-access-token-secret"))

	if _, err := g.VerifyAuthToken(forged); err == nil {
		t.Fatal("token dengan secret lain diterima; mau ditolak")
	}
}

// Refresh token tidak boleh dipakai sebagai authToken (secret berbeda).
func TestVerifyAuthToken_ShouldReject_RefreshToken(t *testing.T) {
	g := NewGenerator("user-secret", "refresh-secret", time.Hour, 24*time.Hour)
	rt, _ := g.GenerateRefreshToken("user-1", "sid-1")
	if _, err := g.VerifyAuthToken(rt); err == nil {
		t.Fatal("refresh token diterima sebagai authToken")
	}
}

// Token dengan alg "none" (tanpa tanda tangan) harus ditolak.
func TestVerifyAuthToken_ShouldReject_AlgNone(t *testing.T) {
	g := NewGenerator("user-secret", "refresh-secret", time.Hour, 24*time.Hour)
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": "user-1", "role": "admin", "exp": time.Now().Add(time.Hour).Unix(),
	})
	s, _ := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := g.VerifyAuthToken(s); err == nil {
		t.Fatal("token alg=none diterima")
	}
}

func TestRefreshTTL_ShouldReturnConfiguredTTL(t *testing.T) {
	g := NewGenerator("a", "b", time.Hour, 36*time.Hour)
	if got := g.RefreshTTL(); got != 36*time.Hour {
		t.Errorf("RefreshTTL = %v; mau 36h", got)
	}
}

func TestSessionHelpers_ShouldGenerateUniqueIDs_AndPrefixRevokeID(t *testing.T) {
	a, b := NewSessionID(), NewSessionID()
	if a == "" || a == b {
		t.Errorf("session id harus unik dan tidak kosong: %q %q", a, b)
	}
	if got := SessionRevokeID("abc"); got != "sid:abc" {
		t.Errorf("SessionRevokeID = %q; mau sid:abc", got)
	}
}
