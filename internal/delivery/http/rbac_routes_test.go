package http

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTenantRelationshipRoutesUseDedicatedFunction(t *testing.T) {
	routes := []string{
		"POST /api/v1/tenant-relationships/initiate",
		"POST /api/v1/tenant-relationships/consume-token",
		"POST /api/v1/tenant-relationships/:id/emit-token",
		"POST /api/v1/tenant-relationships/:id/reject",
		"POST /api/v1/tenant-relationships/:id/cancel",
		"POST /api/v1/tenant-relationships/:id/revoke",
	}
	for _, route := range routes {
		method, path := splitRoute(route)
		function, ok := requiredRBACFunction(method, path)
		require.True(t, ok, route)
		require.Equal(t, "tenant_relationship:write", function, route)
	}
}

func splitRoute(route string) (string, string) {
	for i := 0; i < len(route); i++ {
		if route[i] == ' ' {
			return route[:i], route[i+1:]
		}
	}
	return route, ""
}
