package service

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"
	"sync"
	"time"

	"desafio-importador-frete/internal/dto"
	"desafio-importador-frete/internal/model"
	"desafio-importador-frete/internal/repository"
)

const expectedColumns = 5

var expectedHeader = []string{"origem", "destino", "peso_min", "peso_max", "valor"}

type ImportService interface {
	CreateImport(files []*multipart.FileHeader) (dto.ImportCreatedResponse, error)
	ListImports() (dto.ImportacoesResponse, error)
	GetErrors(id string, page int, limit int) (dto.ErrosResponse, error)
	GetValidRows(id string) (dto.LinhasValidasResponse, error)
}

type ImportadorService struct {
	repo            repository.ImportRepository
	workers         int
	validationDelay time.Duration
}

type csvLine struct {
	number    int
	raw       []string
	duplicate bool
}

type validationResult struct {
	line   csvLine
	reason string
	valid  model.FreightRow
}

func NewImportadorService(repo repository.ImportRepository, workers int) *ImportadorService {
	return NewImportadorServiceWithDelay(repo, workers, 10*time.Millisecond)
}

func NewImportadorServiceWithDelay(repo repository.ImportRepository, workers int, delay time.Duration) *ImportadorService {
	if workers <= 0 {
		workers = 1
	}
	return &ImportadorService{
		repo:            repo,
		workers:         workers,
		validationDelay: delay,
	}
}

func (s *ImportadorService) CreateImport(files []*multipart.FileHeader) (dto.ImportCreatedResponse, error) {
	if len(files) == 0 {
		return dto.ImportCreatedResponse{}, ErrInvalidUpload
	}

	id := newImportID()
	importacao := model.Importacao{
		ID:     id,
		Status: model.StatusPending,
	}
	if err := s.repo.Create(importacao); err != nil {
		return dto.ImportCreatedResponse{}, err
	}

	go s.processFiles(id, files)

	return dto.ImportCreatedResponse{ID: id}, nil
}

func (s *ImportadorService) ListImports() (dto.ImportacoesResponse, error) {
	imports, err := s.repo.List()
	if err != nil {
		return dto.ImportacoesResponse{}, err
	}

	out := make([]dto.ImportacaoResponse, 0, len(imports))
	for _, item := range imports {
		out = append(out, dto.ImportacaoResponse{
			ID:                item.ID,
			Status:            item.Status.String(),
			TotalLinhas:       item.TotalLinhas,
			LinhasProcessadas: item.LinhasProcessadas,
			Validas:           item.Validas,
			Invalidas:         item.Invalidas,
			Progresso:         item.Progresso(),
			CriadaEm:          item.CreatedAt,
			AtualizadaEm:      item.UpdatedAt,
		})
	}

	return dto.ImportacoesResponse{Data: out}, nil
}

func (s *ImportadorService) GetErrors(id string, page int, limit int) (dto.ErrosResponse, error) {
	errorsList, err := s.repo.GetErrors(id)
	if err != nil {
		return dto.ErrosResponse{}, err
	}

	total := len(errorsList)
	pagedErrors := paginateErrors(errorsList, page, limit)

	out := make([]dto.LinhaErroResponse, 0, len(pagedErrors))
	for _, item := range pagedErrors {
		out = append(out, dto.LinhaErroResponse{
			NumeroLinha:    item.NumeroLinha,
			DadosOriginais: item.DadosOriginais,
			Motivo:         item.Motivo,
		})
	}

	response := dto.ErrosResponse{Data: out}
	if page > 0 && limit > 0 {
		response.Page = page
		response.Limit = limit
		response.Total = total
		response.TotalPages = totalPages(total, limit)
	}

	return response, nil
}

