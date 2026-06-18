package handler

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	mux.HandleFunc("GET /api/importacoes/ws", h.StreamImports)
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

func (h *ImportacaoHandler) StreamImports(w http.ResponseWriter, r *http.Request) {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	if !h.writeImportsMessage(conn) {
		return
	}

	for range ticker.C {
		if !h.writeImportsMessage(conn) {
			return
		}
	}
}

func (h *ImportacaoHandler) writeImportsMessage(conn net.Conn) bool {
	response, err := h.service.ListImports()
	if err != nil {
		return writeWebSocketText(conn, []byte(`{"error":"erro ao listar importações"}`))
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return writeWebSocketText(conn, []byte(`{"error":"erro ao serializar importações"}`))
	}

	return writeWebSocketText(conn, payload)
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("upgrade websocket obrigatório")
	}
	if !headerContains(r.Header.Get("Connection"), "upgrade") {
		return nil, errors.New("connection upgrade obrigatório")
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, errors.New("sec-websocket-key obrigatório")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("websocket não suportado")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	accept := websocketAcceptKey(key)
	_, err = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	if err == nil {
		_, err = rw.WriteString("Upgrade: websocket\r\n")
	}
	if err == nil {
		_, err = rw.WriteString("Connection: Upgrade\r\n")
	}
	if err == nil {
		_, err = rw.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	}
	if err == nil {
		err = rw.Flush()
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

func websocketAcceptKey(key string) string {
	const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	hash := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func headerContains(value string, expected string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), expected) {
			return true
		}
	}
	return false
}

func writeWebSocketText(conn net.Conn, payload []byte) bool {
	frame := []byte{0x81}
	length := len(payload)
	switch {
	case length < 126:
		frame = append(frame, byte(length))
	case length <= 65535:
		frame = append(frame, 126, byte(length>>8), byte(length))
	default:
		frame = append(
			frame,
			127,
			byte(uint64(length)>>56),
			byte(uint64(length)>>48),
			byte(uint64(length)>>40),
			byte(uint64(length)>>32),
			byte(uint64(length)>>24),
			byte(uint64(length)>>16),
			byte(uint64(length)>>8),
			byte(uint64(length)),
		)
	}

	frame = append(frame, payload...)
	_, err := conn.Write(frame)
	return err == nil
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
