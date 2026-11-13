package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
	"github.com/refreshTechRusuh/user-service/pkg/token"
)

// stubUserRepo = IUserRepository dengan perilaku bisa diatur per test.
type stubUserRepo struct {
	findByEmail func(email string) (entity.User, bool, error)
	findByID    func(id string) (entity.User, bool, error)
	create      func(u entity.User) error
	created     []entity.User
}

func (s *stubUserRepo) Create(_ context.Context, u entity.User) error {
	s.created = append(s.created, u)
	if s.create != nil {
		return s.create(u)
	}
	return nil
}

func (s *stubUserRepo) FindByEmail(_ context.Context, email string) (entity.User, bool, error) {
	if s.findByEmail != nil {
		return s.findByEmail(email)
	}
	return entity.User{}, false, nil
}

func (s *stubUserRepo) FindByID(_ context.Context, id string) (entity.User, bool, error) {
	if s.findByID != nil {
		return s.findByID(id)
	}
	return entity.User{}, false, nil
}

// stubTokenStore = ITokenStore dengan perekam panggilan Revoke.
type stubTokenStore struct {
	revokeErr error
	revoked   []string
	ttl       time.Duration
}

func (s *stubTokenStore) Revoke(_ context.Context, id string, ttl time.Duration) error {
	s.revoked = append(s.revoked, id)
	s.ttl = ttl
	return s.revokeErr
}

func (s *stubTokenStore) IsRevoked(_ context.Context, id string) (bool, error) {
	for _, r := range s.revoked {
		if r == id {
			return true, nil
		}
	}
	return false, nil
}

func newGen() *token.Generator {
	return token.NewGenerator("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
}

func codeOf(t *testing.T, err error) apperror.ErrorCode {
	t.Helper()
	var ae *apperror.AppError
	require.ErrorAs(t, err, &ae)
	return ae.Code
}

func hashed(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

// ---------------------------------------------------------------- Register

func TestRegister_ShouldCreateCustomerWithHashedPassword_WhenEmailFree(t *testing.T) {
	repo := &stubUserRepo{}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	res, err := uc.Register(context.Background(), dto.RegisterRequest{Email: "a@b.com", Password: "rahasia123"})

	require.NoError(t, err)
	assert.Equal(t, "a@b.com", res.Email)
	assert.Equal(t, entity.RoleCustomer, res.Role)
	assert.NotEmpty(t, res.UserID)
	require.Len(t, repo.created, 1)
	assert.NotEqual(t, "rahasia123", repo.created[0].PasswordHash, "password tidak boleh disimpan polos")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(repo.created[0].PasswordHash), []byte("rahasia123")))
}

func TestRegister_ShouldReturnEmailTaken_WhenEmailExists(t *testing.T) {
	repo := &stubUserRepo{findByEmail: func(string) (entity.User, bool, error) { return entity.User{}, true, nil }}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	_, err := uc.Register(context.Background(), dto.RegisterRequest{Email: "a@b.com", Password: "rahasia123"})

	assert.Equal(t, apperror.ErrEmailTaken, codeOf(t, err))
	assert.Empty(t, repo.created)
}

func TestRegister_ShouldReturnInternal_WhenLookupFails(t *testing.T) {
	repo := &stubUserRepo{findByEmail: func(string) (entity.User, bool, error) { return entity.User{}, false, errors.New("db") }}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	_, err := uc.Register(context.Background(), dto.RegisterRequest{Email: "a@b.com", Password: "rahasia123"})

	assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
}

func TestRegister_ShouldPropagateRepoError_WhenCreateRaces(t *testing.T) {
	repo := &stubUserRepo{create: func(entity.User) error { return apperror.EmailTaken() }}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	_, err := uc.Register(context.Background(), dto.RegisterRequest{Email: "a@b.com", Password: "rahasia123"})

	assert.Equal(t, apperror.ErrEmailTaken, codeOf(t, err))
}

func TestRegister_ShouldReturnInternal_WhenPasswordTooLongForBcrypt(t *testing.T) {
	uc := NewUserUsecase(&stubUserRepo{}, newGen(), &stubTokenStore{})
	tooLong := make([]byte, 80) // bcrypt menolak > 72 byte
	for i := range tooLong {
		tooLong[i] = 'a'
	}

	_, err := uc.Register(context.Background(), dto.RegisterRequest{Email: "a@b.com", Password: string(tooLong)})

	assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
}

// ------------------------------------------------------------------- Login