func (s *ImportadorService) GetValidRows(id string) (dto.LinhasValidasResponse, error) {
	validRows, err := s.repo.GetValidRows(id)
	if err != nil {
		return dto.LinhasValidasResponse{}, err
	}

	out := make([]dto.LinhaValidaResponse, 0, len(validRows))
	for _, item := range validRows {
		out = append(out, dto.LinhaValidaResponse{
			NumeroLinha: item.NumeroLinha,
			Origem:      item.DadosOrigem.Origem,
			Destino:     item.DadosOrigem.Destino,
			PesoMin:     item.DadosOrigem.PesoMin,
			PesoMax:     item.DadosOrigem.PesoMax,
			Valor:       item.DadosOrigem.Valor,
		})
	}

	return dto.LinhasValidasResponse{
		Data:  out,
		Total: len(out),
	}, nil
}

func (s *ImportadorService) processFiles(importID string, files []*multipart.FileHeader) {
	if err := s.repo.SetStatus(importID, model.StatusProcessing); err != nil {
		return
	}

	lines, err := readCSVFiles(files)
	if err != nil {
		_ = s.repo.SetStatus(importID, model.StatusFailed)
		_ = s.repo.AddError(model.LinhaErro{
			ImportID:       importID,
			NumeroLinha:    0,
			DadosOriginais: nil,
			Motivo:         err.Error(),
		})
		return
	}

	markDuplicates(lines)
	if err := s.repo.SetTotal(importID, len(lines)); err != nil {
		return
	}

	s.runWorkerPool(importID, lines)
	_ = s.repo.SetStatus(importID, model.StatusCompleted)
}

func (s *ImportadorService) runWorkerPool(importID string, lines []csvLine) {
	jobs := make(chan csvLine)
	results := make(chan validationResult)

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range jobs {
				valid, reason := validateLine(line, s.validationDelay)
				results <- validationResult{
					line:   line,
					reason: reason,
					valid:  valid,
				}
			}
		}()
	}

	go func() {
		for _, line := range lines {
			jobs <- line
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.reason == "" {
			_ = s.repo.AddValid(model.LinhaValida{
				ImportID:    importID,
				NumeroLinha: result.line.number,
				DadosOrigem: result.valid,
			})
			_ = s.repo.IncrementCounters(importID, 1, 0)
			continue
		}

		_ = s.repo.AddError(model.LinhaErro{
			ImportID:       importID,
			NumeroLinha:    result.line.number,
			DadosOriginais: append([]string(nil), result.line.raw...),
			Motivo:         result.reason,
		})
		_ = s.repo.IncrementCounters(importID, 0, 1)
	}
}

func readCSVFiles(files []*multipart.FileHeader) ([]csvLine, error) {
	var lines []csvLine
	nextLine := 2

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: nao foi possivel abrir arquivo", ErrInvalidCSV)
		}

		fileLines, err := readCSV(file, nextLine)
		closeErr := file.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, fmt.Errorf("%w: nao foi possivel fechar arquivo", ErrInvalidCSV)
		}

		lines = append(lines, fileLines...)
		nextLine += len(fileLines)
	}

	return lines, nil
}

func readCSV(reader io.Reader, firstDataLine int) ([]csvLine, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: arquivo vazio", ErrInvalidCSV)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: falha ao ler cabecalho", ErrInvalidCSV)
	}
	if !validHeader(header) {
		return nil, fmt.Errorf("%w: cabecalho esperado origem,destino,peso_min,peso_max,valor", ErrInvalidCSV)
	}

	var lines []csvLine
	lineNumber := firstDataLine
	for {
		row, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			lines = append(lines, csvLine{
				number: lineNumber,
				raw:    []string{},
			})
			lineNumber++
			continue
		}
		lines = append(lines, csvLine{
			number: lineNumber,
			raw:    normalizeRecordLength(row),
		})
		lineNumber++
	}

	return lines, nil
}

func validHeader(header []string) bool {
	if len(header) != len(expectedHeader) {
		return false
	}
	for i, column := range expectedHeader {
		if strings.ToLower(strings.TrimSpace(header[i])) != column {
			return false
		}
	}
	return true
}

