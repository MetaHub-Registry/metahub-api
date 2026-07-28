package config

import "github.com/spf13/viper"

// DefaultUserQuotaBytes is the storage quota auto-provisioned for a personal
// namespace (user/<username>) on first push: 5 GiB. See openspec change
// add-registry-namespace-authz.
const DefaultUserQuotaBytes = int64(5) << 30 // 5 GiB

// RegistryConfig holds the namespace-authorization switches. Both enforcement
// flags default to false so a deploy is behavior-neutral (ungated reads,
// admin-only writes) until enabled per environment (dev -> prod); they are
// independent so read and write isolation can be rolled out separately.
type RegistryConfig struct {
	EnforcePullAuthz      bool  // gate GET/HEAD by namespace class + groups
	EnforceWriteOwnership bool  // gate push by owner/org-admin/project-admin (not just global-admin)
	DefaultUserQuotaBytes int64 // personal-namespace default quota
}

// RegistryConfigFromViper reads the registry switches. Missing flags default to
// false; a missing/zero default quota falls back to DefaultUserQuotaBytes.
func RegistryConfigFromViper() RegistryConfig {
	q := viper.GetInt64("registry.default_user_quota_bytes")
	if q <= 0 {
		q = DefaultUserQuotaBytes
	}
	return RegistryConfig{
		EnforcePullAuthz:      viper.GetBool("registry.enforce_pull_authz"),
		EnforceWriteOwnership: viper.GetBool("registry.enforce_write_ownership"),
		DefaultUserQuotaBytes: q,
	}
}
