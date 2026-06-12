package handler

import (
	"io"
	"net/http"
	"strings"

	"internalwallet/proto/pb"
)

// AdminTransferImportPreviewHandler handles multipart upload for transfer batch import preview.
// It forwards the uploaded file bytes to Admin.ImportTransferBatch RPC with preview=true.
type AdminTransferImportPreviewHandler struct {
	client pb.AdminClient
}

func NewAdminTransferImportPreviewHandler(client pb.AdminClient) *AdminTransferImportPreviewHandler {
	return &AdminTransferImportPreviewHandler{client: client}
}

func (h *AdminTransferImportPreviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.client == nil {
		WriteError(w, "Admin service not available", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseMultipartForm(transferImportMaxBytes); err != nil {
		WriteError(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, transferImportMaxBytes+1))
	if err != nil {
		WriteError(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	if int64(len(data)) > transferImportMaxBytes {
		WriteError(w, "File too large", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	network := strings.TrimSpace(r.FormValue("network"))
	currency := strings.TrimSpace(r.FormValue("currency"))
	description := strings.TrimSpace(r.FormValue("description"))
	transferType := strings.TrimSpace(r.FormValue("transfer_type"))

	resp, rpcErr := h.client.ImportTransferBatch(r.Context(), &pb.ImportTransferBatchRequest{
		File:         data,
		Filename:     header.Filename,
		Name:         name,
		Network:      network,
		Currency:     currency,
		Description:  description,
		TransferType: transferType,
		Preview:      true,
	})
	if rpcErr != nil {
		WriteRPCError(w, r, rpcErr)
		return
	}
	WriteJSON(w, resp)
}
