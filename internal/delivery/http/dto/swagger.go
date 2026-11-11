package dto

// The envelope types below exist so Swag can describe the API's standard
// {"data": ...} response shape without exposing domain entities.

type ErrorResponse struct {
	Error string `json:"error" example:"invalid request"`
	Code  string `json:"code" example:"bad_request"`
}

type AuthEnvelope struct {
	Data AuthTokenResponse `json:"data"`
}

type MessageEnvelope struct {
	Data MessageResponse `json:"data"`
}

type PartyEnvelope struct {
	Data PartyResponse `json:"data"`
}

type PartyListEnvelope struct {
	Data []PartyResponse `json:"data"`
}

type RegisterTenantEnvelope struct {
	Data RegisterTenantResponse `json:"data"`
}

type HealthEnvelope struct {
	Data HealthResponse `json:"data"`
}

type HealthResponse struct {
	Status    string `json:"status" example:"ok"`
	Service   string `json:"service" example:"enterprise-system"`
	Timestamp string `json:"timestamp" example:"2026-11-10T12:00:00Z"`
}