func TestLogin_ShouldIssueTokens_WhenCredentialsValid(t *testing.T) {
	user := entity.User{ID: "u1", Email: "a@b.com", PasswordHash: hashed(t, "rahasia123"), Role: entity.RoleAdmin}
	repo := &stubUserRepo{findByEmail: func(string) (entity.User, bool, error) { return user, true, nil }}
	gen := newGen()
	uc := NewUserUsecase(repo, gen, &stubTokenStore{})

	res, err := uc.Login(context.Background(), dto.LoginRequest{Email: "a@b.com", Password: "rahasia123"})

	require.NoError(t, err)
	assert.Equal(t, entity.RoleAdmin, res.Role)
	assert.Equal(t, 3600, res.ExpiresIn)
	claims, err := gen.VerifyAuthToken(res.AuthToken)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.Subject)
	assert.Equal(t, "admin", claims.Role)
	refresh, err := gen.VerifyRefreshToken(res.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "u1", refresh.Subject)
	assert.NotEmpty(t, claims.SessionID, "setiap login membuat sesi baru")
	assert.Equal(t, claims.SessionID, refresh.SessionID, "authToken & refreshToken harus satu sesi")
}

func TestLogin_ShouldGiveSameError_ForUnknownEmailAndWrongPassword(t *testing.T) {
	known := entity.User{ID: "u1", PasswordHash: hashed(t, "benar-benar")}
	repo := &stubUserRepo{findByEmail: func(email string) (entity.User, bool, error) {
		if email == "ada@b.com" {
			return known, true, nil
		}
		return entity.User{}, false, nil
	}}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	_, errUnknown := uc.Login(context.Background(), dto.LoginRequest{Email: "tidak@ada.com", Password: "x"})
	_, errWrongPw := uc.Login(context.Background(), dto.LoginRequest{Email: "ada@b.com", Password: "salah"})

	assert.Equal(t, apperror.ErrInvalidCredentials, codeOf(t, errUnknown))
	assert.Equal(t, apperror.ErrInvalidCredentials, codeOf(t, errWrongPw))
	var a, b *apperror.AppError
	require.ErrorAs(t, errUnknown, &a)
	require.ErrorAs(t, errWrongPw, &b)
	assert.Equal(t, a.Message, b.Message, "pesan harus sama agar email tidak bisa dienumerasi")
}

func TestLogin_ShouldReturnInternal_WhenLookupFails(t *testing.T) {
	repo := &stubUserRepo{findByEmail: func(string) (entity.User, bool, error) { return entity.User{}, false, errors.New("db") }}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	_, err := uc.Login(context.Background(), dto.LoginRequest{Email: "a@b.com", Password: "x"})

	assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
}

// ---------------------------------------------------------------------- Me

func TestMe_ShouldReturnProfile_WhenUserExists(t *testing.T) {
	repo := &stubUserRepo{findByID: func(string) (entity.User, bool, error) {
		return entity.User{ID: "u1", Email: "a@b.com", Role: entity.RoleCustomer}, true, nil
	}}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	res, err := uc.Me(context.Background(), "u1")

	require.NoError(t, err)
	assert.Equal(t, dto.MeResponse{UserID: "u1", Email: "a@b.com", Role: entity.RoleCustomer}, res)
}

func TestMe_ShouldReturnUnauthorized_WhenUserDeleted(t *testing.T) {
	uc := NewUserUsecase(&stubUserRepo{}, newGen(), &stubTokenStore{})

	_, err := uc.Me(context.Background(), "hilang")

	assert.Equal(t, apperror.ErrUnauthorized, codeOf(t, err))
}

func TestMe_ShouldReturnInternal_WhenLookupFails(t *testing.T) {
	repo := &stubUserRepo{findByID: func(string) (entity.User, bool, error) { return entity.User{}, false, errors.New("db") }}
	uc := NewUserUsecase(repo, newGen(), &stubTokenStore{})

	_, err := uc.Me(context.Background(), "u1")

	assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
}

// ------------------------------------------------------------------ Logout

func TestLogout_ShouldRevokeSession_WhenSessionIDGiven(t *testing.T) {
	store := &stubTokenStore{}
	uc := NewUserUsecase(&stubUserRepo{}, newGen(), store)

	err := uc.Logout(context.Background(), "sid-1")

	require.NoError(t, err)
	assert.Equal(t, []string{token.SessionRevokeID("sid-1")}, store.revoked)
	assert.Equal(t, 24*time.Hour, store.ttl, "TTL denylist = umur maksimum sesi (TTL refresh token)")
}

func TestLogout_ShouldSucceedWithoutRevoking_WhenSessionIDEmpty(t *testing.T) {
	store := &stubTokenStore{}
	uc := NewUserUsecase(&stubUserRepo{}, newGen(), store)

	err := uc.Logout(context.Background(), "")

	require.NoError(t, err, "token lama tanpa sid: tidak ada yang bisa dicabut")
	assert.Empty(t, store.revoked)
}

func TestLogout_ShouldReturnInternal_WhenStoreFails(t *testing.T) {
	uc := NewUserUsecase(&stubUserRepo{}, newGen(), &stubTokenStore{revokeErr: errors.New("redis")})

	err := uc.Logout(context.Background(), "sid-1")

	assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
}

// ----------------------------------------------------------------- Refresh

func userRepo(role entity.Role) *stubUserRepo {
	return &stubUserRepo{findByID: func(id string) (entity.User, bool, error) {
		return entity.User{ID: id, Email: "a@b.com", Role: role}, true, nil
	}}
}

