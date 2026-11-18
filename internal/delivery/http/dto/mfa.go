package dto

type UpdateMFAPreferencesRequest struct {
	Enabled bool   `json:"enabled"`
	Method  string `json:"method"`
	OTPCode string `json:"otp_code"`
}

type RequestSecurityCodeRequest struct {
	Action string `json:"action" binding:"required"`
}

type TOTPSetupResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

type TOTPSetupEnvelope struct {
	Data TOTPSetupResponse `json:"data"`
}

type TOTPVerifyRequest struct {
	Code string `json:"code" binding:"required"`
}

type TOTPVerifyResponse struct {
	BackupCodes []string `json:"backup_codes"`
}

type TOTPVerifyEnvelope struct {
	Data TOTPVerifyResponse `json:"data"`
}
