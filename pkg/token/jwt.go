// Package token membuat & memverifikasi JWT (JSON Web Token) untuk auth user.
// JWT = token bertanda tangan: isinya (claims) bisa diverifikasi siapa pun yang
// punya secret, TANPA query DB. Ini yang bikin token "self-verifiable".
//
// Struktur JWT: <header>.<payload>.<signature> (base64). Signature = HMAC dari
// header+payload pakai secret. Kalau isi diubah, signature tak cocok → ditolak.
package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims = data yang kita taruh di dalam JWT.
// RegisteredClaims menyediakan field standar (sub, exp, iat) dari spesifikasi JWT.
//
// SessionID (sid) mengikat authToken dan refreshToken ke SATU sesi login.
// Logout mencabut sid, sehingga kedua token sesi itu ikut tidak berlaku.
// Token lama (sebelum ada sid) punya SessionID kosong dan tidak bisa dicabut per sesi.
type Claims struct {
	Role      string `json:"role"`
	SessionID string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

// NewSessionID membuat id sesi baru (satu per login).
func NewSessionID() string { return uuid.NewString() }

// SessionRevokeID = id yang disimpan di denylist untuk sebuah sesi.
// Diberi prefix agar tidak tertukar dengan denylist jenis lain.
func SessionRevokeID(sessionID string) string { return "sid:" + sessionID }

// Generator membuat token memakai secret & masa berlaku dari config.
// Dipakai usecase lewat interface/konkret; secret tak pernah keluar dari sini.
type Generator struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// NewGenerator membuat Generator. Secret & TTL berasal dari config (JWTConfig).
func NewGenerator(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *Generator {
	return &Generator{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

// GenerateAuthToken membuat access token (authToken) untuk userID + role.
// Claim: sub=userID, role, sid, exp=now+accessTTL, iat=now.
func (g *Generator) GenerateAuthToken(userID, role, sessionID string) (string, error) {
	return g.sign(userID, role, sessionID, g.accessSecret, g.accessTTL)
}

// GenerateRefreshToken membuat refresh token (TTL panjang) untuk userID.
// Role tidak dibawa di refresh token (cuma untuk menerbitkan ulang authToken;
// role dibaca ulang dari DB saat refresh). sid sama dengan authToken sesi itu.
func (g *Generator) GenerateRefreshToken(userID, sessionID string) (string, error) {
	return g.sign(userID, "", sessionID, g.refreshSecret, g.refreshTTL)
}

// RefreshTTL mengembalikan masa berlaku refreshToken (umur maksimum sebuah sesi).
// Dipakai sebagai TTL denylist sesi saat logout.
func (g *Generator) RefreshTTL() time.Duration { return g.refreshTTL }

// AccessTTLSeconds mengembalikan masa berlaku authToken dalam detik
// (dipakai untuk field expiresIn di response login).
func (g *Generator) AccessTTLSeconds() int {
	return int(g.accessTTL.Seconds())
}

// VerifyAuthToken memeriksa authToken: signature valid + belum kedaluwarsa.
// Mengembalikan claims (sub=userID, role) bila sah; error bila tidak.
func (g *Generator) VerifyAuthToken(tokenStr string) (*Claims, error) {
	return parse(tokenStr, g.accessSecret)
}

// VerifyRefreshToken sama, tapi pakai secret refresh.
func (g *Generator) VerifyRefreshToken(tokenStr string) (*Claims, error) {
	return parse(tokenStr, g.refreshSecret)
}

// sign adalah inti pembuatan JWT: susun claims -> tandatangani dengan HMAC-SHA256.
func (g *Generator) sign(userID, role, sessionID string, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,                           // sub = id user
			IssuedAt:  jwt.NewNumericDate(now),          // iat = kapan dibuat
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)), // exp = kapan kedaluwarsa
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(secret)
}

// parse membongkar JWT & memverifikasinya. Dua hal diperiksa:
//  1. metode sign HARUS HMAC (cegah serangan "alg: none" / RS256 palsu)
//  2. signature cocok dengan secret + exp belum lewat (dicek otomatis library)
func parse(tokenStr string, secret []byte) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