func normalizeRecordLength(row []string) []string {
	out := make([]string, expectedColumns)
	copy(out, row)
	if len(row) > expectedColumns {
		return append(out, row[expectedColumns:]...)
	}
	return out
}

func markDuplicates(lines []csvLine) {
	seen := make(map[string]struct{}, len(lines))
	for i := range lines {
		key, ok := duplicateKey(lines[i].raw)
		if !ok {
			continue
		}
		if _, exists := seen[key]; exists {
			lines[i].duplicate = true
			continue
		}
		seen[key] = struct{}{}
	}
}

func duplicateKey(row []string) (string, bool) {
	if len(row) < expectedColumns {
		return "", false
	}
	fields := row[:expectedColumns]
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			return "", false
		}
	}

	pesoMin, err := parseDecimal(fields[2])
	if err != nil {
		return "", false
	}
	pesoMax, err := parseDecimal(fields[3])
	if err != nil {
		return "", false
	}

	return strings.Join([]string{
		normalizeText(fields[0]),
		normalizeText(fields[1]),
		normalizeDecimal(pesoMin),
		normalizeDecimal(pesoMax),
	}, "|"), true
}

func validateLine(line csvLine, delay time.Duration) (model.FreightRow, string) {
	if delay > 0 {
		time.Sleep(delay)
	}

	if line.duplicate {
		return model.FreightRow{}, "linha duplicada para mesma origem, destino e faixa de peso"
	}
	if len(line.raw) != expectedColumns {
		return model.FreightRow{}, "linha deve conter exatamente 5 colunas"
	}

	origem := strings.TrimSpace(line.raw[0])
	destino := strings.TrimSpace(line.raw[1])
	pesoMinRaw := strings.TrimSpace(line.raw[2])
	pesoMaxRaw := strings.TrimSpace(line.raw[3])
	valorRaw := strings.TrimSpace(line.raw[4])

	if origem == "" {
		return model.FreightRow{}, "origem e obrigatoria"
	}
	if destino == "" {
		return model.FreightRow{}, "destino e obrigatorio"
	}
	if pesoMinRaw == "" {
		return model.FreightRow{}, "peso_min e obrigatorio"
	}
	if pesoMaxRaw == "" {
		return model.FreightRow{}, "peso_max e obrigatorio"
	}
	if valorRaw == "" {
		return model.FreightRow{}, "valor e obrigatorio"
	}

	pesoMin, err := parseDecimal(pesoMinRaw)
	if err != nil {
		return model.FreightRow{}, "peso_min deve ser numerico"
	}
	pesoMax, err := parseDecimal(pesoMaxRaw)
	if err != nil {
		return model.FreightRow{}, "peso_max deve ser numerico"
	}
	valor, err := parseDecimal(valorRaw)
	if err != nil {
		return model.FreightRow{}, "valor deve ser numerico"
	}

	if pesoMin < 0 || pesoMax < 0 {
		return model.FreightRow{}, "peso nao pode ser negativo"
	}
	if pesoMax <= pesoMin {
		return model.FreightRow{}, "peso_max deve ser maior que peso_min"
	}
	if valor <= 0 {
		return model.FreightRow{}, "valor deve ser maior que zero"
	}

	return model.FreightRow{
		Origem:  origem,
		Destino: destino,
		PesoMin: pesoMin,
		PesoMax: pesoMax,
		Valor:   valor,
	}, ""
}

func parseDecimal(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

func normalizeText(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func paginateErrors(errorsList []model.LinhaErro, page int, limit int) []model.LinhaErro {
	if page <= 0 || limit <= 0 {
		return errorsList
	}

	start := (page - 1) * limit
	if start >= len(errorsList) {
		return []model.LinhaErro{}
	}

	end := start + limit
	if end > len(errorsList) {
		end = len(errorsList)
	}

	return errorsList[start:end]
}

func totalPages(total int, limit int) int {
	if total == 0 || limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}

func newImportID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
