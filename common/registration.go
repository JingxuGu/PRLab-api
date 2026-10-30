package common

// ApprovalRegistrationEnabled is operator-controlled. Users apply with an email;
// only root approval can create an account. The old variable remains a fallback
// so upgrading an existing PRLab installation never opens unreviewed signup.
func ApprovalRegistrationEnabled() bool {
	return GetEnvOrDefaultBool("APPROVAL_REGISTRATION_ENABLED", GetEnvOrDefaultBool("INVITATION_REGISTRATION_ENABLED", true))
}

// Kept for existing OAuth/WeChat guards and maintenance-branch compatibility.
func InvitationRegistrationEnabled() bool { return ApprovalRegistrationEnabled() }
