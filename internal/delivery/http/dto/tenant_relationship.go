package dto

import "time"

type InitiateRelationshipRequest struct {
	HoldingTenantID  string `json:"holding_tenant_id" binding:"required"`
	RelationshipType string `json:"relationship_type" binding:"required"`
}

type ConsumeInviteTokenRequest struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

type RelationshipReasonRequest struct {
	Reason string `json:"reason"`
}

type EmitInviteTokenResponse struct {
	InviteCode string    `json:"invite_code"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type TenantRelationshipResponse struct {
	ID                 string     `json:"id"`
	TenantIDHolding    string     `json:"tenant_id_holding"`
	TenantIDSubsidiary string     `json:"tenant_id_subsidiary"`
	RelationshipType   string     `json:"relationship_type"`
	Status             string     `json:"status"`
	RequestedAt        time.Time  `json:"requested_at"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
	RejectedAt         *time.Time `json:"rejected_at,omitempty"`
	CanceledAt         *time.Time `json:"canceled_at,omitempty"`
	EndsAt             *time.Time `json:"ends_at,omitempty"`
	RejectionReason    string     `json:"rejection_reason,omitempty"`
}

type TenantRelationshipEnvelope struct {
	Data TenantRelationshipResponse `json:"data"`
}

type EmitInviteTokenEnvelope struct {
	Data EmitInviteTokenResponse `json:"data"`
}
