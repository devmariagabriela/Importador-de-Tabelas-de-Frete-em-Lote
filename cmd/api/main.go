package main

import (
	"log"
	"net/http"

	"desafio-importador-frete/internal/config"
	"desafio-importador-frete/internal/handler"
	"desafio-importador-frete/internal/middleware"
	"desafio-importador-frete/internal/repository"
	"desafio-importador-frete/internal/service"
)

func main() {
	cfg := config.Load()

	repo := repository.NewMemoryImportRepository()
	importService := service.NewImportadorService(repo, cfg.ImportWorkers)
	importHandler := handler.NewImportacaoHandler(importService)

	mux := http.NewServeMux()
	importHandler.Register(mux)

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: middleware.Chain(
			mux,
			middleware.Recover,
			middleware.Logger,
			middleware.CORS(cfg.CORSOrigin),
		),
	}

	log.Printf("API ouvindo na porta %s com %d workers", cfg.Port, cfg.ImportWorkers)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("erro ao iniciar servidor: %v", err)
	}
}
