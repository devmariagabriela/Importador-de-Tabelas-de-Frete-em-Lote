# Importador de Tabelas de Frete em Lote

Aplicação web fullstack para upload e validação em lote de tabelas de frete em CSV.

O backend processa as linhas em background usando goroutines e um worker pool configurável. O frontend permite enviar um ou mais arquivos, acompanha o progresso da importação por polling e exibe os erros encontrados por linha.

## Stack

- Backend: Go 1.22 com `net/http`
- Frontend: Vue 3 com Composition API e Vite
- Infra: Docker Compose
- Armazenamento: memória da aplicação
- Banco de dados: não utilizado
- ORM: não utilizado

## Requisitos Atendidos

- Upload de um ou mais arquivos CSV
- Processamento concorrente com goroutines
- Worker pool configurável por variável de ambiente
- Validação com `time.Sleep(10 * time.Millisecond)` por linha
- Listagem de importações com status, contadores e percentual de progresso
- Listagem de linhas inválidas com número da linha, dados originais e motivo
- Paginação opcional das linhas inválidas
- Exportação das linhas válidas em JSON
- Armazenamento em memória protegido por mutex
- Frontend com upload, barra de progresso, resumo, tabela de erros paginada e botão de exportação
- Docker Compose com serviços de backend e frontend
- Massa de teste com erros propositais
- Testes unitários para regras de validação

## Arquitetura

O backend segue uma organização simples em camadas:

```text
Request
  -> Handler
  -> DTO
  -> Service
  -> Repository em memória
  -> Response
```

Estrutura principal:

```text
cmd/
  api/
    main.go
internal/
  config/
  dto/
  handler/
  middleware/
  model/
  repository/
  service/
frontend/
  src/
    App.vue
    main.js
    style.css
massa-teste/
  gerar-massa-teste.py
```

Responsabilidades:

- `cmd/api`: ponto de entrada da API.
- `internal/config`: leitura de variáveis de ambiente.
- `internal/handler`: rotas HTTP, multipart upload e respostas JSON.
- `internal/dto`: contratos de resposta da API.
- `internal/service`: regras de negócio, leitura do CSV, validações e worker pool.
- `internal/repository`: armazenamento em memória com `sync.RWMutex`.
- `internal/model`: entidades de domínio e status da importação.
- `internal/middleware`: CORS, log e recover.

## Como Rodar com Docker

Com Docker Compose v2:

```bash
cp .env.example .env
docker compose up -d
```

Se o ambiente tiver apenas Docker Compose v1:

```bash
cp .env.example .env
docker-compose up -d
```

Portas:

- Frontend: http://localhost:5173
- Backend: http://localhost:8080

Health check:

```bash
curl http://localhost:8080/api/health
```

Resposta esperada:

```json
{
  "status": "ok"
}
```

## Variáveis de Ambiente

Exemplo em `.env.example`:

```env
IMPORT_WORKERS=20
CORS_ORIGIN=*
VITE_API_BASE_URL=http://localhost:8080
```

Descrição:

- `IMPORT_WORKERS`: quantidade de workers usados no processamento paralelo.
- `CORS_ORIGIN`: origem liberada para chamadas ao backend.
- `VITE_API_BASE_URL`: URL da API usada pelo frontend.

## Endpoints

### Criar Importação

```http
POST /api/importar
```

Recebe um ou mais arquivos CSV via `multipart/form-data`.

Exemplo:

```bash
curl -X POST http://localhost:8080/api/importar \
  -F "files=@massa-teste/tabela_frete_teste.csv"
```

Resposta:

```json
{
  "id": "1781627076051443654"
}
```

Status HTTP:

- `202 Accepted`: importação criada e processamento iniciado.
- `400 Bad Request`: upload inválido ou nenhum arquivo enviado.
- `500 Internal Server Error`: erro inesperado.

### Listar Importações

```http
GET /api/importacoes
```

Exemplo:

```bash
curl http://localhost:8080/api/importacoes
```

Resposta:

```json
{
  "data": [
    {
      "id": "1781627076051443654",
      "status": "CONCLUIDA",
      "total_linhas": 5100,
      "linhas_processadas": 5100,
      "validas": 1735,
      "invalidas": 3365,
      "progresso": 100,
      "criada_em": "2026-06-16T16:24:36.051447856Z",
      "atualizada_em": "2026-06-16T16:24:38.721426525Z"
    }
  ]
}
```

### Listar Erros da Importação

```http
GET /api/importacoes/{id}/erros
```

Exemplo:

```bash
curl http://localhost:8080/api/importacoes/1781627076051443654/erros
```

Resposta:

```json
{
  "data": [
    {
      "numero_linha": 3,
      "dados_originais": ["BELO HORIZONTE", "", "30", "50", "185.91"],
      "motivo": "destino e obrigatório"
    }
  ]
}
```

O endpoint também aceita paginação opcional via query string:

```bash
curl "http://localhost:8080/api/importacoes/1781627076051443654/erros?page=1&limit=50"
```

Resposta paginada:

