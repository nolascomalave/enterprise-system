package http

// rbacRouteFunctions maps protected HTTP routes to the system_function.code
// required to access them. Routes not present in this map are denied by default
// (fail-closed) to the RBACMiddleware.
var rbacRouteFunctions = map[string]string{
	"POST /api/v1/parties":                            "party:write",
	"GET /api/v1/parties":                             "party:read",
	"GET /api/v1/parties/:id":                         "party:read",
	"POST /api/v1/parties/merge":                      "party:write",
	"POST /api/v1/parties/:id/names":                  "party:write",
	"PUT /api/v1/parties/:id/names/:name_id/validity": "party:write",
	"POST /api/v1/parties/:id/nationalities":          "identity:write",
	"POST /api/v1/parties/:id/identity-documents":     "identity:write",

	"GET /api/v1/jobs":                       "org:read",
	"POST /api/v1/jobs":                      "org:write",
	"PUT /api/v1/jobs/:id":                   "org:write",
	"GET /api/v1/departments":                "org:read",
	"POST /api/v1/departments":               "org:write",
	"POST /api/v1/departments/:id/groups":    "org:write",
	"POST /api/v1/departments/:id/contacts":  "org:write",
	"POST /api/v1/departments/:id/addresses": "org:write",
	"GET /api/v1/employees":                  "org:read",
	"POST /api/v1/employees":                 "org:write",

	"POST /api/v1/documents":                 "documents:write",
	"POST /api/v1/client-applications":       "admin:write",
	"GET /api/v1/platform/status":            "platform:admin",

	"GET /api/v1/rbac/my-permissions":        "rbac:read",
	"GET /api/v1/rbac/my-menu":               "rbac:read",

	"POST /api/v1/auth/change-password":      "account:write",
	"POST /api/v1/corporate-groups":          "org:write",
}

// requiredRBACFunction returns the system function required for a route.
func requiredRBACFunction(method, fullPath string) (string, bool) {
	scope, ok := rbacRouteFunctions[method+" "+fullPath]
	return scope, ok
}
