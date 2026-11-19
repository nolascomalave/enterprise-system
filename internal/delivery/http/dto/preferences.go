package dto

type UpdatePreferencesRequest struct {
	Preferences map[string]any `json:"preferences" binding:"required"`
}

type PreferencesResponse struct {
	Preferences map[string]any `json:"preferences"`
}

type PreferencesEnvelope struct {
	Data PreferencesResponse `json:"data"`
}
