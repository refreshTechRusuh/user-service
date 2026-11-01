package usecase

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/entity"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// fakeUserRepo = IUserRepository di memori untuk test.
type fakeUserRepo struct {
	byEmail   map[string]entity.User
	createErr error
	created   int
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: map[string]entity.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u entity.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.byEmail[u.Email] = u
	f.created++
	return nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (entity.User, bool, error) {
	u, ok := f.byEmail[email]
	return u, ok, nil
}

func (f *fakeUserRepo) FindByID(_ context.Context, id string) (entity.User, bool, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, true, nil
		}
	}
	return entity.User{}, false, nil
}

const goodPassword = "admin-password-123"

func TestEnsureAdmin_ShouldCreateAdmin_WhenEmailNotRegistered(t *testing.T) {
	repo := newFakeUserRepo()
	uc := NewUserUsecase(repo, nil, nil)

	res, err := uc.EnsureAdmin(context.Background(), " admin@rusuh.local ", goodPassword)
	if err != nil || res != BootstrapCreated {
		t.Fatalf("hasil=%s err=%v; mau CREATED", res, err)
	}
	u := repo.byEmail["admin@rusuh.local"]
	if u.Role != entity.RoleAdmin {
		t.Errorf("role = %s; mau admin", u.Role)
	}
	if u.PasswordHash == goodPassword {
		t.Fatal("password tersimpan plain text")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(goodPassword)) != nil {
		t.Error("hash bcrypt tidak cocok dengan password")
	}
}

func TestEnsureAdmin_ShouldBeIdempotent_WhenRunTwice(t *testing.T) {
	repo := newFakeUserRepo()
	uc := NewUserUsecase(repo, nil, nil)
	ctx := context.Background()

	if _, err := uc.EnsureAdmin(ctx, "admin@rusuh.local", goodPassword); err != nil {
		t.Fatal(err)
	}
	oldHash := repo.byEmail["admin@rusuh.local"].PasswordHash

	res, err := uc.EnsureAdmin(ctx, "admin@rusuh.local", "password-lain-123456")
	if err != nil || res != BootstrapExists {
		t.Fatalf("hasil=%s err=%v; mau EXISTS", res, err)
	}
	if repo.created != 1 {
		t.Errorf("Create dipanggil %d kali; mau 1", repo.created)
	}
	if repo.byEmail["admin@rusuh.local"].PasswordHash != oldHash {
		t.Error("password admin tertimpa saat startup ulang")
	}
}

// Akun customer dengan email yang sama TIDAK boleh dinaikkan jadi admin otomatis.
func TestEnsureAdmin_ShouldNotPromoteCustomer_WhenEmailBelongsToCustomer(t *testing.T) {
	repo := newFakeUserRepo()
	repo.byEmail["budi@mail.com"] = entity.User{ID: "u1", Email: "budi@mail.com", Role: entity.RoleCustomer}
	uc := NewUserUsecase(repo, nil, nil)

	res, err := uc.EnsureAdmin(context.Background(), "budi@mail.com", goodPassword)
	if err != nil || res != BootstrapNotAdm {
		t.Fatalf("hasil=%s err=%v; mau NOT_ADMIN", res, err)
	}
	if repo.byEmail["budi@mail.com"].Role != entity.RoleCustomer {
		t.Error("customer dinaikkan jadi admin")
	}
}

func TestEnsureAdmin_ShouldTreatRaceAsExists_WhenAnotherInstanceCreatedFirst(t *testing.T) {
	repo := newFakeUserRepo()
	repo.createErr = apperror.EmailTaken()
	uc := NewUserUsecase(repo, nil, nil)

	res, err := uc.EnsureAdmin(context.Background(), "admin@rusuh.local", goodPassword)
	if err != nil || res != BootstrapExists {
		t.Fatalf("hasil=%s err=%v; mau EXISTS", res, err)
	}
}

func TestEnsureAdmin_ShouldValidateInput(t *testing.T) {
	cases := []struct {
		name, email, password string
		wantRes               BootstrapResult
		wantErr               bool
	}{
		{"email kosong → dilewati", "", "", BootstrapSkipped, false},
		{"email tanpa @", "admin", goodPassword, "", true},
		{"password < 12", "admin@rusuh.local", "pendek123", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc := NewUserUsecase(newFakeUserRepo(), nil, nil)
			res, err := uc.EnsureAdmin(context.Background(), c.email, c.password)
			if (err != nil) != c.wantErr || res != c.wantRes {
				t.Errorf("hasil=%q err=%v; mau hasil=%q err?=%v", res, err, c.wantRes, c.wantErr)
			}
		})
	}
}

func TestEnsureAdmin_ShouldReturnError_WhenRepoFails(t *testing.T) {
	repo := newFakeUserRepo()
	repo.createErr = errors.New("db mati")
	uc := NewUserUsecase(repo, nil, nil)

	if _, err := uc.EnsureAdmin(context.Background(), "admin@rusuh.local", goodPassword); err == nil {
		t.Fatal("mau error saat DB gagal")
	}
}
