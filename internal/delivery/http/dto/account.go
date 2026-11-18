package dto

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	OTPCode         string `json:"otp_code"`
}

type ForgotPasswordRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
