package dto

type RegisterTenantRequest struct {
	CompanyName    string `json:"company_name" binding:"required"`
	TaxID          string `json:"tax_id" binding:"required"`
	Slug           string `json:"slug" binding:"required"`
	AdminFirstName string `json:"admin_first_name" binding:"required"`
	AdminLastName  string `json:"admin_last_name" binding:"required"`
	AdminEmail     string `json:"admin_email" binding:"required,email"`
	AdminPassword  string `json:"admin_password" binding:"required,min=8"`
}

type RegisterTenantResponse struct {
	TenantID            string `json:"tenant_id"`
	OrganizationPartyID string `json:"organization_party_id"`
	AdminPartyID        string `json:"admin_party_id"`
	Slug                string `json:"slug"`
}
