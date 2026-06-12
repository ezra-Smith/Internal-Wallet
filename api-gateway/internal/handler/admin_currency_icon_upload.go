package handler

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"internalwallet/api-gateway/internal/storage"
	"internalwallet/proto/pb"
)

const currencyIconUploadMaxBytes = 2 << 20 // 2MB

type AdminCurrencyIconUploadHandler struct {
	uploader    *storage.S3Uploader
	adminClient pb.AdminClient
}

func NewAdminCurrencyIconUploadHandler(uploader *storage.S3Uploader, adminClient pb.AdminClient) *AdminCurrencyIconUploadHandler {
	return &AdminCurrencyIconUploadHandler{
		uploader:    uploader,
		adminClient: adminClient,
	}
}

func (h *AdminCurrencyIconUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.uploader == nil {
		WriteError(w, "Upload service not available", http.StatusServiceUnavailable)
		return
	}
	if h.adminClient == nil {
		WriteError(w, "Admin service not available", http.StatusServiceUnavailable)
		return
	}

	// Permission gate: align with Admin.UpdateCurrencyConfig (legacy permission code)
	rbacResp, err := h.adminClient.GetMyRBAC(r.Context(), &pb.GetMyRBACRequest{})
	if err != nil {
		WriteRPCError(w, r, err)
		return
	}
	if !rbacResp.GetSuccess() || rbacResp.GetData() == nil {
		WriteError(w, "Forbidden", http.StatusForbidden)
		return
	}
	if !hasPermission(rbacResp.GetData().GetUserPermissionCodes(), "rpc:UpdateCurrencyConfig") && !hasPermission(rbacResp.GetData().GetUserPermissionCodes(), "rpc:CreateCurrency") {
		WriteError(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(currencyIconUploadMaxBytes); err != nil {
		WriteError(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, currencyIconUploadMaxBytes+1))
	if err != nil {
		WriteError(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	if int64(len(data)) > currencyIconUploadMaxBytes {
		WriteError(w, "File too large", http.StatusBadRequest)
		return
	}

	contentType, ext, ok := detectCurrencyIconContentType(data)
	if !ok {
		WriteError(w, "Unsupported file type", http.StatusBadRequest)
		return
	}

	assetCode := normalizeAssetCode(r.FormValue("asset_code"))
	if strings.TrimSpace(r.FormValue("asset_code")) != "" && assetCode == "" {
		WriteError(w, "Invalid asset_code", http.StatusBadRequest)
		return
	}

	key := h.uploader.CurrencyIconObjectKey(assetCode, ext)
	url, err := h.uploader.Upload(r.Context(), key, contentType, data)
	if err != nil {
		WriteError(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]string{"url": url})
}

func normalizeAssetCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if len(s) > 32 {
		return ""
	}
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return ""
	}
	return s
}

func detectCurrencyIconContentType(data []byte) (contentType string, ext string, ok bool) {
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

func hasPermission(codes []string, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	for _, c := range codes {
		cc := strings.TrimSpace(c)
		if cc == "" {
			continue
		}
		if cc == "*" || cc == required {
			return true
		}
	}
	return false
}
