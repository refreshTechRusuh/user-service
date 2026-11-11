package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
	"github.com/refreshTechRusuh/user-service/pkg/middleware"
	"github.com/refreshTechRusuh/user-service/pkg/token"
)

// ---------------------------------------------------------------- helpers

var gen = token.NewGenerator("access-secret", "refresh-secret", time.Hour, 24*time.Hour)

// authed membungkus handler dengan RequireAuth supaya userId masuk ke context.
func authed(h http.HandlerFunc) http.Handler {
	return middleware.RequireAuth(gen)(h)
}

func call(t *testing.T, h http.Handler, method, target, body string, withAuth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if withAuth {
		tok, err := gen.GenerateAuthToken("u1", "customer")
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func env(t *testing.T, rr *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &e))
	return e
}

// ------------------------------------------------------------------ stubs

type stubUserUC struct {
	register func(dto.RegisterRequest) (dto.RegisterResponse, error)
	login    func(dto.LoginRequest) (dto.LoginResponse, error)
	me       func(string) (dto.MeResponse, error)
	logout   func(dto.LogoutRequest) error
}

func (s stubUserUC) Register(_ context.Context, r dto.RegisterRequest) (dto.RegisterResponse, error) {
	return s.register(r)
}
func (s stubUserUC) Login(_ context.Context, r dto.LoginRequest) (dto.LoginResponse, error) {
	return s.login(r)
}
func (s stubUserUC) Me(_ context.Context, id string) (dto.MeResponse, error) { return s.me(id) }
func (s stubUserUC) Logout(_ context.Context, r dto.LogoutRequest) error     { return s.logout(r) }

type stubMemberUC struct {
	create func(string, dto.MemberRequest) (dto.MemberResponse, error)
	list   func(string) (dto.MemberListResponse, error)
	update func(string, string, dto.MemberRequest) (dto.MemberResponse, error)
	del    func(string, string) error
}

func (s stubMemberUC) Create(_ context.Context, u string, r dto.MemberRequest) (dto.MemberResponse, error) {
	return s.create(u, r)
}
func (s stubMemberUC) List(_ context.Context, u string) (dto.MemberListResponse, error) {
	return s.list(u)
}
func (s stubMemberUC) Update(_ context.Context, u, m string, r dto.MemberRequest) (dto.MemberResponse, error) {
	return s.update(u, m, r)
}
func (s stubMemberUC) Delete(_ context.Context, u, m string) error { return s.del(u, m) }

type stubHistoryUC struct {
	list func(userID string, page, size int) (dto.TicketHistoryResponse, error)
}

func (s stubHistoryUC) List(_ context.Context, u string, p, sz int) (dto.TicketHistoryResponse, error) {
	return s.list(u, p, sz)
}

// -------------------------------------------------------------- UserHandler

