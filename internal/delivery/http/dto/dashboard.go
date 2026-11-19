package dto

import "time"

type DashboardRecentPartyResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PartyType string    `json:"party_type"`
	CreatedAt time.Time `json:"created_at"`
}

type DashboardSummaryResponse struct {
	TotalParties     int                            `json:"total_parties"`
	TotalEmployees   int                            `json:"total_employees"`
	TotalDepartments int                            `json:"total_departments"`
	RecentParties    []DashboardRecentPartyResponse `json:"recent_parties"`
}

type DashboardSummaryEnvelope struct {
	Data DashboardSummaryResponse `json:"data"`
}
