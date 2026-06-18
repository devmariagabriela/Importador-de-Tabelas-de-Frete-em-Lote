package service

import (
	"bytes"
	"strings"
	"testing"

	"desafio-importador-frete/internal/model"
	"github.com/xuri/excelize/v2"
)

func TestValidateLine(t *testing.T) {
	tests := []struct {
		name string
		line importLine
		want string
	}{
		{
			name: "linha valida",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0", "10", "45.90"}},
			want: "",
		},
		{
			name: "origem obrigatória",
			line: importLine{raw: []string{"", "RIO DE JANEIRO", "0", "10", "45.90"}},
			want: "Origem é obrigatória",
		},
		{
			name: "peso min obrigatório",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "", "10", "45.90"}},
			want: "Peso Mínimo é obrigatório",
		},
		{
			name: "peso max obrigatório",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0", "", "45.90"}},
			want: "Peso Máximo é obrigatório",
		},
		{
			name: "valor obrigatório",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0", "10", ""}},
			want: "Valor é obrigatório",
		},
		{
			name: "peso max menor que min",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "10", "5", "45.90"}},
			want: "Peso Máximo deve ser maior que Peso Mínimo",
		},
		{
			name: "valor inválido",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0", "10", "0"}},
			want: "Valor deve ser maior que zero",
		},
		{
			name: "peso negativo",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "-1", "10", "45.90"}},
			want: "Peso não pode ser negativo",
		},
		{
			name: "duplicada",
			line: importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0", "10", "45.90"}, duplicate: true},
			want: "Linha duplicada para mesma origem, destino e faixa de peso",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := validateLine(tt.line, 0)
			if got != tt.want {
				t.Fatalf("validateLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorField(t *testing.T) {
	tests := []struct {
		reason string
		want   string
	}{
		{reason: "Origem é obrigatória", want: "origem"},
		{reason: "Destino é obrigatório", want: "destino"},
		{reason: "Peso Máximo deve ser maior que Peso Mínimo", want: "peso_max"},
		{reason: "Peso não pode ser negativo", want: "peso"},
		{reason: "Valor deve ser maior que zero", want: "valor"},
		{reason: "Linha duplicada para mesma origem, destino e faixa de peso", want: "linha"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			if got := errorField(tt.reason); got != tt.want {
				t.Fatalf("errorField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateLineReturnsValidFreightRow(t *testing.T) {
	got, reason := validateLine(importLine{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0", "10", "45.90"}}, 0)
	if reason != "" {
		t.Fatalf("validateLine() reason = %q, want empty", reason)
	}
	if got.Origem != "SAO PAULO" || got.Destino != "RIO DE JANEIRO" || got.PesoMin != 0 || got.PesoMax != 10 || got.Valor != 45.90 {
		t.Fatalf("validateLine() valid row = %+v", got)
	}
}

func TestMarkDuplicates(t *testing.T) {
	lines := []importLine{
		{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0", "10", "45.90"}},
		{raw: []string{"sao paulo", "rio de janeiro", "0", "10", "99.90"}},
		{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "0.0", "10.0", "55.00"}},
		{raw: []string{"SAO PAULO", "RIO DE JANEIRO", "10", "20", "55.00"}},
	}

	markDuplicates(lines)

	if lines[0].duplicate {
		t.Fatal("primeira ocorrência não deve ser marcada como duplicada")
	}
	if !lines[1].duplicate {
		t.Fatal("segunda ocorrência da mesma faixa deve ser marcada como duplicada")
	}
	if !lines[2].duplicate {
		t.Fatal("faixa numericamente equivalente deve ser marcada como duplicada")
	}
	if lines[3].duplicate {
		t.Fatal("faixa diferente não deve ser marcada como duplicada")
	}
}

func TestChunkLines(t *testing.T) {
	lines := []importLine{{number: 1}, {number: 2}, {number: 3}, {number: 4}, {number: 5}}

	chunks := chunkLines(lines, 2)
	if len(chunks) != 2 {
		t.Fatalf("chunkLines() returned %d chunks, want 2", len(chunks))
	}
	if len(chunks[0]) != 3 || len(chunks[1]) != 2 {
		t.Fatalf("chunkLines() sizes = %d, %d; want 3, 2", len(chunks[0]), len(chunks[1]))
	}
	if chunks[0][0].number != 1 || chunks[1][1].number != 5 {
		t.Fatalf("chunkLines() did not preserve line order: %+v", chunks)
	}
}

func TestReadCSV(t *testing.T) {
	content := strings.NewReader("origem,destino,peso_min,peso_max,valor\nSAO PAULO,RIO DE JANEIRO,0,10,45.90\n")

	lines, err := readCSV(content, 2)
	if err != nil {
		t.Fatalf("readCSV() error = %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("readCSV() returned %d lines, want 1", len(lines))
	}
	if lines[0].number != 2 || lines[0].raw[0] != "SAO PAULO" || lines[0].raw[4] != "45.90" {
		t.Fatalf("readCSV() line = %+v", lines[0])
	}
}

func TestReadXLSX(t *testing.T) {
	workbook := excelize.NewFile()
	sheet := workbook.GetSheetName(0)
	rows := [][]interface{}{
		{"origem", "destino", "peso_min", "peso_max", "valor"},
		{"SAO PAULO", "RIO DE JANEIRO", 0, 10, 45.90},
	}
	for rowIndex, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, rowIndex+1)
		if err != nil {
			t.Fatalf("CoordinatesToCellName() error = %v", err)
		}
		if err := workbook.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatalf("SetSheetRow() error = %v", err)
		}
	}

	var buffer bytes.Buffer
	if err := workbook.Write(&buffer); err != nil {
		t.Fatalf("workbook.Write() error = %v", err)
	}

	lines, err := readXLSX(&buffer, 2)
	if err != nil {
		t.Fatalf("readXLSX() error = %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("readXLSX() returned %d lines, want 1", len(lines))
	}
	if lines[0].number != 2 || lines[0].raw[0] != "SAO PAULO" || lines[0].raw[4] != "45.9" {
		t.Fatalf("readXLSX() line = %+v", lines[0])
	}
}

func TestImportProgress(t *testing.T) {
	importacao := model.Importacao{
		TotalLinhas:       4,
		LinhasProcessadas: 3,
	}

	if got := importacao.Progresso(); got != 75 {
		t.Fatalf("Progresso() = %v, want 75", got)
	}
}

func TestPaginateErrors(t *testing.T) {
	errorsList := []model.LinhaErro{
		{NumeroLinha: 2},
		{NumeroLinha: 3},
		{NumeroLinha: 4},
	}

	got := paginateErrors(errorsList, 2, 2)
	if len(got) != 1 || got[0].NumeroLinha != 4 {
		t.Fatalf("paginateErrors() = %+v, want only line 4", got)
	}

	if pages := totalPages(len(errorsList), 2); pages != 2 {
		t.Fatalf("totalPages() = %d, want 2", pages)
	}
}
