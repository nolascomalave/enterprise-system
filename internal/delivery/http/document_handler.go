package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxDocumentBytes     = 10 << 20 // 10 MB
	maxMultipartMemory   = 1 << 20  // spill most files to disk
	maxOriginTableLength = 128
	maxDescriptionLength = 2000
)

var originTablePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// allowedDocumentMIMEs is the magic-byte whitelist accepted by the endpoint.
var allowedDocumentMIMEs = map[string]struct{}{
	"application/pdf": {},
	"image/png":       {},
	"image/jpeg":      {},
	"image/gif":       {},
	"image/webp":      {},
	"text/plain":      {},
	"application/zip": {},
}

type DocumentUseCase interface {
	Upload(ctx context.Context, in usecase.UploadDocumentInput) (*domain.AttachedDocument, error)
}

type DocumentHandler struct {
	uc DocumentUseCase
}

func NewDocumentHandler(uc DocumentUseCase) *DocumentHandler {
	return &DocumentHandler{uc: uc}
}

// Create godoc
// @Summary      Upload an attached document
// @Description  Stores a document in content-addressed storage and records its metadata.
// @Tags         Documents
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file formData file true "Document (max 10 MB)"
// @Param        origin_table formData string true "Business table that owns the document"
// @Param        origin_id formData string true "ID of the origin row"
// @Param        description formData string false "Optional description"
// @Success      201 {object} dto.DocumentEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /documents [post]
func (h *DocumentHandler) Create(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDocumentBytes)
	if err := c.Request.ParseMultipartForm(maxMultipartMemory); err != nil {
		c.Error(badRequestError(fmt.Errorf("parse multipart form: %w", err)))
		return
	}
	defer c.Request.MultipartForm.RemoveAll()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(apperror.NewValidation("invalid_document", "file is required", err))
		return
	}
	if fileHeader.Size > maxDocumentBytes {
		c.Error(apperror.NewValidation("document_too_large", "document exceeds 10 MB", nil))
		return
	}

	name, err := safeOriginalName(fileHeader.Filename)
	if err != nil {
		c.Error(apperror.NewValidation("invalid_document_name", "invalid file name", err))
		return
	}

	originTable := strings.TrimSpace(c.PostForm("origin_table"))
	if originTable == "" || len(originTable) > maxOriginTableLength || !originTablePattern.MatchString(originTable) {
		c.Error(apperror.NewValidation("invalid_origin", "invalid origin_table", nil))
		return
	}
	originID, err := uuid.Parse(strings.TrimSpace(c.PostForm("origin_id")))
	if err != nil || originID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_origin", "invalid origin_id", err))
		return
	}
	description := strings.TrimSpace(c.PostForm("description"))
	if len(description) > maxDescriptionLength {
		c.Error(apperror.NewValidation("invalid_description", "description is too long", nil))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.Error(apperror.NewValidation("invalid_document", "cannot read uploaded file", err))
		return
	}
	defer file.Close()

	mimeType, err := detectDocumentMIME(file)
	if err != nil {
		c.Error(apperror.NewValidation("invalid_document", "cannot inspect file content", err))
		return
	}
	if _, allowed := allowedDocumentMIMEs[mimeType]; !allowed {
		c.Error(apperror.NewValidation("unsupported_document_type", "unsupported document type", nil))
		return
	}

	document, err := h.uc.Upload(c.Request.Context(), usecase.UploadDocumentInput{
		TenantID:     sessionContext.TenantID,
		OriginTable:  originTable,
		OriginID:     originID,
		OriginalName: name,
		MimeType:     mimeType,
		Description:  description,
		Reader:       file,
	})
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.DocumentEnvelope{Data: dto.NewAttachedDocumentResponse(document)})
}

func safeOriginalName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.New("empty file name")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", errors.New("file name must not contain path separators")
	}
	base := filepath.Base(name)
	if base == "." || base == ".." || base != name {
		return "", errors.New("invalid file name")
	}
	return base, nil
}

// detectDocumentMIME reads the first 512 bytes (magic bytes), rewinds the
// stream and returns the normalized MIME type.
func detectDocumentMIME(file multipartFile) (string, error) {
	buffer := make([]byte, 512)
	n, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	if n == 0 {
		return "", errors.New("empty file")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	detected := http.DetectContentType(buffer[:n])
	normalized, _, err := mime.ParseMediaType(detected)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

type multipartFile interface {
	io.Reader
	io.Seeker
}
