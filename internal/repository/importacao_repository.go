package repository

import (
	"errors"
	"sort"
	"sync"
	"time"

	"desafio-importador-frete/internal/model"
)

var ErrNotFound = errors.New("importação não encontrada")

type ImportRepository interface {
	Create(importacao model.Importacao) error
	SetStatus(id string, status model.ImportStatus) error
	SetTotal(id string, total int) error
	IncrementCounters(id string, validas int, invalidas int) error
	AddError(err model.LinhaErro) error
	AddValid(row model.LinhaValida) error
	List() ([]model.Importacao, error)
	GetErrors(importID string) ([]model.LinhaErro, error)
	GetValidRows(importID string) ([]model.LinhaValida, error)
}

type MemoryImportRepository struct {
	mu         sync.RWMutex
	imports    map[string]model.Importacao
	errorsByID map[string][]model.LinhaErro
	validByID  map[string][]model.LinhaValida
}

func NewMemoryImportRepository() *MemoryImportRepository {
	return &MemoryImportRepository{
		imports:    make(map[string]model.Importacao),
		errorsByID: make(map[string][]model.LinhaErro),
		validByID:  make(map[string][]model.LinhaValida),
	}
}

func (r *MemoryImportRepository) Create(importacao model.Importacao) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	importacao.CreatedAt = now
	importacao.UpdatedAt = now
	r.imports[importacao.ID] = importacao
	return nil
}

func (r *MemoryImportRepository) SetStatus(id string, status model.ImportStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	importacao, ok := r.imports[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	importacao.Status = status
	importacao.UpdatedAt = now
	if status == model.StatusProcessing && importacao.StartedAt.IsZero() {
		importacao.StartedAt = now
	}
	if status == model.StatusCompleted || status == model.StatusFailed {
		importacao.FinishedAt = now
	}
	r.imports[id] = importacao
	return nil
}

func (r *MemoryImportRepository) SetTotal(id string, total int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	importacao, ok := r.imports[id]
	if !ok {
		return ErrNotFound
	}
	importacao.TotalLinhas = total
	importacao.UpdatedAt = time.Now()
	r.imports[id] = importacao
	return nil
}

func (r *MemoryImportRepository) IncrementCounters(id string, validas int, invalidas int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	importacao, ok := r.imports[id]
	if !ok {
		return ErrNotFound
	}
	importacao.Validas += validas
	importacao.Invalidas += invalidas
	importacao.LinhasProcessadas += validas + invalidas
	importacao.UpdatedAt = time.Now()
	r.imports[id] = importacao
	return nil
}

func (r *MemoryImportRepository) AddError(err model.LinhaErro) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.imports[err.ImportID]; !ok {
		return ErrNotFound
	}
	err.CreatedAt = time.Now()
	r.errorsByID[err.ImportID] = append(r.errorsByID[err.ImportID], err)
	return nil
}

func (r *MemoryImportRepository) AddValid(row model.LinhaValida) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.imports[row.ImportID]; !ok {
		return ErrNotFound
	}
	row.CreatedAt = time.Now()
	r.validByID[row.ImportID] = append(r.validByID[row.ImportID], row)
	return nil
}

func (r *MemoryImportRepository) List() ([]model.Importacao, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	imports := make([]model.Importacao, 0, len(r.imports))
	for _, importacao := range r.imports {
		imports = append(imports, importacao)
	}
	sort.Slice(imports, func(i, j int) bool {
		return imports[i].CreatedAt.After(imports[j].CreatedAt)
	})
	return imports, nil
}

func (r *MemoryImportRepository) GetErrors(importID string) ([]model.LinhaErro, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.imports[importID]; !ok {
		return nil, ErrNotFound
	}
	errors := append([]model.LinhaErro(nil), r.errorsByID[importID]...)
	sort.Slice(errors, func(i, j int) bool {
		return errors[i].NumeroLinha < errors[j].NumeroLinha
	})
	return errors, nil
}

func (r *MemoryImportRepository) GetValidRows(importID string) ([]model.LinhaValida, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.imports[importID]; !ok {
		return nil, ErrNotFound
	}
	rows := append([]model.LinhaValida(nil), r.validByID[importID]...)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].NumeroLinha < rows[j].NumeroLinha
	})
	return rows, nil
}
