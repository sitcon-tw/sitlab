package workspace

import "testing"

func TestRolePolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role   Role
		read   bool
		write  bool
		manage bool
	}{
		{RoleOwner, true, true, true},
		{RoleEditor, true, true, false},
		{RoleViewer, true, false, false},
		{Role("unknown"), false, false, false},
	}
	for _, tt := range tests {
		if got := tt.role.CanRead(); got != tt.read {
			t.Errorf("%s CanRead() = %v", tt.role, got)
		}
		if got := tt.role.CanWriteTasks(); got != tt.write {
			t.Errorf("%s CanWriteTasks() = %v", tt.role, got)
		}
		if got := tt.role.CanManageWorkspace(); got != tt.manage {
			t.Errorf("%s CanManageWorkspace() = %v", tt.role, got)
		}
	}
}
