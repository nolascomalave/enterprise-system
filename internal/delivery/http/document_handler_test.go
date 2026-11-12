package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDocumentUseCase struct {
	input usecase.UploadDocumentInput
	doc   *domain.AttachedDocument
	err   error
}

func (f *fakeDocumentUseCase) Upload(ctx context.Context, in usecase.UploadDocumentInput) (*domain.AttachedDocument, error) {
	f.input = in
	return f.doc, f.err
}

func documentTestRouter(tenantID uuid.UUID, handler *DocumentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorMiddleware())
	router.POST("/documents", func(c *gin.Context) {
		ctx := enttenant.WithSessionContext(c.Request.Context(), enttenant.SessionContext{
			TenantID: tenantID, UserID: uuid.New(), SessionID: uuid.New(),
		})
		c.Request = c.Request.WithContext(ctx)
		handler.Create(c)
	})
	return router
}

func multipartDocumentRequest(t *testing.T, filename string, content []byte, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/documents", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func decodeDocumentResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func TestDocumentHandlerUploadsValidFile(t *testing.T) {
	tenantID := uuid.New()
	originID := uuid.New()
	document := &domain.AttachedDocument{
		ID: uuid.New(), TenantID: tenantID, OriginTable: "parties", OriginID: originID,
		OriginalName: "photo.png", StorageKey: "abc", StorageProvider: "local",
		MimeType: "image/png", SizeBytes: 12, DocumentHash: []byte("12345678901234567890123456789012"),
	}
	uc := &fakeDocumentUseCase{doc: document}
	router := documentTestRouter(tenantID, NewDocumentHandler(uc))

	content := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	request := multipartDocumentRequest(t, "photo.png", content, map[string]string{
		"origin_table": "parties", "origin_id": originID.String(), "description": "profile photo",
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, tenantID, uc.input.TenantID)
	assert.Equal(t, "parties", uc.input.OriginTable)
	assert.Equal(t, originID, uc.input.OriginID)
	assert.Equal(t, "photo.png", uc.input.OriginalName)
	assert.Equal(t, "image/png", uc.input.MimeType)
	assert.Equal(t, "profile photo", uc.input.Description)
	got, err := io.ReadAll(uc.input.Reader)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	body := decodeDocumentResponse(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, document.ID.String(), data["id"])
}

func TestDocumentHandlerRejectsUnsafeFilename(t *testing.T) {
	tenantID := uuid.New()
	router := documentTestRouter(tenantID, NewDocumentHandler(&fakeDocumentUseCase{}))

	request := multipartDocumentRequest(t, `..\..\etc\passwd`, []byte("content"), map[string]string{
		"origin_table": "parties", "origin_id": uuid.New().String(),
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	body := decodeDocumentResponse(t, recorder)
	assert.Equal(t, "invalid_document_name", body["code"])
}

func TestDocumentHandlerRejectsUnsupportedMIME(t *testing.T) {
	tenantID := uuid.New()
	router := documentTestRouter(tenantID, NewDocumentHandler(&fakeDocumentUseCase{}))

	request := multipartDocumentRequest(t, "payload.bin", []byte{0x00, 0x01, 0x02, 0x03}, map[string]string{
		"origin_table": "parties", "origin_id": uuid.New().String(),
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestDocumentHandlerRejectsMissingOrigin(t *testing.T) {
	tenantID := uuid.New()
	router := documentTestRouter(tenantID, NewDocumentHandler(&fakeDocumentUseCase{}))

	request := multipartDocumentRequest(t, "photo.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), map[string]string{
		"origin_id": uuid.New().String(),
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestDocumentHandlerRejectsOversizedFile(t *testing.T) {
	tenantID := uuid.New()
	router := documentTestRouter(tenantID, NewDocumentHandler(&fakeDocumentUseCase{}))

	content := bytes.Repeat([]byte("a"), maxDocumentBytes+1)
	request := multipartDocumentRequest(t, "big.txt", content, map[string]string{
		"origin_table": "parties", "origin_id": uuid.New().String(),
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestDocumentHandlerRequiresSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewDocumentHandler(&fakeDocumentUseCase{})
	router := gin.New()
	router.Use(ErrorMiddleware())
	router.POST("/documents", handler.Create)

	request := multipartDocumentRequest(t, "photo.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), map[string]string{
		"origin_table": "parties", "origin_id": uuid.New().String(),
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
