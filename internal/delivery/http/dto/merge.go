package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type MergePartiesRequest struct {
	SourcePartyID string `json:"source_party_id" binding:"required"`
	TargetPartyID string `json:"target_party_id" binding:"required"`
}

type MergePartiesResponse struct {
	SourceID           string    `json:"source_id"`
	TargetID           string    `json:"target_id"`
	CompressedPartyIDs []string  `json:"compressed_party_ids"`
	MergedAt           time.Time `json:"merged_at"`
}

type MergePartiesEnvelope struct {
	Data MergePartiesResponse `json:"data"`
}

func NewMergePartiesResponse(result *domain.MergeResult) MergePartiesResponse {
	if result == nil {
		return MergePartiesResponse{}
	}
	compressed := make([]string, 0, len(result.CompressedPartyIDs))
	for _, id := range result.CompressedPartyIDs {
		compressed = append(compressed, id.String())
	}
	return MergePartiesResponse{
		SourceID: result.SourceID.String(), TargetID: result.TargetID.String(),
		CompressedPartyIDs: compressed, MergedAt: result.MergedAt,
	}
}
