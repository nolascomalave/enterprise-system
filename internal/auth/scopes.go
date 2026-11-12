package auth

// OAuth2 scopes used by client-credentials tokens.
const (
	ScopePartiesRead    = "parties:read"
	ScopePartiesWrite   = "parties:write"
	ScopeIdentityWrite  = "identity:write"
	ScopeOrgRead        = "org:read"
	ScopeOrgWrite       = "org:write"
	ScopeDocumentsWrite = "documents:write"
	ScopeLocationsRead  = "locations:read"
	ScopeAdminWrite     = "admin:write"
)

var knownScopes = map[string]struct{}{
	ScopePartiesRead:    {},
	ScopePartiesWrite:   {},
	ScopeIdentityWrite:  {},
	ScopeOrgRead:        {},
	ScopeOrgWrite:       {},
	ScopeDocumentsWrite: {},
	ScopeLocationsRead:  {},
	ScopeAdminWrite:     {},
}

// IsKnownScope reports whether a scope is part of the supported catalog.
func IsKnownScope(scope string) bool {
	_, ok := knownScopes[scope]
	return ok
}

// HasScope returns true when the granted scopes include the required one.
// The wildcard "*" grants every scope.
func HasScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == "*" || scope == required {
			return true
		}
	}
	return false
}
