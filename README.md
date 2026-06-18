# Importador de Tabelas de Frete em Lote

Aplicação web fullstack para upload e validação em lote de tabelas de frete em CSV.

O backend processa as linhas em background usando goroutines e um worker pool configurável. O frontend permite enviar um ou mais arquivos, acompanha o progresso da importação por WebSocket e exibe os erros encontrados por linha.

## Stack

- Backend: Go 1.22 com `net/http`
- Frontend: Vue 3 com Composition API e Vite
- Infra: Docker Compose
- Armazenamento: memória da aplicação
- Banco de dados: não utilizado
- ORM: não utilizado

## Requisitos Atendidos

- Upload de um ou mais arquivos CSV
- Suporte adicional a XLS e XLSX como diferencial
- Processamento concorrente com goroutines
- Worker pool configurável por variável de ambiente
- Validação com `time.Sleep(10 * time.Millisecond)` por linha
- Listagem de importações com status, contadores e percentual de progresso
- Exibição da duração de processamento por importação
- Atualização de progresso em tempo real com WebSocket
- Listagem de linhas inválidas com número da linha, dados originais e motivo
- Highlight visual da célula inválida no frontend
- Paginação opcional das linhas inválidas
- Exportação das linhas válidas em JSON
- Armazenamento em memória protegido por mutex
- Frontend com upload, barra de progresso, resumo, tabela de válidas, tabela de erros paginada e botão de exportação
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
- `internal/service`: regras de negócio, leitura de CSV, leitura adicional de Excel, validações e worker pool.
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

Como diferencial, o mesmo endpoint também aceita arquivos XLS e XLSX.

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
      "duracao_ms": 2840,
      "criada_em": "2026-06-16T16:24:36.051447856Z",
      "atualizada_em": "2026-06-16T16:24:38.721426525Z"
    }
  ]
}
```

### WebSocket de Importações

```http
GET /api/importacoes/ws
```

Abre uma conexão WebSocket que envia uma mensagem JSON a cada segundo com o mesmo payload de `GET /api/importacoes`.

No frontend, o progresso é atualizado automaticamente por WebSocket. O botão `Atualizar` permanece disponível apenas como ação manual de contingência, caso a conexão em tempo real falhe ou o avaliador queira forçar uma nova consulta ao backend.

Exemplo de mensagem:

```json
{
  "data": [
    {
      "id": "1781627076051443654",
      "status": "PROCESSANDO",
      "total_linhas": 5100,
      "linhas_processadas": 1200,
      "validas": 900,
      "invalidas": 300,
      "progresso": 23.52,
      "duracao_ms": 640,
      "criada_em": "2026-06-16T16:24:36.051447856Z",
      "atualizada_em": "2026-06-16T16:24:36.691447856Z"
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
      "motivo": "Destino é obrigatório",
      "campo": "destino"
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
      "motivo": "Destino é obrigatório",
      "campo": "destino"
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

Retorna as linhas válidas da importação em JSON. No frontend, o botão `Exportar` na tabela de linhas válidas baixa esse conteúdo como arquivo `.json`.

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

## Formatos de Arquivo

O formato CSV é o formato principal da aplicação e atende integralmente aos requisitos propostos no desafio.

Como funcionalidade adicional, também foi implementado suporte para arquivos XLS e XLSX, permitindo maior flexibilidade no processo de importação.

Todos os formatos são convertidos internamente para uma estrutura única de processamento, utilizando o mesmo pipeline de validação concorrente baseado em goroutines.

### Formatos suportados

- CSV: formato principal e recomendado
- XLS: diferencial
- XLSX: diferencial

## Formato Esperado do CSV

O CSV é o formato principal esperado pelo desafio:

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

Para XLS e XLSX, o layout deve ser o mesmo. A primeira aba é lida e a primeira linha deve conter o cabeçalho.

## Regras de Validação

- `origem` é obrigatória.
- `destino` é obrigatório.
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
  -> se o arquivo for XLS/XLSX, converte a primeira aba para o mesmo modelo de linhas
  -> marca duplicidades
  -> divide linhas em chunks
  -> envia chunks para worker pool
  -> atualiza contadores e erros no repository
```

Cada linha executa obrigatoriamente:

```go
time.Sleep(10 * time.Millisecond)
```

Esse atraso simula uma chamada externa, como validação de localidade.

Com `IMPORT_WORKERS=20`, a massa de teste de aproximadamente 5000 linhas deve concluir abaixo do limite de 10 segundos exigido no desafio.

Resultado medido localmente via Docker Compose:

```text
Arquivo: massa-teste/tabela_frete_teste.csv
Linhas: 5100
Workers: 20
Tempo medido entre upload e conclusão: 3,04s
Duração registrada pela API: 2616ms
```

Para medir no seu ambiente:

```bash
docker compose up -d --build
cd massa-teste
python3 gerar-massa-teste.py
```

Depois faça upload do arquivo pelo frontend e acompanhe a coluna `Duração` na tabela de importações.

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
- O fluxo principal lê CSV e divide as linhas em chunks para validação paralela.
- XLS e XLSX são diferenciais: a primeira aba é convertida para o mesmo modelo interno usado pelo CSV.
- O worker pool evita criar uma goroutine por linha e permite controlar concorrência por `IMPORT_WORKERS`.
- O repository em memória usa `sync.RWMutex` para proteger leituras e escritas concorrentes.
- A deduplicação é feita antes do processamento paralelo para manter comportamento determinístico.
- O frontend usa WebSocket para acompanhar progresso com atualização contínua sem abrir uma nova requisição a cada ciclo.
- O botão `Atualizar` no frontend não substitui o WebSocket; ele apenas força uma consulta manual a `GET /api/importacoes` como fallback operacional.
- A paginação dos erros foi adicionada de forma compatível: sem query string, o endpoint continua retornando todos os erros.
- As linhas válidas são armazenadas em memória para permitir exportação JSON sem reprocessar o CSV.
- O backend retorna o campo inválido em cada erro para permitir highlight visual no frontend.
- A duração da importação é calculada em memória entre os status `PROCESSANDO` e `CONCLUIDA`/`FALHOU`.

## Limitações Conhecidas

- Os dados não persistem após restart do backend.
- Arquivos CSV devem estar em UTF-8.

## Melhorias Futuras

- Adicionar endpoint de cancelamento de importação.
- Adicionar benchmark automatizado para a massa de 5000 linhas.