func TestRegisterHandler(t *testing.T) {
	ok := stubUserUC{register: func(r dto.RegisterRequest) (dto.RegisterResponse, error) {
		return dto.RegisterResponse{UserID: "u1", Email: r.Email, Role: entity.RoleCustomer}, nil
	}}
	h := http.HandlerFunc(NewUserHandler(ok).Register)

	t.Run("sukses → 201", func(t *testing.T) {
		rr := call(t, h, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"rahasia123"}`, false)
		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.NotContains(t, rr.Body.String(), "rahasia123", "password tidak boleh kembali di response")
	})
	t.Run("JSON rusak → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, h, http.MethodPost, "/auth/register", `{`, false).Code)
	})
	t.Run("validasi gagal → 400", func(t *testing.T) {
		rr := call(t, h, http.MethodPost, "/auth/register", `{"email":"x","password":"1"}`, false)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, env(t, rr).Message, "email")
	})
	t.Run("email dipakai → 409", func(t *testing.T) {
		uc := stubUserUC{register: func(dto.RegisterRequest) (dto.RegisterResponse, error) {
			return dto.RegisterResponse{}, apperror.EmailTaken()
		}}
		rr := call(t, http.HandlerFunc(NewUserHandler(uc).Register), http.MethodPost, "/auth/register",
			`{"email":"a@b.com","password":"rahasia123"}`, false)
		assert.Equal(t, http.StatusConflict, rr.Code)
	})
}

func TestLoginHandler(t *testing.T) {
	uc := stubUserUC{login: func(r dto.LoginRequest) (dto.LoginResponse, error) {
		if r.Password != "benar" {
			return dto.LoginResponse{}, apperror.InvalidCredentials()
		}
		return dto.LoginResponse{AuthToken: "tok", RefreshToken: "ref", Role: entity.RoleCustomer, ExpiresIn: 3600}, nil
	}}
	h := http.HandlerFunc(NewUserHandler(uc).Login)

	t.Run("sukses → 200 (bukan 201)", func(t *testing.T) {
		rr := call(t, h, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"benar"}`, false)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"authToken":"tok"`)
	})
	t.Run("JSON rusak → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, h, http.MethodPost, "/auth/login", `{`, false).Code)
	})
	t.Run("field kosong → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, h, http.MethodPost, "/auth/login", `{}`, false).Code)
	})
	t.Run("kredensial salah → 401", func(t *testing.T) {
		rr := call(t, h, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"salah"}`, false)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestMeHandler(t *testing.T) {
	var gotID string
	uc := stubUserUC{me: func(id string) (dto.MeResponse, error) {
		gotID = id
		if id == "hilang" {
			return dto.MeResponse{}, apperror.Unauthorized("Sesi tidak valid")
		}
		return dto.MeResponse{UserID: id, Email: "a@b.com", Role: entity.RoleCustomer}, nil
	}}
	h := NewUserHandler(uc)

	t.Run("userId diambil dari token", func(t *testing.T) {
		rr := call(t, authed(h.Me), http.MethodGet, "/auth/me", "", true)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "u1", gotID)
	})
	t.Run("tanpa userId di context → 401", func(t *testing.T) {
		rr := call(t, http.HandlerFunc(h.Me), http.MethodGet, "/auth/me", "", false)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
	t.Run("usecase menolak → 401", func(t *testing.T) {
		h2 := NewUserHandler(stubUserUC{me: func(string) (dto.MeResponse, error) {
			return dto.MeResponse{}, apperror.Unauthorized("Sesi tidak valid")
		}})
		assert.Equal(t, http.StatusUnauthorized, call(t, authed(h2.Me), http.MethodGet, "/auth/me", "", true).Code)
	})
}

func TestLogoutHandler(t *testing.T) {
	var got dto.LogoutRequest
	uc := stubUserUC{logout: func(r dto.LogoutRequest) error { got = r; return nil }}
	h := http.HandlerFunc(NewUserHandler(uc).Logout)

	t.Run("dengan refreshToken → diteruskan", func(t *testing.T) {
		rr := call(t, h, http.MethodPost, "/auth/logout", `{"refreshToken":"abc"}`, false)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "abc", got.RefreshToken)
	})
	t.Run("tanpa body sama sekali → tetap 200 (refreshToken opsional)", func(t *testing.T) {
		got = dto.LogoutRequest{RefreshToken: "sisa"}
		rr := call(t, h, http.MethodPost, "/auth/logout", "", false)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, got.RefreshToken)
	})
	t.Run("JSON rusak → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, h, http.MethodPost, "/auth/logout", `{`, false).Code)
	})
	t.Run("usecase error → 500", func(t *testing.T) {
		bad := stubUserUC{logout: func(dto.LogoutRequest) error { return apperror.Internal(errors.New("redis")) }}
		rr := call(t, http.HandlerFunc(NewUserHandler(bad).Logout), http.MethodPost, "/auth/logout", `{"refreshToken":"x"}`, false)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// ------------------------------------------------------------ MemberHandler

const memberBody = `{"nik":"3273000000000001","nama":"Budi","tglLahir":"1990-01-01","namaIbu":"Siti"}`

func okMemberUC() stubMemberUC {
	m := dto.MemberResponse{MemberID: "m1", NIK: "3273000000000001", Nama: "Budi", TglLahir: "1990-01-01", NamaIbu: "Siti"}
	return stubMemberUC{
		create: func(string, dto.MemberRequest) (dto.MemberResponse, error) { return m, nil },
		list: func(string) (dto.MemberListResponse, error) {
			return dto.MemberListResponse{Items: []dto.MemberResponse{m}}, nil
		},
		update: func(string, string, dto.MemberRequest) (dto.MemberResponse, error) { return m, nil },
		del:    func(string, string) error { return nil },
	}
}

func TestMemberHandler_ShouldReturn401_WhenNoUserInContext(t *testing.T) {
	h := NewMemberHandler(okMemberUC())
	for name, fn := range map[string]http.HandlerFunc{
		"create": h.Create, "list": h.List, "update": h.Update, "delete": h.Delete,
	} {
		t.Run(name, func(t *testing.T) {
			rr := call(t, fn, http.MethodPost, "/x", memberBody, false)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestMemberCreateHandler(t *testing.T) {
	var gotUser string
	uc := okMemberUC()
	uc.create = func(u string, r dto.MemberRequest) (dto.MemberResponse, error) {
		gotUser = u
		if r.Nama == "Gagal" {
			return dto.MemberResponse{}, apperror.NIKInvalid()
		}
		return dto.MemberResponse{MemberID: "m1", NIK: r.NIK, Nama: r.Nama, TglLahir: r.TglLahir, NamaIbu: r.NamaIbu}, nil
	}
	h := authed(NewMemberHandler(uc).Create)

	t.Run("sukses", func(t *testing.T) {
		rr := call(t, h, http.MethodPost, "/m", memberBody, true)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "u1", gotUser)
		assert.Contains(t, rr.Body.String(), `"tglLahir":"1990-01-01"`)
		assert.Contains(t, rr.Body.String(), `"namaIbu":"Siti"`)
	})
	t.Run("JSON rusak → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, h, http.MethodPost, "/m", `{`, true).Code)
	})
	t.Run("validasi gagal → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, h, http.MethodPost, "/m", `{"nik":"1"}`, true).Code)
	})
	t.Run("NIK tidak cocok → 400", func(t *testing.T) {
		body := strings.Replace(memberBody, "Budi", "Gagal", 1)
		assert.Equal(t, http.StatusBadRequest, call(t, h, http.MethodPost, "/m", body, true).Code)
	})
}

func TestMemberListHandler(t *testing.T) {
	t.Run("sukses memuat tglLahir & namaIbu", func(t *testing.T) {
		rr := call(t, authed(NewMemberHandler(okMemberUC()).List), http.MethodGet, "/m", "", true)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"tglLahir":"1990-01-01"`)
		assert.Contains(t, rr.Body.String(), `"namaIbu":"Siti"`)
	})
	t.Run("usecase error → 500", func(t *testing.T) {
		uc := okMemberUC()
		uc.list = func(string) (dto.MemberListResponse, error) {
			return dto.MemberListResponse{}, apperror.Internal(errors.New("db"))
		}
		rr := call(t, authed(NewMemberHandler(uc).List), http.MethodGet, "/m", "", true)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestMemberUpdateHandler(t *testing.T) {
	var gotMember string
	uc := okMemberUC()
	uc.update = func(_, m string, _ dto.MemberRequest) (dto.MemberResponse, error) {
		gotMember = m
		if m == "asing" {
			return dto.MemberResponse{}, apperror.NotFound("member")
		}
		return dto.MemberResponse{MemberID: m}, nil
	}
	mux := http.NewServeMux()
	mux.Handle("PUT /m/{id}", authed(NewMemberHandler(uc).Update))

	t.Run("id dari path diteruskan", func(t *testing.T) {
		rr := call(t, mux, http.MethodPut, "/m/m1", memberBody, true)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "m1", gotMember)
	})
	t.Run("JSON rusak → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, mux, http.MethodPut, "/m/m1", `{`, true).Code)
	})
	t.Run("validasi gagal → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, call(t, mux, http.MethodPut, "/m/m1", `{}`, true).Code)
	})
	t.Run("bukan milik user → 404", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, call(t, mux, http.MethodPut, "/m/asing", memberBody, true).Code)
	})
}

func TestMemberDeleteHandler(t *testing.T) {
	uc := okMemberUC()
	uc.del = func(_, m string) error {
		if m == "asing" {
			return apperror.NotFound("member")
		}
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("DELETE /m/{id}", authed(NewMemberHandler(uc).Delete))

	assert.Equal(t, http.StatusOK, call(t, mux, http.MethodDelete, "/m/m1", "", true).Code)
	assert.Equal(t, http.StatusNotFound, call(t, mux, http.MethodDelete, "/m/asing", "", true).Code)
}

// ----------------------------------------------------------- HistoryHandler

func TestHistoryHandler(t *testing.T) {
	var gotPage, gotSize int
	uc := stubHistoryUC{list: func(_ string, p, s int) (dto.TicketHistoryResponse, error) {
		gotPage, gotSize = p, s
		return dto.TicketHistoryResponse{Items: []dto.TicketHistoryItem{}, Page: p, PageSize: s}, nil
	}}
	h := authed(NewHistoryHandler(uc).List)

	t.Run("query page & pageSize dibaca", func(t *testing.T) {
		rr := call(t, h, http.MethodGet, "/h?page=3&pageSize=50", "", true)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 3, gotPage)
		assert.Equal(t, 50, gotSize)
	})
	t.Run("tanpa query → default 1 & 20", func(t *testing.T) {
		call(t, h, http.MethodGet, "/h", "", true)
		assert.Equal(t, 1, gotPage)
		assert.Equal(t, 20, gotSize)
	})
	t.Run("query bukan angka → default", func(t *testing.T) {
		call(t, h, http.MethodGet, "/h?page=abc&pageSize=xyz", "", true)
		assert.Equal(t, 1, gotPage)
		assert.Equal(t, 20, gotSize)
	})
	t.Run("tanpa userId → 401", func(t *testing.T) {
		rr := call(t, http.HandlerFunc(NewHistoryHandler(uc).List), http.MethodGet, "/h", "", false)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
	t.Run("usecase error → 500", func(t *testing.T) {
		bad := stubHistoryUC{list: func(string, int, int) (dto.TicketHistoryResponse, error) {
			return dto.TicketHistoryResponse{}, apperror.Internal(errors.New("db"))
		}}
		rr := call(t, authed(NewHistoryHandler(bad).List), http.MethodGet, "/h", "", true)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestClientIP(t *testing.T) {
	t.Run("X-Forwarded-For: nilai pertama", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
		assert.Equal(t, "1.1.1.1", clientIP(r))
	})
	t.Run("tanpa XFF → RemoteAddr tanpa port", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "3.3.3.3:5555"
		assert.Equal(t, "3.3.3.3", clientIP(r))
	})
	t.Run("RemoteAddr tanpa port → apa adanya", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "4.4.4.4"
		assert.Equal(t, "4.4.4.4", clientIP(r))
	})
	t.Run("XFF kosong spasi → jatuh ke RemoteAddr", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", " , ")
		r.RemoteAddr = "5.5.5.5:1"
		assert.Equal(t, "5.5.5.5", clientIP(r))
	})
}
