package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hugovillarreal/ridesharing/internal/models"
)

// --- Mock Repository ---

type mockUserRepo struct {
	users           map[uuid.UUID]*models.UserWithDriver
	updateErr       error
	availabilityErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[uuid.UUID]*models.UserWithDriver),
	}
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.UserWithDriver, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (m *mockUserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, name, phone string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if user, ok := m.users[id]; ok {
		user.Name = name
		user.Phone = phone
	}
	return nil
}

func (m *mockUserRepo) UpdateDriver(ctx context.Context, userID uuid.UUID, vehicleType, licensePlate string) error {
	if user, ok := m.users[userID]; ok && user.Driver != nil {
		user.Driver.VehicleType = vehicleType
		user.Driver.LicensePlate = licensePlate
	}
	return nil
}

func (m *mockUserRepo) SetAvailability(ctx context.Context, userID uuid.UUID, available bool) error {
	if m.availabilityErr != nil {
		return m.availabilityErr
	}
	if user, ok := m.users[userID]; ok && user.Driver != nil {
		user.Driver.IsAvailable = available
	}
	return nil
}

// --- Tests ---

func TestGetProfile_Found(t *testing.T) {
	repo := newMockUserRepo()
	userID := uuid.New()
	repo.users[userID] = &models.UserWithDriver{
		User: models.User{
			ID:    userID,
			Email: "test@example.com",
			Name:  "Test User",
			Role:  "rider",
		},
	}

	svc := NewService(repo)
	profile, err := svc.GetProfile(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if profile.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %q", profile.Email)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	svc := NewService(newMockUserRepo())
	_, err := svc.GetProfile(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestUpdateProfile_Success(t *testing.T) {
	repo := newMockUserRepo()
	userID := uuid.New()
	repo.users[userID] = &models.UserWithDriver{
		User: models.User{
			ID:   userID,
			Name: "Old Name",
			Role: "rider",
		},
	}

	svc := NewService(repo)
	updated, err := svc.UpdateProfile(context.Background(), userID, UpdateProfileRequest{
		Name:  "New Name",
		Phone: "555-1234",
	})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %q", updated.Name)
	}
}

func TestUpdateProfile_NotFound(t *testing.T) {
	svc := NewService(newMockUserRepo())
	_, err := svc.UpdateProfile(context.Background(), uuid.New(), UpdateProfileRequest{Name: "x"})
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestUpdateProfile_Error(t *testing.T) {
	repo := newMockUserRepo()
	userID := uuid.New()
	repo.users[userID] = &models.UserWithDriver{
		User: models.User{ID: userID, Name: "Test", Role: "rider"},
	}
	repo.updateErr = errors.New("db error")

	svc := NewService(repo)
	_, err := svc.UpdateProfile(context.Background(), userID, UpdateProfileRequest{Name: "New"})
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

func TestUpdateProfile_DriverFields(t *testing.T) {
	repo := newMockUserRepo()
	userID := uuid.New()
	repo.users[userID] = &models.UserWithDriver{
		User: models.User{
			ID:   userID,
			Name: "Driver",
			Role: "driver",
		},
		Driver: &models.Driver{
			UserID:       userID,
			VehicleType:  "sedan",
			LicensePlate: "OLD-123",
			IsAvailable:  true,
			Rating:       4.5,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	svc := NewService(repo)
	updated, err := svc.UpdateProfile(context.Background(), userID, UpdateProfileRequest{
		VehicleType:  "SUV",
		LicensePlate: "NEW-456",
	})
	if err != nil {
		t.Fatalf("UpdateProfile driver failed: %v", err)
	}
	if updated.Driver.VehicleType != "SUV" {
		t.Errorf("expected vehicle_type 'SUV', got %q", updated.Driver.VehicleType)
	}
}

func TestSetAvailability_Success(t *testing.T) {
	repo := newMockUserRepo()
	userID := uuid.New()
	repo.users[userID] = &models.UserWithDriver{
		User: models.User{ID: userID, Role: "driver"},
		Driver: &models.Driver{
			UserID:      userID,
			IsAvailable: true,
		},
	}

	svc := NewService(repo)
	err := svc.SetAvailability(context.Background(), userID, false)
	if err != nil {
		t.Fatalf("SetAvailability failed: %v", err)
	}
}

func TestSetAvailability_NotDriver(t *testing.T) {
	repo := newMockUserRepo()
	userID := uuid.New()
	repo.users[userID] = &models.UserWithDriver{
		User: models.User{ID: userID, Role: "rider"},
	}

	svc := NewService(repo)
	err := svc.SetAvailability(context.Background(), userID, true)
	if err == nil {
		t.Fatal("expected error for non-driver")
	}
}

func TestSetAvailability_NotFound(t *testing.T) {
	svc := NewService(newMockUserRepo())
	err := svc.SetAvailability(context.Background(), uuid.New(), true)
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestSetAvailability_RepoError(t *testing.T) {
	repo := newMockUserRepo()
	userID := uuid.New()
	repo.users[userID] = &models.UserWithDriver{
		User:   models.User{ID: userID, Role: "driver"},
		Driver: &models.Driver{UserID: userID},
	}
	repo.availabilityErr = errors.New("db error")

	svc := NewService(repo)
	err := svc.SetAvailability(context.Background(), userID, false)
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}
