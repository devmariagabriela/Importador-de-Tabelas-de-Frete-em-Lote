<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'

type ImportStatus = 'CONCLUIDA' | 'PROCESSANDO' | 'PENDENTE' | 'FALHOU' | string
type FreightField = 'origem' | 'destino' | 'peso_min' | 'peso_max' | 'valor'

interface Importacao {
  id: string
  status: ImportStatus
  total_linhas: number
  linhas_processadas: number
  validas: number
  invalidas: number
  progresso: number
  duracao_ms: number
}

interface LinhaErro {
  numero_linha: number
  dados_originais: string[]
  motivo: string
  campo?: FreightField | 'peso' | 'linha' | string
}

interface LinhaValida {
  numero_linha: number
  origem: string
  destino: string
  peso_min: number
  peso_max: number
  valor: number
}

interface ImportacoesResponse {
  data?: Importacao[]
}

interface ErrosResponse {
  data?: LinhaErro[]
  page?: number
  limit?: number
  total?: number
  total_pages?: number
}

interface LinhasValidasResponse {
  data?: LinhaValida[]
}

interface ImportCreatedResponse {
  id?: string
  error?: string
}

interface ErrorPagination {
  page: number
  limit: number
  total: number
  total_pages: number
}

interface Totals {
  total: number
  validas: number
  invalidas: number
}

interface ApplyImportsOptions {
  refreshDetails?: boolean
}

const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8086'

const files = ref<File[]>([])
const imports = ref<Importacao[]>([])
const errors = ref<LinhaErro[]>([])
const validRows = ref<LinhaValida[]>([])
const selectedImportId = ref('')
const loading = ref(false)
const refreshing = ref(false)
const uploading = ref(false)
const message = ref('')
const socketConnected = ref(false)
const errorsPage = ref(1)
const errorsLimit = 50
const validRowsPage = ref(1)
const validRowsLimit = 50
const errorPagination = ref<ErrorPagination>({
  page: 1,
  limit: errorsLimit,
  total: 0,
  total_pages: 0,
})
const freightFields: FreightField[] = ['origem', 'destino', 'peso_min', 'peso_max', 'valor']
let importSocket: WebSocket | null = null
let refreshingDetails = false

const selectedImport = computed(() => imports.value.find((item) => item.id === selectedImportId.value))
const activeImports = computed(() => imports.value.filter((item) => item.status === 'PROCESSANDO').length)
const overallProgress = computed(() => {
  if (!totals.value.total) return 0
  return ((totals.value.validas + totals.value.invalidas) * 100) / totals.value.total
})
const validRowsTotalPages = computed(() => Math.ceil(validRows.value.length / validRowsLimit))
const paginatedValidRows = computed(() => {
  const start = (validRowsPage.value - 1) * validRowsLimit
  return validRows.value.slice(start, start + validRowsLimit)
})
const totals = computed(() => {
  return imports.value.reduce(
    (acc: Totals, item) => {
      acc.total += item.total_linhas
      acc.validas += item.validas
      acc.invalidas += item.invalidas
      return acc
    },
    { total: 0, validas: 0, invalidas: 0 },
  )
})

function onFilesChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  files.value = Array.from(input?.files || [])
  message.value = ''
}

async function upload() {
  if (!files.value.length) {
    message.value = 'Selecione ao menos um arquivo CSV.'
    return
  }

  const form = new FormData()
  files.value.forEach((file) => form.append('files', file))

  uploading.value = true
  message.value = ''
  try {
    const response = await fetch(`${apiBase}/api/importar`, {
      method: 'POST',
      body: form,
    })
    const payload = (await response.json()) as ImportCreatedResponse
    if (!response.ok) {
      throw new Error(payload.error || 'Falha no upload.')
    }
    selectedImportId.value = payload.id || ''
    files.value = []
    await refreshImports()
  } catch (error) {
    message.value = errorMessage(error)
  } finally {
    uploading.value = false
  }
}

