package dto

import "enterprise-system/internal/domain"

// PermissionUniverseResponse is the resolved set of system function codes for a user.
type PermissionUniverseResponse struct {
	Functions []string `json:"functions"`
}

// PermissionUniverseEnvelope wraps the permission universe in the standard data envelope.
type PermissionUniverseEnvelope struct {
	Data PermissionUniverseResponse `json:"data"`
}

// MenuTreeEnvelope wraps the resolved menu tree in the standard data envelope.
type MenuTreeEnvelope struct {
	Data []domain.MenuTreeNode `json:"data"`
}
