package common

// InvitationRegistrationEnabled is an operator-controlled policy, not a client option.
// PRLab defaults to invitation-only registration. Operators can explicitly opt out.
func InvitationRegistrationEnabled() bool {
	return GetEnvOrDefaultBool("INVITATION_REGISTRATION_ENABLED", true)
}
