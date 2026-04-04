package middleware

import (
	"testing"
)

func TestRoleHierarchy(t *testing.T) {
	// VIEWER < DEVELOPER < PROJECT_ADMIN < ORG_ADMIN < PLATFORM_ADMIN
	if roleHierarchy[RoleViewer] >= roleHierarchy[RoleDeveloper] {
		t.Error("VIEWER should be lower than DEVELOPER")
	}
	if roleHierarchy[RoleDeveloper] >= roleHierarchy[RoleProjectAdmin] {
		t.Error("DEVELOPER should be lower than PROJECT_ADMIN")
	}
	if roleHierarchy[RoleProjectAdmin] >= roleHierarchy[RoleOrgAdmin] {
		t.Error("PROJECT_ADMIN should be lower than ORG_ADMIN")
	}
	if roleHierarchy[RoleOrgAdmin] >= roleHierarchy[RolePlatformAdmin] {
		t.Error("ORG_ADMIN should be lower than PLATFORM_ADMIN")
	}
}

func TestRoleConstants(t *testing.T) {
	expected := map[string]string{
		RolePlatformAdmin: "PLATFORM_ADMIN",
		RoleOrgAdmin:      "ORG_ADMIN",
		RoleProjectAdmin:  "PROJECT_ADMIN",
		RoleDeveloper:     "DEVELOPER",
		RoleViewer:        "VIEWER",
	}
	for constant, value := range expected {
		if constant != value {
			t.Errorf("constant %q != %q", constant, value)
		}
	}
}

func TestFormatRoleName(t *testing.T) {
	tests := []struct {
		role string
		want string
	}{
		{RolePlatformAdmin, "平台管理员"},
		{RoleDeveloper, "开发者"},
		{RoleViewer, "查看者"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		got := formatRoleName(tt.role)
		if got != tt.want {
			t.Errorf("formatRoleName(%q) = %q, want %q", tt.role, got, tt.want)
		}
	}
}

func TestAllRolesInHierarchy(t *testing.T) {
	roles := []string{RolePlatformAdmin, RoleOrgAdmin, RoleProjectAdmin, RoleDeveloper, RoleViewer}
	for _, r := range roles {
		if _, ok := roleHierarchy[r]; !ok {
			t.Errorf("role %q not in hierarchy", r)
		}
	}
}