```json
{
  "data": [
    {
      "numero_linha": 3,
      "dados_originais": ["BELO HORIZONTE", "", "30", "50", "185.91"],
      "motivo": "destino e obrigatório"
    }
  ],
  "page": 1,
  "limit": 50,
  "total": 3365,
  "total_pages": 68
}
```

Sem `page` e `limit`, o endpoint mantém compatibilidade e retorna todos os erros no campo `data`.

### Exportar Linhas Válidas

```http
GET /api/importacoes/{id}/validas
```

Retorna as linhas válidas da importação em JSON. No frontend, o botão `Exportar válidas` baixa esse conteúdo como arquivo `.json`.

Exemplo:

```bash
curl http://localhost:8080/api/importacoes/1781627076051443654/validas
```

Resposta:

```json
{
  "data": [
    {
      "numero_linha": 2,
      "origem": "SAO PAULO",
      "destino": "RIO DE JANEIRO",
      "peso_min": 0,
      "peso_max": 10,
      "valor": 45.9
    }
  ],
  "total": 1735
}
```

Status HTTP:

- `200 OK`: recurso encontrado.
- `404 Not Found`: importação inexistente.

## Formato Esperado do CSV

```csv
origem,destino,peso_min,peso_max,valor
SAO PAULO,RIO DE JANEIRO,0,10,45.90
CURITIBA,FLORIANOPOLIS,0,5,32.00
```

Campos:

- `origem`: cidade de origem.
- `destino`: cidade de destino.
- `peso_min`: peso mínimo da faixa.
- `peso_max`: peso máximo da faixa.
- `valor`: valor do frete.

## Regras de Validação

- `origem` e obrigatória.
- `destino` e obrigatório.
- `peso_min` deve ser numérico.
- `peso_max` deve ser numérico.
- `valor` deve ser numérico.
- Pesos não podem ser negativos.
- `peso_max` deve ser maior que `peso_min`.
- `valor` deve ser maior que zero.
- Não pode haver duplicidade de `origem + destino + peso_min + peso_max`.

## Processamento Concorrente

O processamento acontece em background após o upload.

Fluxo simplificado:

```text
POST /api/importar
  -> cria importação em memória
  -> inicia goroutine de processamento
  -> lê CSV
  -> marca duplicidades
  -> envia linhas para worker pool
  -> atualiza contadores e erros no repository
```

Cada linha executa obrigatoriamente:

```go
time.Sleep(10 * time.Millisecond)
```

Esse atraso simula uma chamada externa, como validação de localidade.

Com `IMPORT_WORKERS=20`, a massa de teste foi processada em aproximadamente 3 segundos no ambiente local usado durante o desenvolvimento.

## Armazenamento em Memória

Os dados ficam somente na memória do processo Go.

São armazenados:

- importações criadas
- status da importação
- total de linhas
- linhas processadas
- quantidade de linhas válidas
- quantidade de linhas inválidas
- erros por importação
- linhas válidas por importação, usadas na exportação JSON

Como não há banco de dados, os dados são perdidos ao reiniciar o backend ou o container.

Isso atende à restrição do desafio: sem banco externo e sem ORM.

## Massa de Teste

Para gerar a massa:

```bash
cd massa-teste
python3 gerar-massa-teste.py
```

O script gera `tabela_frete_teste.csv` com linhas válidas e erros propositais.

Tipos de erro incluídos:

- origem ou destino vazio
- `peso_max` menor que `peso_min`
- valor zero ou negativo
- peso negativo
- duplicidade de origem, destino e faixa de peso

Observação: o script gera 5000 linhas base e adiciona cerca de 100 duplicatas. Por isso, o arquivo final pode ter aproximadamente 5100 registros, além do cabeçalho.

## Testes

Rodar testes do backend:

```bash
go test ./...
```

Caso o Go não esteja instalado localmente, é possível rodar via Docker:

```bash
docker run --rm -v "$PWD":/app -w /app golang:1.22-alpine go test ./...
```

## Decisões Técnicas

- Foi usada a biblioteca padrão `net/http` para manter a API simples e idiomática.
- O worker pool evita criar uma goroutine por linha e permite controlar concorrência por `IMPORT_WORKERS`.
- O repository em memória usa `sync.RWMutex` para proteger leituras e escritas concorrentes.
- A deduplicação é feita antes do processamento paralelo para manter comportamento determinístico.
- O frontend usa polling a cada 1 segundo para acompanhar progresso sem complexidade extra.
- A paginação dos erros foi adicionada de forma compatível: sem query string, o endpoint continua retornando todos os erros.
- As linhas válidas são armazenadas em memória para permitir exportação JSON sem reprocessar o CSV.

## Limitações Conhecidas

- Os dados não persistem após restart do backend.
- O frontend usa polling, não WebSocket ou SSE.
- O CSV deve estar em UTF-8.
- O highlight visual por célula inválida não foi implementado, pois exigiria retornar metadados de campo inválido por linha.

## Melhorias Futuras

- Adicionar SSE para progresso em tempo real.
- Adicionar endpoint de cancelamento de importação.
- Highlight visual por célula inválida no frontend.
- Adicionar benchmark automatizado para a massa de 5000 linhas.
