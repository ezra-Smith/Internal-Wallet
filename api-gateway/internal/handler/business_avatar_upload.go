package handler

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"internalwallet/api-gateway/internal/storage"
	"internalwallet/common/middleware"
)

const businessAvatarUploadMaxBytes = 2 << 20 // 2MB

type BusinessAvatarUploadHandler struct {
	uploader *storage.S3Uploader
}

func NewBusinessAvatarUploadHandler(uploader *storage.S3Uploader) *BusinessAvatarUploadHandler {
	return &BusinessAvatarUploadHandler{
		uploader: uploader,
	}
}

func (h *BusinessAvatarUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.uploader == nil {
		WriteError(w, "Upload service not available", http.StatusServiceUnavailable)
		return
	}

	uidStr := strings.TrimSpace(middleware.GetUserID(r.Context()))
	if uidStr == "" {
		WriteError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		WriteError(w, "Invalid user id", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(businessAvatarUploadMaxBytes); err != nil {
		WriteError(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, businessAvatarUploadMaxBytes+1))
	if err != nil {
		WriteError(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	if int64(len(data)) > businessAvatarUploadMaxBytes {
		WriteError(w, "File too large", http.StatusBadRequest)
		return
	}

	contentType, ext, ok := detectAvatarContentType(data)
	if !ok {
		WriteError(w, "Unsupported file type", http.StatusBadRequest)
		return
	}

	key := h.uploader.UserAvatarObjectKey(uid, ext)
	url, err := h.uploader.Upload(r.Context(), key, contentType, data)
	if err != nil {
		WriteError(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]string{"avatar_url": url})
}

func detectAvatarContentType(data []byte) (contentType string, ext string, ok bool) {
	if len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp", ".webp", true
	}
	switch http.DetectContentType(data) {
	case "image/png":
		return "image/png", ".png", true
	case "image/jpeg":
		return "image/jpeg", ".jpg", true
	case "image/gif":
		return "image/gif", ".gif", true
	default:
		return "", "", false
	}
}
