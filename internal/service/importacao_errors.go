package service

import "errors"

var (
	ErrInvalidUpload = errors.New("envie ao menos um arquivo CSV no campo files")
	ErrInvalidCSV    = errors.New("csv inválido")
)
