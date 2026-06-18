package dto

import "time"

type ImportCreatedResponse struct {
	ID string `json:"id"`
}

type ImportacaoResponse struct {
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	TotalLinhas       int       `json:"total_linhas"`
	LinhasProcessadas int       `json:"linhas_processadas"`
	Validas           int       `json:"validas"`
	Invalidas         int       `json:"invalidas"`
	Progresso         float64   `json:"progresso"`
	CriadaEm          time.Time `json:"criada_em"`
	AtualizadaEm      time.Time `json:"atualizada_em"`
}

type ImportacoesResponse struct {
	Data []ImportacaoResponse `json:"data"`
}

type LinhaErroResponse struct {
	NumeroLinha    int      `json:"numero_linha"`
	DadosOriginais []string `json:"dados_originais"`
	Motivo         string   `json:"motivo"`
}

type ErrosResponse struct {
	Data       []LinhaErroResponse `json:"data"`
	Page       int                 `json:"page,omitempty"`
	Limit      int                 `json:"limit,omitempty"`
	Total      int                 `json:"total,omitempty"`
	TotalPages int                 `json:"total_pages,omitempty"`
}

type LinhaValidaResponse struct {
	NumeroLinha int     `json:"numero_linha"`
	Origem      string  `json:"origem"`
	Destino     string  `json:"destino"`
	PesoMin     float64 `json:"peso_min"`
	PesoMax     float64 `json:"peso_max"`
	Valor       float64 `json:"valor"`
}

type LinhasValidasResponse struct {
	Data  []LinhaValidaResponse `json:"data"`
	Total int                   `json:"total"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
