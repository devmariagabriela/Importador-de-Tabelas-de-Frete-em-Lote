package model

import "time"

type ImportStatus int

const (
	StatusPending ImportStatus = iota
	StatusProcessing
	StatusCompleted
	StatusFailed
)

func (s ImportStatus) String() string {
	switch s {
	case StatusPending:
		return "PENDENTE"
	case StatusProcessing:
		return "PROCESSANDO"
	case StatusCompleted:
		return "CONCLUIDA"
	case StatusFailed:
		return "FALHOU"
	default:
		return "DESCONHECIDO"
	}
}

type Importacao struct {
	ID                string
	Status            ImportStatus
	TotalLinhas       int
	LinhasProcessadas int
	Validas           int
	Invalidas         int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (i Importacao) Progresso() float64 {
	if i.TotalLinhas == 0 {
		if i.Status == StatusCompleted {
			return 100
		}
		return 0
	}
	return float64(i.LinhasProcessadas) * 100 / float64(i.TotalLinhas)
}

type LinhaErro struct {
	ImportID       string
	NumeroLinha    int
	DadosOriginais []string
	Motivo         string
	CreatedAt      time.Time
}

type LinhaValida struct {
	ImportID    string
	NumeroLinha int
	DadosOrigem FreightRow
	CreatedAt   time.Time
}

type FreightRow struct {
	Origem  string
	Destino string
	PesoMin float64
	PesoMax float64
	Valor   float64
}
