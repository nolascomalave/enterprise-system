package dto

type UpdateMFAPreferencesRequest struct {
	Enabled bool   `json:"enabled"`
	Method  string `json:"method"`
}

type RequestSecurityCodeRequest struct {
	Action string `json:"action" binding:"required"`
}
