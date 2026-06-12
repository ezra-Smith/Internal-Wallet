package handler

import (
	"io"
	"net/http"
	"strings"

	"internalwallet/api-gateway/internal/storage"
	"internalwallet/proto/pb"
)

const chainIconUploadMaxBytes = 2 << 20 // 2MB

type AdminChainIconUploadHandler struct {
	uploader    *storage.S3Uploader
	adminClient pb.AdminClient
}

func NewAdminChainIconUploadHandler(uploader *storage.S3Uploader, adminClient pb.AdminClient) *AdminChainIconUploadHandler {
	return &AdminChainIconUploadHandler{
		uploader:    uploader,
		adminClient: adminClient,
	}
}

func (h *AdminChainIconUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Permission gate: reuse UpdateCurrencyConfig permission (same page capability).
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

	if err := r.ParseMultipartForm(chainIconUploadMaxBytes); err != nil {
		WriteError(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, chainIconUploadMaxBytes+1))
	if err != nil {
		WriteError(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	if int64(len(data)) > chainIconUploadMaxBytes {
		WriteError(w, "File too large", http.StatusBadRequest)
		return
	}

	contentType, ext, ok := detectCurrencyIconContentType(data)
	if !ok {
		WriteError(w, "Unsupported file type", http.StatusBadRequest)
		return
	}

	chainCode := normalizeChainCode(r.FormValue("chain_code"))
	if strings.TrimSpace(r.FormValue("chain_code")) != "" && chainCode == "" {
		WriteError(w, "Invalid chain_code", http.StatusBadRequest)
		return
	}

	key := h.uploader.ChainIconObjectKey(chainCode, ext)
	url, err := h.uploader.Upload(r.Context(), key, contentType, data)
	if err != nil {
		WriteError(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]string{"url": url})
}

func normalizeChainCode(s string) string {
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
