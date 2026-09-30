package auth

import (
	"testing"

	"github.com/Potterluo/dream-interviewer/internal/store"
)

// IsAdmin is the single gate every admin route and every scope widening
// depends on, so its two conditions are pinned here.
//
// The regression this guards: the check used to be "API key => admin tier"
// alone, ignoring the owner's role. A demoted admin's admin-tier key then
// kept full admin access — including reading every user's interviews,
// rewriting the shared model configuration, and re-promoting its owner —
// because revocation by role never reached the key path. It was reachable
// only through an API key, which is why it survived a session-only review.
func TestIsAdminRequiresRoleAndKeyTier(t *testing.T) {
	cases := []struct {
		name string
		id   Identity
		want bool
	}{
		{
			name: "admin session",
			id:   Identity{Role: store.RoleAdmin, AuthMethod: AuthSession},
			want: true,
		},
		{
			name: "user session",
			id:   Identity{Role: store.RoleUser, AuthMethod: AuthSession},
			want: false,
		},
		{
			name: "admin with admin-tier key",
			id: Identity{Role: store.RoleAdmin, AuthMethod: AuthAPIKey,
				APIKeyType: store.APIKeyTypeAdmin},
			want: true,
		},
		{
			// The escalation: owner demoted, key still admin tier.
			name: "DEMOTED admin with admin-tier key",
			id: Identity{Role: store.RoleUser, AuthMethod: AuthAPIKey,
				APIKeyType: store.APIKeyTypeAdmin},
			want: false,
		},
		{
			// The deliberate narrowing, which must keep working.
			name: "admin with a deliberately narrow user-tier key",
			id: Identity{Role: store.RoleAdmin, AuthMethod: AuthAPIKey,
				APIKeyType: store.APIKeyTypeUser},
			want: false,
		},
		{
			name: "user with user-tier key",
			id: Identity{Role: store.RoleUser, AuthMethod: AuthAPIKey,
				APIKeyType: store.APIKeyTypeUser},
			want: false,
		},
		{
			// An identity with no role at all must not pass: fail closed.
			name: "empty identity",
			id:   Identity{},
			want: false,
		},
		{
			name: "admin-tier key but an empty role",
			id:   Identity{AuthMethod: AuthAPIKey, APIKeyType: store.APIKeyTypeAdmin},
			want: false,
		},
		{
			name: "unknown role string",
			id:   Identity{Role: "superuser", AuthMethod: AuthSession},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.id.IsAdmin(); got != tc.want {
				t.Fatalf("IsAdmin() = %v, want %v (identity: %+v)", got, tc.want, tc.id)
			}
		})
	}
}

func TestEffectiveUserIDAndReadOnly(t *testing.T) {
	self := Identity{UserID: "u_a", Role: store.RoleUser}
	if self.EffectiveUserID() != "u_a" {
		t.Fatalf("EffectiveUserID = %q", self.EffectiveUserID())
	}
	if self.ReadOnly() {
		t.Fatal("a plain session must not be read-only")
	}

	acting := Identity{UserID: "u_admin", Role: store.RoleAdmin, ActAsUserID: "u_b"}
	if acting.EffectiveUserID() != "u_b" {
		t.Fatalf("actAs was not honoured: %q", acting.EffectiveUserID())
	}
	if !acting.ReadOnly() {
		t.Fatal("actAs must be read-only so mutating routes reject it")
	}
	if !acting.IsActingAs() {
		t.Fatal("IsActingAs should be true")
	}

	// ActAs pointing at yourself is not impersonation.
	same := Identity{UserID: "u_admin", Role: store.RoleAdmin, ActAsUserID: "u_admin"}
	if same.IsActingAs() || same.ReadOnly() {
		t.Fatal("actAs with your own id must not count as impersonation")
	}
	// …but the resolved user is still yourself.
	if same.EffectiveUserID() != "u_admin" {
		t.Fatalf("EffectiveUserID = %q", same.EffectiveUserID())
	}
}
