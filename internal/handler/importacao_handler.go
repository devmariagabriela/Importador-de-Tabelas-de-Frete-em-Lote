package handler

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"desafio-importador-frete/internal/dto"
	"desafio-importador-frete/internal/repository"
	"desafio-importador-frete/internal/service"
)

type ImportacaoHandler struct {
	service service.ImportService
}

func NewImportacaoHandler(service service.ImportService) *ImportacaoHandler {
	return &ImportacaoHandler{service: service}
}

func (h *ImportacaoHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/importar", h.CreateImport)
	mux.HandleFunc("GET /api/importacoes", h.ListImports)
	mux.HandleFunc("GET /api/importacoes/", h.GetImportResource)
	mux.HandleFunc("GET /api/health", h.Health)
}

func (h *ImportacaoHandler) CreateImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "multipart/form-data inválido")
		return
	}

	files := collectFiles(r.MultipartForm.File)
	response, err := h.service.CreateImport(files)
	if err != nil {
		if errors.Is(err, service.ErrInvalidUpload) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "erro ao criar importação")
		return
	}

	writeJSON(w, http.StatusAccepted, response)
}

func (h *ImportacaoHandler) ListImports(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.ListImports()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao listar importações")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *ImportacaoHandler) GetImportResource(w http.ResponseWriter, r *http.Request) {
	id, resource, ok := extractImportResource(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "rota não encontrada")
		return
	}

	switch resource {
	case "erros":
		h.GetErrors(w, r, id)
	case "validas":
		h.GetValidRows(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "rota não encontrada")
	}
}

func (h *ImportacaoHandler) GetErrors(w http.ResponseWriter, r *http.Request, id string) {
	page := parsePositiveQueryInt(r, "page")
	limit := parsePositiveQueryInt(r, "limit")

	response, err := h.service.GetErrors(id, page, limit)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "importação não encontrada")
			return
		}
		writeError(w, http.StatusInternalServerError, "erro ao listar erros")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *ImportacaoHandler) GetValidRows(w http.ResponseWriter, r *http.Request, id string) {
	response, err := h.service.GetValidRows(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "importação não encontrada")
			return
		}
		writeError(w, http.StatusInternalServerError, "erro ao listar linhas válidas")
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="linhas-validas-`+id+`.json"`)
	writeJSON(w, http.StatusOK, response)
}

func (h *ImportacaoHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func collectFiles(filesByField map[string][]*multipart.FileHeader) []*multipart.FileHeader {
	var files []*multipart.FileHeader
	files = append(files, filesByField["files"]...)
	files = append(files, filesByField["file"]...)
	return files
}

func extractImportResource(path string) (string, string, bool) {
	const prefix = "/api/importacoes/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, prefix), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func parsePositiveQueryInt(r *http.Request, key string) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0
	}

	return value
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, dto.ErrorResponse{Error: message})
}
