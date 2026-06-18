# Importador de Tabelas de Frete em Lote

Aplicacao web fullstack para upload e validacao em lote de tabelas de frete em CSV.

O backend processa as linhas em background usando goroutines e um worker pool configuravel. O frontend permite enviar um ou mais arquivos, acompanha o progresso da importacao por polling e exibe os erros encontrados por linha.

## Stack

- Backend: Go 1.22 com `net/http`
- Frontend: Vue 3 com Composition API e Vite
- Infra: Docker Compose
- Armazenamento: memoria da aplicacao
- Banco de dados: nao utilizado
- ORM: nao utilizado

## Requisitos Atendidos

- Upload de um ou mais arquivos CSV
- Processamento concorrente com goroutines
- Worker pool configuravel por variavel de ambiente
- Validacao com `time.Sleep(10 * time.Millisecond)` por linha
- Listagem de importacoes com status, contadores e percentual de progresso
- Listagem de linhas invalidas com numero da linha, dados originais e motivo
- Paginacao opcional das linhas invalidas
- Exportacao das linhas validas em JSON
- Armazenamento em memoria protegido por mutex
- Frontend com upload, barra de progresso, resumo, tabela de erros paginada e botao de exportacao
- Docker Compose com servicos de backend e frontend
- Massa de teste com erros propositais
- Testes unitarios para regras de validacao

## Arquitetura

O backend segue uma organizacao simples em camadas:

```text
Request
  -> Handler
  -> DTO
  -> Service
  -> Repository em memoria
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
- `internal/config`: leitura de variaveis de ambiente.
- `internal/handler`: rotas HTTP, multipart upload e respostas JSON.
- `internal/dto`: contratos de resposta da API.
- `internal/service`: regras de negocio, leitura do CSV, validacoes e worker pool.
- `internal/repository`: armazenamento em memoria com `sync.RWMutex`.
- `internal/model`: entidades de dominio e status da importacao.
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

## Variaveis de Ambiente

Exemplo em `.env.example`:

```env
IMPORT_WORKERS=20
CORS_ORIGIN=*
VITE_API_BASE_URL=http://localhost:8080
```

Descricao:

- `IMPORT_WORKERS`: quantidade de workers usados no processamento paralelo.
- `CORS_ORIGIN`: origem liberada para chamadas ao backend.
- `VITE_API_BASE_URL`: URL da API usada pelo frontend.

## Endpoints

### Criar Importacao

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

- `202 Accepted`: importacao criada e processamento iniciado.
- `400 Bad Request`: upload invalido ou nenhum arquivo enviado.
- `500 Internal Server Error`: erro inesperado.

### Listar Importacoes

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

### Listar Erros da Importacao

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
      "motivo": "destino e obrigatorio"
    }
  ]
}
```

O endpoint tambem aceita paginacao opcional via query string:

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
      "motivo": "destino e obrigatorio"
    }
  ],
  "page": 1,
  "limit": 50,
  "total": 3365,
  "total_pages": 68
}
```

Sem `page` e `limit`, o endpoint mantem compatibilidade e retorna todos os erros no campo `data`.

### Exportar Linhas Validas

```http
GET /api/importacoes/{id}/validas
```

Retorna as linhas validas da importacao em JSON. No frontend, o botao `Exportar validas` baixa esse conteudo como arquivo `.json`.

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
- `404 Not Found`: importacao inexistente.

## Formato Esperado do CSV

```csv
origem,destino,peso_min,peso_max,valor
SAO PAULO,RIO DE JANEIRO,0,10,45.90
CURITIBA,FLORIANOPOLIS,0,5,32.00
```

Campos:

- `origem`: cidade de origem.
- `destino`: cidade de destino.
- `peso_min`: peso minimo da faixa.
- `peso_max`: peso maximo da faixa.
- `valor`: valor do frete.

## Regras de Validacao

- `origem` e obrigatoria.
- `destino` e obrigatorio.
- `peso_min` deve ser numerico.
- `peso_max` deve ser numerico.
- `valor` deve ser numerico.
- Pesos nao podem ser negativos.
- `peso_max` deve ser maior que `peso_min`.
- `valor` deve ser maior que zero.
- Nao pode haver duplicidade de `origem + destino + peso_min + peso_max`.

## Processamento Concorrente

O processamento acontece em background apos o upload.

Fluxo simplificado:

```text
POST /api/importar
  -> cria importacao em memoria
  -> inicia goroutine de processamento
  -> le CSV
  -> marca duplicidades
  -> envia linhas para worker pool
  -> atualiza contadores e erros no repository
```

Cada linha executa obrigatoriamente:

```go
time.Sleep(10 * time.Millisecond)
```

Esse atraso simula uma chamada externa, como validacao de localidade.

Com `IMPORT_WORKERS=20`, a massa de teste foi processada em aproximadamente 3 segundos no ambiente local usado durante o desenvolvimento.

## Armazenamento em Memoria

Os dados ficam somente na memoria do processo Go.

Sao armazenados:

- importacoes criadas
- status da importacao
- total de linhas
- linhas processadas
- quantidade de linhas validas
- quantidade de linhas invalidas
- erros por importacao
- linhas validas por importacao, usadas na exportacao JSON

Como nao ha banco de dados, os dados sao perdidos ao reiniciar o backend ou o container.

Isso atende a restricao do desafio: sem banco externo e sem ORM.

## Massa de Teste

Para gerar a massa:

```bash
cd massa-teste
python3 gerar-massa-teste.py
```

O script gera `tabela_frete_teste.csv` com linhas validas e erros propositais.

Tipos de erro incluidos:

- origem ou destino vazio
- `peso_max` menor que `peso_min`
- valor zero ou negativo
- peso negativo
- duplicidade de origem, destino e faixa de peso

Observacao: o script gera 5000 linhas base e adiciona cerca de 100 duplicatas. Por isso, o arquivo final pode ter aproximadamente 5100 registros, alem do cabecalho.

## Testes

Rodar testes do backend:

```bash
go test ./...
```

Caso o Go nao esteja instalado localmente, e possivel rodar via Docker:

```bash
docker run --rm -v "$PWD":/app -w /app golang:1.22-alpine go test ./...
```

## Decisoes Tecnicas

- Foi usada a biblioteca padrao `net/http` para manter a API simples e idiomatica.
- O worker pool evita criar uma goroutine por linha e permite controlar concorrencia por `IMPORT_WORKERS`.
- O repository em memoria usa `sync.RWMutex` para proteger leituras e escritas concorrentes.
- A deduplicacao e feita antes do processamento paralelo para manter comportamento deterministico.
- O frontend usa polling a cada 1 segundo para acompanhar progresso sem complexidade extra.
- A paginacao dos erros foi adicionada de forma compativel: sem query string, o endpoint continua retornando todos os erros.
- As linhas validas sao armazenadas em memoria para permitir exportacao JSON sem reprocessar o CSV.

## Limitacoes Conhecidas

- Os dados nao persistem apos restart do backend.
- O frontend usa polling, nao WebSocket ou SSE.
- O CSV deve estar em UTF-8.
- O highlight visual por celula invalida nao foi implementado, pois exigiria retornar metadados de campo invalido por linha.

## Melhorias Futuras

- Adicionar SSE para progresso em tempo real.
- Adicionar endpoint de cancelamento de importacao.
- Highlight visual por celula invalida no frontend.
- Adicionar benchmark automatizado para a massa de 5000 linhas.