func TestRefresh_ShouldIssueNewPairInSameSession_AndConsumeOldRefreshToken(t *testing.T) {
	gen := newGen()
	store := &stubTokenStore{}
	uc := NewUserUsecase(userRepo(entity.RoleAdmin), gen, store)
	old, err := gen.GenerateRefreshToken("u1", "sid-1")
	require.NoError(t, err)

	res, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: old})

	require.NoError(t, err)
	auth, err := gen.VerifyAuthToken(res.AuthToken)
	require.NoError(t, err)
	assert.Equal(t, "u1", auth.Subject)
	assert.Equal(t, "admin", auth.Role)
	assert.Equal(t, "sid-1", auth.SessionID, "sesi tidak berganti saat refresh")
	newRefresh, err := gen.VerifyRefreshToken(res.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "sid-1", newRefresh.SessionID)
	assert.Equal(t, 3600, res.ExpiresIn)
	assert.Equal(t, entity.RoleAdmin, res.Role)
	assert.Equal(t, []string{old}, store.revoked, "refresh token lama ditandai terpakai")
}

func TestRefresh_ShouldRejectSecondUse_WhenRefreshTokenAlreadyUsed(t *testing.T) {
	gen := newGen()
	uc := NewUserUsecase(userRepo(entity.RoleCustomer), gen, &stubTokenStore{})
	old, err := gen.GenerateRefreshToken("u1", "sid-1")
	require.NoError(t, err)

	_, err = uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: old})
	require.NoError(t, err)
	_, err = uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: old})

	assert.Equal(t, apperror.ErrUnauthorized, codeOf(t, err))
}

func TestRefresh_ShouldReject_WhenSessionLoggedOut(t *testing.T) {
	gen := newGen()
	store := &stubTokenStore{revoked: []string{token.SessionRevokeID("sid-1")}}
	uc := NewUserUsecase(userRepo(entity.RoleCustomer), gen, store)
	rt, err := gen.GenerateRefreshToken("u1", "sid-1")
	require.NoError(t, err)

	_, err = uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: rt})

	assert.Equal(t, apperror.ErrUnauthorized, codeOf(t, err), "logout harus mematikan refresh token sesi itu")
}

func TestRefresh_ShouldReject_WhenTokenInvalidOrWrongKind(t *testing.T) {
	gen := newGen()
	uc := NewUserUsecase(userRepo(entity.RoleCustomer), gen, &stubTokenStore{})
	authTok, err := gen.GenerateAuthToken("u1", "customer", "sid-1")
	require.NoError(t, err)
	expired, err := token.NewGenerator("access-secret", "refresh-secret", time.Hour, -time.Minute).GenerateRefreshToken("u1", "sid-1")
	require.NoError(t, err)

	for name, tok := range map[string]string{"sampah": "sampah", "authToken dipakai sebagai refresh": authTok, "kedaluwarsa": expired} {
		t.Run(name, func(t *testing.T) {
			_, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: tok})
			assert.Equal(t, apperror.ErrUnauthorized, codeOf(t, err))
		})
	}
}

func TestRefresh_ShouldReject_WhenUserNoLongerExists(t *testing.T) {
	gen := newGen()
	store := &stubTokenStore{}
	uc := NewUserUsecase(&stubUserRepo{}, gen, store)
	rt, err := gen.GenerateRefreshToken("hilang", "sid-1")
	require.NoError(t, err)

	_, err = uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: rt})

	assert.Equal(t, apperror.ErrUnauthorized, codeOf(t, err))
	assert.Empty(t, store.revoked, "token tidak boleh hangus bila gagal sebelum diterbitkan ulang")
}

func TestRefresh_ShouldStartNewSession_WhenLegacyTokenHasNoSessionID(t *testing.T) {
	gen := newGen()
	uc := NewUserUsecase(userRepo(entity.RoleCustomer), gen, &stubTokenStore{})
	legacy, err := gen.GenerateRefreshToken("u1", "")
	require.NoError(t, err)

	res, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: legacy})

	require.NoError(t, err)
	auth, err := gen.VerifyAuthToken(res.AuthToken)
	require.NoError(t, err)
	assert.NotEmpty(t, auth.SessionID)
}

func TestRefresh_ShouldReturnInternal_WhenStoreOrRepoFails(t *testing.T) {
	gen := newGen()
	rt, err := gen.GenerateRefreshToken("u1", "sid-1")
	require.NoError(t, err)

	t.Run("revoke gagal", func(t *testing.T) {
		uc := NewUserUsecase(userRepo(entity.RoleCustomer), gen, &stubTokenStore{revokeErr: errors.New("redis")})
		_, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: rt})
		assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
	})
	t.Run("lookup user gagal", func(t *testing.T) {
		repo := &stubUserRepo{findByID: func(string) (entity.User, bool, error) { return entity.User{}, false, errors.New("db") }}
		uc := NewUserUsecase(repo, gen, &stubTokenStore{})
		_, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: rt})
		assert.Equal(t, apperror.ErrInternal, codeOf(t, err))
	})
}