async function refreshImports() {
  if (refreshing.value) return

  refreshing.value = true
  loading.value = true
  try {
    const response = await fetch(`${apiBase}/api/importacoes`)
    if (!response.ok) {
      throw new Error('Falha ao carregar importações.')
    }

    const payload = (await response.json()) as ImportacoesResponse
    await applyImports(payload.data || [], { refreshDetails: true })
  } catch (error) {
    message.value = 'Não foi possível carregar as importações.'
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function applyImports(nextImports: Importacao[], options: ApplyImportsOptions = {}) {
  const previousSelected = selectedImport.value
  imports.value = nextImports

  if (selectedImportId.value && !imports.value.some((item) => item.id === selectedImportId.value)) {
    selectedImportId.value = ''
    errors.value = []
    validRows.value = []
    resetErrorPagination()
    resetValidRowsPagination()
  }
  if (!selectedImportId.value && imports.value.length) {
    selectedImportId.value = imports.value[0].id
  }

  const currentSelected = selectedImport.value
  const statusChanged = previousSelected?.status !== currentSelected?.status
  const countersChanged =
    previousSelected?.linhas_processadas !== currentSelected?.linhas_processadas ||
    previousSelected?.validas !== currentSelected?.validas ||
    previousSelected?.invalidas !== currentSelected?.invalidas
  const finishedNow =
    statusChanged && (currentSelected?.status === 'CONCLUIDA' || currentSelected?.status === 'FALHOU')

  if (options.refreshDetails || countersChanged || finishedNow) {
    await refreshSelectedDetails()
  }
}

async function refreshSelectedDetails() {
  if (!selectedImportId.value || refreshingDetails) return

  refreshingDetails = true
  try {
    await refreshErrors(selectedImportId.value)
    await refreshValidRows(selectedImportId.value)
  } catch (error) {
    message.value = errorMessage(error)
  } finally {
    refreshingDetails = false
  }
}

function connectImportSocket() {
  if (importSocket) {
    importSocket.close()
  }

  importSocket = new WebSocket(`${webSocketBase()}/api/importacoes/ws`)
  importSocket.onopen = () => {
    socketConnected.value = true
  }
  importSocket.onmessage = async (event: MessageEvent<string>) => {
    try {
      const payload = JSON.parse(event.data) as ImportacoesResponse
      await applyImports(payload.data || [])
      loading.value = false
    } catch (error) {
      message.value = 'Não foi possível processar atualizações em tempo real.'
    }
  }
  importSocket.onerror = () => {
    socketConnected.value = false
    message.value = 'Conexão em tempo real indisponível. Use Atualizar para recarregar.'
    loading.value = false
  }
  importSocket.onclose = () => {
    socketConnected.value = false
  }
}

function webSocketBase(): string {
  const url = new URL(apiBase)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.origin
}

async function refreshErrors(id: string) {
  if (!id) {
    errors.value = []
    resetErrorPagination()
    return
  }
  const response = await fetch(
    `${apiBase}/api/importacoes/${id}/erros?page=${errorsPage.value}&limit=${errorsLimit}`,
  )
  if (!response.ok) {
    errors.value = []
    resetErrorPagination()
    throw new Error('Falha ao carregar erros da importação.')
  }

  const payload = (await response.json()) as ErrosResponse
  errors.value = payload.data || []
  errorPagination.value = {
    page: payload.page || errorsPage.value,
    limit: payload.limit || errorsLimit,
    total: payload.total || errors.value.length,
    total_pages: payload.total_pages || 0,
  }
}

async function refreshValidRows(id: string) {
  if (!id) {
    validRows.value = []
    resetValidRowsPagination()
    return
  }

  const response = await fetch(`${apiBase}/api/importacoes/${id}/validas`)
  if (!response.ok) {
    validRows.value = []
    resetValidRowsPagination()
    throw new Error('Falha ao carregar linhas válidas da importação.')
  }

  const payload = (await response.json()) as LinhasValidasResponse
  validRows.value = payload.data || []
  if (validRowsPage.value > validRowsTotalPages.value) {
    validRowsPage.value = Math.max(validRowsTotalPages.value, 1)
  }
}

async function selectImport(id: string) {
  selectedImportId.value = id
  errorsPage.value = 1
  validRowsPage.value = 1
  try {
    await refreshErrors(id)
    await refreshValidRows(id)
  } catch (error) {
    message.value = errorMessage(error)
  }
}

async function changeErrorsPage(direction: number) {
  if (!selectedImport.value) return

  const nextPage = errorsPage.value + direction
  if (nextPage < 1 || (errorPagination.value.total_pages && nextPage > errorPagination.value.total_pages)) {
    return
  }

  errorsPage.value = nextPage
  try {
    await refreshErrors(selectedImportId.value)
  } catch (error) {
    message.value = errorMessage(error)
  }
}

function changeValidRowsPage(direction: number) {
  if (!selectedImport.value) return

  const nextPage = validRowsPage.value + direction
  if (nextPage < 1 || (validRowsTotalPages.value && nextPage > validRowsTotalPages.value)) {
    return
  }

  validRowsPage.value = nextPage
}

async function exportValidRows() {
  if (!selectedImportId.value) {
    message.value = 'Selecione uma importação para exportar.'
    return
  }

  try {
    const response = await fetch(`${apiBase}/api/importacoes/${selectedImportId.value}/validas`)
    if (!response.ok) {
      throw new Error('Falha ao exportar linhas válidas.')
    }

    const payload = (await response.json()) as LinhasValidasResponse
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `linhas-validas-${selectedImportId.value}.json`
    link.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    message.value = errorMessage(error)
  }
}

function resetValidRowsPagination() {
  validRowsPage.value = 1
}

function resetErrorPagination() {
  errorsPage.value = 1
  errorPagination.value = {
    page: 1,
    limit: errorsLimit,
    total: 0,
    total_pages: 0,
  }
}

function statusClass(status: ImportStatus): string {
  return {
    CONCLUIDA: 'ok',
    PROCESSANDO: 'running',
    PENDENTE: 'pending',
    FALHOU: 'failed',
  }[status] || 'pending'
}

function formatPercent(value: number): string {
  return `${Number(value || 0).toFixed(1)}%`
}

function formatDuration(value: number): string {
  const duration = Number(value || 0)
  if (!duration) return '-'
  if (duration < 1000) return `${duration} ms`
  return `${(duration / 1000).toFixed(1)} s`
}

function shortId(value: string): string {
  if (!value) return '-'
  return value.length > 10 ? value.slice(-10) : value
}

function errorValue(error: LinhaErro, index: number): string {
  return error.dados_originais?.[index] ?? ''
}

function errorCellClass(error: LinhaErro, field: FreightField): { 'invalid-cell': boolean } {
  return {
    'invalid-cell': error.campo === field || (error.campo === 'peso' && field.startsWith('peso_')),
  }
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Erro inesperado.'
}

onMounted(() => {
  refreshImports()
  connectImportSocket()
})

onUnmounted(() => {
  if (importSocket) {
    importSocket.close()
  }
})
</script>

<template>
  <main class="shell">
    <header class="hero">
      <div>
        <p class="eyebrow">Validação em lote</p>
        <h1>Importador de Tabelas de <span>Frete</span></h1>
        <p class="hero-text">Processamento concorrente, progresso em tempo real e rastreabilidade dos erros por linha.</p>
      </div>
      <div class="hero-actions">
        <span class="live-status" :class="{ online: socketConnected }">
          {{ socketConnected ? 'Tempo real ativo' : 'Atualização manual' }}
        </span>
        <button class="ghost" type="button" :disabled="loading" @click="refreshImports">
          Atualizar
        </button>
      </div>
    </header>

    <section class="summary-grid">
      <div class="summary total">
        <span>Total de linhas</span>
        <strong>{{ totals.total }}</strong>
        <small>{{ imports.length }} importação(ões)</small>
      </div>
      <div class="summary success">
        <span>Válidas</span>
        <strong>{{ totals.validas }}</strong>
        <small>Registros aprovados</small>
      </div>
      <div class="summary danger">
        <span>Inválidas</span>
        <strong>{{ totals.invalidas }}</strong>
        <small>{{ activeImports }} em processamento</small>
      </div>
    </section>

    <section class="upload-row">
      <label class="dropzone">
        <input
          type="file"
          accept=".csv,.xls,.xlsx,text/csv,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          multiple
          @change="onFilesChange"
        />
        <span class="upload-icon">^</span>
        <span class="upload-title">{{ files.length ? `${files.length} arquivo(s) selecionado(s)` : 'Selecionar CSV' }}</span>
        <small>CSV principal; XLS e XLSX opcionais</small>
        <em>Arraste ou clique</em>
      </label>
      <button class="import-button" type="button" :disabled="uploading || !files.length" @click="upload">
        {{ uploading ? 'Enviando...' : 'Importar' }}
      </button>
      <p v-if="message" class="message">{{ message }}</p>
    </section>

    <section class="workspace">
      <section class="panel imports-panel">
        <div class="panel-head">
          <div>
            <h2>Importações</h2>
            <small>Histórico e progresso</small>
          </div>
          <span>{{ imports.length }}</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Progresso</th>
                <th>Linhas</th>
                <th>Válidas</th>
                <th>Inválidas</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in imports"
                :key="item.id"
                :class="{ selected: item.id === selectedImportId }"
                @click="selectImport(item.id)"
              >
                <td class="mono">#{{ shortId(item.id) }}</td>
                <td><span class="badge" :class="statusClass(item.status)">{{ item.status }}</span></td>
                <td>
                  <div class="progress">
                    <span :style="{ width: `${Math.min(item.progresso, 100)}%` }"></span>
                  </div>
                  <small>{{ formatPercent(item.progresso) }}</small>
                </td>
                <td>{{ item.linhas_processadas }}/{{ item.total_linhas }}</td>
                <td>{{ item.validas }}</td>
                <td>{{ item.invalidas }}</td>
              </tr>
              <tr v-if="!imports.length">
                <td colspan="6" class="empty">
                  <span class="empty-icon">[]</span>
                  <strong>Nenhuma importação criada</strong>
                  <span>Envie um CSV para iniciar a validação.</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <div>
            <h2>Linhas válidas</h2>
            <small>Registros prontos para exportação</small>
          </div>
          <div class="panel-actions">
            <button class="ghost small" type="button" :disabled="!selectedImport" @click="exportValidRows">
              Exportar
            </button>
            <span>{{ validRows.length }}</span>
          </div>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Linha</th>
                <th>Origem</th>
                <th>Destino</th>
                <th>Peso mín.</th>
                <th>Peso máx.</th>
                <th>Valor</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in paginatedValidRows" :key="row.numero_linha">
                <td>{{ row.numero_linha }}</td>
                <td>{{ row.origem }}</td>
                <td>{{ row.destino }}</td>
                <td>{{ row.peso_min }}</td>
                <td>{{ row.peso_max }}</td>
                <td>{{ row.valor }}</td>
              </tr>
              <tr v-if="!selectedImport">
                <td colspan="6" class="empty">
                  <span class="empty-icon">~</span>
                  <strong>Nenhuma importação selecionada</strong>
                  <span>Selecione um item no histórico.</span>
                </td>
              </tr>
              <tr v-else-if="!validRows.length">
                <td colspan="6" class="empty">
                  <strong>Sem linhas válidas para exibir</strong>
                  <span>A validação ainda não retornou registros aprovados.</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="pager" v-if="selectedImport && validRowsTotalPages > 1">
          <button class="ghost small" type="button" :disabled="validRowsPage <= 1" @click="changeValidRowsPage(-1)">
            Anterior
          </button>
          <span>Página {{ validRowsPage }} de {{ validRowsTotalPages }}</span>
          <button
            class="ghost small"
            type="button"
            :disabled="validRowsPage >= validRowsTotalPages"
            @click="changeValidRowsPage(1)"
          >
            Próxima
          </button>
        </div>
      </section>

      <section class="panel errors-panel">
        <div class="panel-head">
          <div>
            <h2>Erros</h2>
            <small>Linhas rejeitadas e motivo</small>
          </div>
          <span>{{ selectedImport ? selectedImport.invalidas : 0 }}</span>
        </div>
        <div class="selected-line" v-if="selectedImport">
          <span class="badge" :class="statusClass(selectedImport.status)">{{ selectedImport.status }}</span>
          <span>{{ selectedImport.validas }} válidas</span>
          <span>{{ selectedImport.invalidas }} inválidas</span>
          <span>{{ selectedImport.total_linhas }} total</span>
          <span>{{ formatDuration(selectedImport.duracao_ms) }}</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Linha</th>
                <th>Origem</th>
                <th>Destino</th>
                <th>Peso mín.</th>
                <th>Peso máx.</th>
                <th>Valor</th>
                <th>Motivo</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="error in errors" :key="`${error.numero_linha}-${error.motivo}`">
                <td>{{ error.numero_linha }}</td>
                <td
                  v-for="(field, index) in freightFields"
                  :key="field"
                  class="mono"
                  :class="errorCellClass(error, field)"
                >
                  {{ errorValue(error, index) }}
                </td>
                <td>{{ error.motivo }}</td>
              </tr>
              <tr v-if="!selectedImport">
                <td colspan="7" class="empty">
                  <span class="empty-icon">!</span>
                  <strong>Nenhuma importação selecionada</strong>
                  <span>Selecione um item no histórico.</span>
                </td>
              </tr>
              <tr v-else-if="!errors.length">
                <td colspan="7" class="empty">
                  <strong>Sem erros para exibir</strong>
                  <span>A importação selecionada não possui linhas inválidas nesta página.</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="pager" v-if="selectedImport && errorPagination.total_pages > 1">
          <button class="ghost small" type="button" :disabled="errorsPage <= 1" @click="changeErrorsPage(-1)">
            Anterior
          </button>
          <span>Página {{ errorPagination.page }} de {{ errorPagination.total_pages }}</span>
          <button
            class="ghost small"
            type="button"
            :disabled="errorsPage >= errorPagination.total_pages"
            @click="changeErrorsPage(1)"
          >
            Próxima
          </button>
        </div>
      </section>
    </section>
  </main>
</template>
