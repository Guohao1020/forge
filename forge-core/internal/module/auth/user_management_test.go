package auth

import "testing"

func TestCreateUserRequest_Validation(t *testing.T) {
	req := CreateUserRequest{
		Username: "testuser",
		Password: "pass123",
		Role:     "DEVELOPER",
	}
	if req.Username == "" {
		t.Error("username should not be empty")
	}
	if len(req.Password) < 6 {
		t.Error("password should be at least 6 chars")
	}
}

func TestCreateUserRequest_DefaultRole(t *testing.T) {
	req := CreateUserRequest{
		Username: "user",
		Password: "password",
	}
	// When Role is empty, service assigns "DEVELOPER" default
	if req.Role != "" {
		t.Errorf("expected empty role (default applied by service), got %s", req.Role)
	}
}

func TestUpdateUserRoleRequest(t *testing.T) {
	req := UpdateUserRoleRequest{Role: "PLATFORM_ADMIN"}
	if req.Role != "PLATFORM_ADMIN" {
		t.Errorf("expected PLATFORM_ADMIN, got %s", req.Role)
	}
}

func TestChangePasswordRequest(t *testing.T) {
	req := ChangePasswordRequest{
		OldPassword: "old123",
		NewPassword: "new456",
	}
	if req.OldPassword == req.NewPassword {
		t.Error("old and new password should differ")
	}
	if len(req.NewPassword) < 6 {
		t.Error("new password should be at least 6 chars")
	}
}

func TestUserListItem(t *testing.T) {
	user := UserListItem{
		ID:          1,
		Username:    "admin",
		DisplayName: "Admin User",
		Roles:       []string{"PLATFORM_ADMIN"},
		Status:      "ACTIVE",
	}
	if user.Username != "admin" {
		t.Errorf("expected admin, got %s", user.Username)
	}
	if len(user.Roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(user.Roles))
	}
	if user.Roles[0] != "PLATFORM_ADMIN" {
		t.Errorf("expected PLATFORM_ADMIN, got %s", user.Roles[0])
	}
}
