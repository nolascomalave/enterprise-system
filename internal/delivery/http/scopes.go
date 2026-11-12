package http

import "enterprise-system/internal/auth"

// requiredScope maps a protected route to the client-credentials scope needed
// to access it. Routes not present in this table are denied to M2M tokens.
func requiredScope(method, fullPath string) (string, bool) {
	scope, ok := routeScopes[method+" "+fullPath]
	return scope, ok
}

var routeScopes = map[string]string{
	"GET /api/v1/parties":                             auth.ScopePartiesRead,
	"POST /api/v1/parties":                            auth.ScopePartiesWrite,
	"GET /api/v1/parties/:id":                         auth.ScopePartiesRead,
	"POST /api/v1/parties/merge":                      auth.ScopePartiesWrite,
	"POST /api/v1/parties/:id/names":                  auth.ScopePartiesWrite,
	"PUT /api/v1/parties/:id/names/:name_id/validity": auth.ScopePartiesWrite,
	"POST /api/v1/parties/:id/nationalities":          auth.ScopeIdentityWrite,
	"POST /api/v1/parties/:id/identity-documents":     auth.ScopeIdentityWrite,

	"GET /api/v1/jobs":                       auth.ScopeOrgRead,
	"POST /api/v1/jobs":                      auth.ScopeOrgWrite,
	"PUT /api/v1/jobs/:id":                   auth.ScopeOrgWrite,
	"GET /api/v1/departments":                auth.ScopeOrgRead,
	"POST /api/v1/departments":               auth.ScopeOrgWrite,
	"POST /api/v1/departments/:id/groups":    auth.ScopeOrgWrite,
	"POST /api/v1/departments/:id/contacts":  auth.ScopeOrgWrite,
	"POST /api/v1/departments/:id/addresses": auth.ScopeOrgWrite,
	"GET /api/v1/employees":                  auth.ScopeOrgRead,
	"POST /api/v1/employees":                 auth.ScopeOrgWrite,
	"POST /api/v1/documents":                 auth.ScopeDocumentsWrite,
	"POST /api/v1/client-applications":       auth.ScopeAdminWrite,
}
