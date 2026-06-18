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
	StartedAt         time.Time
	FinishedAt        time.Time
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

func (i Importacao) DuracaoMS() int64 {
	if i.StartedAt.IsZero() {
		return 0
	}

	end := i.FinishedAt
	if end.IsZero() {
		end = time.Now()
	}

	return end.Sub(i.StartedAt).Milliseconds()
}

type LinhaErro struct {
	ImportID       string
	NumeroLinha    int
	DadosOriginais []string
	Motivo         string
	Campo          string
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
