package dto

import (
	"encoding/hex"
	"time"

	"enterprise-system/internal/domain"
)

// AttachedDocumentResponse is the public representation of document metadata.
// Storage internals are not exposed.
type AttachedDocumentResponse struct {
	ID           string     `json:"id"`
	OriginTable  string     `json:"origin_table"`
	OriginID     string     `json:"origin_id"`
	OriginalName string     `json:"original_name"`
	MimeType     string     `json:"mime_type"`
	SizeBytes    int64      `json:"size_bytes"`
	DocumentHash string     `json:"document_hash"`
	Description  *string    `json:"description,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// DocumentEnvelope is the standard single-resource envelope.
type DocumentEnvelope struct {
	Data AttachedDocumentResponse `json:"data"`
}

func NewAttachedDocumentResponse(document *domain.AttachedDocument) AttachedDocumentResponse {
	if document == nil {
		return AttachedDocumentResponse{}
	}
	return AttachedDocumentResponse{
		ID:           document.ID.String(),
		OriginTable:  document.OriginTable,
		OriginID:     document.OriginID.String(),
		OriginalName: document.OriginalName,
		MimeType:     document.MimeType,
		SizeBytes:    document.SizeBytes,
		DocumentHash: hex.EncodeToString(document.DocumentHash),
		Description:  document.Description,
		CreatedAt:    document.CreatedAt,
		DeletedAt:    document.DeletedAt,
	}
}
