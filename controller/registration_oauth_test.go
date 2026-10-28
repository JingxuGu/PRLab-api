package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/stretchr/testify/require"
)

// Embed the interface so any unexpected provider call fails the test. Only lookups
// may happen before the invitation policy rejects a new OAuth registration.
type invitationOAuthProvider struct {
	oauth.Provider
	existing bool
}

func (p invitationOAuthProvider) IsUserIDTaken(string) bool { return p.existing }
func (p invitationOAuthProvider) FillUserByProviderID(u *model.User, _ string) error {
	u.Id = 42
	u.Username = "existing-user"
	return nil
}

func TestInvitationPolicyBlocksNewOAuthButPreservesExistingLogin(t *testing.T) {
	t.Setenv("INVITATION_REGISTRATION_ENABLED", "true")
	old := common.RegisterEnabled
	common.RegisterEnabled = true
	t.Cleanup(func() { common.RegisterEnabled = old })
	input := &oauth.OAuthUser{ProviderUserID: "provider-user", Extra: map[string]interface{}{}}
	_, err := findOrCreateOAuthUser(nil, invitationOAuthProvider{}, input, nil)
	var disabled *OAuthRegistrationDisabledError
	require.ErrorAs(t, err, &disabled)
	user, err := findOrCreateOAuthUser(nil, invitationOAuthProvider{existing: true}, input, nil)
	require.NoError(t, err)
	require.Equal(t, 42, user.Id)
}
