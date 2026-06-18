<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'

const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

const files = ref([])
const imports = ref([])
const errors = ref([])
const selectedImportId = ref('')
const loading = ref(false)
const refreshing = ref(false)
const uploading = ref(false)
const message = ref('')
const errorsPage = ref(1)
const errorsLimit = 50
const errorPagination = ref({
  page: 1,
  limit: errorsLimit,
  total: 0,
  total_pages: 0,
})
let timer = 0

const selectedImport = computed(() => imports.value.find((item) => item.id === selectedImportId.value))
const totals = computed(() => {
  return imports.value.reduce(
    (acc, item) => {
      acc.total += item.total_linhas
      acc.validas += item.validas
      acc.invalidas += item.invalidas
      return acc
    },
    { total: 0, validas: 0, invalidas: 0 },
  )
})

function onFilesChange(event) {
  files.value = Array.from(event.target.files || [])
  message.value = ''
}

async function upload() {
  if (!files.value.length) {
    message.value = 'Selecione ao menos um CSV.'
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
    const payload = await response.json()
    if (!response.ok) {
      throw new Error(payload.error || 'Falha no upload.')
    }
    selectedImportId.value = payload.id
    files.value = []
    await refreshImports()
  } catch (error) {
    message.value = error.message
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

    const payload = await response.json()
    imports.value = payload.data || []

    if (selectedImportId.value && !imports.value.some((item) => item.id === selectedImportId.value)) {
      selectedImportId.value = ''
      errors.value = []
      resetErrorPagination()
    }
    if (!selectedImportId.value && imports.value.length) {
      selectedImportId.value = imports.value[0].id
    }

    if (selectedImportId.value) {
      await refreshErrors(selectedImportId.value)
    }
  } catch (error) {
    message.value = 'Não foi possível carregar as importações.'
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function refreshErrors(id) {
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

  const payload = await response.json()
  errors.value = payload.data || []
  errorPagination.value = {
    page: payload.page || errorsPage.value,
    limit: payload.limit || errorsLimit,
    total: payload.total || errors.value.length,
    total_pages: payload.total_pages || 0,
  }
}

async function selectImport(id) {
  selectedImportId.value = id
  errorsPage.value = 1
  try {
    await refreshErrors(id)
  } catch (error) {
    message.value = error.message
  }
}

async function changeErrorsPage(direction) {
  if (!selectedImport.value) return

  const nextPage = errorsPage.value + direction
  if (nextPage < 1 || (errorPagination.value.total_pages && nextPage > errorPagination.value.total_pages)) {
    return
  }

  errorsPage.value = nextPage
  try {
    await refreshErrors(selectedImportId.value)
  } catch (error) {
    message.value = error.message
  }
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

    const payload = await response.json()
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `linhas-validas-${selectedImportId.value}.json`
    link.click()
    URL.revokeObjectURL(url)
  } catch (error) {
    message.value = error.message
  }
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

function statusClass(status) {
  return {
    CONCLUIDA: 'ok',
    PROCESSANDO: 'running',
    PENDENTE: 'pending',
    FALHOU: 'failed',
  }[status] || 'pending'
}

function formatPercent(value) {
  return `${Number(value || 0).toFixed(1)}%`
}

onMounted(() => {
  refreshImports()
  timer = window.setInterval(refreshImports, 1000)
})

onUnmounted(() => {
  window.clearInterval(timer)
})
</script>

<template>
  <main class="shell">
    <section class="topbar">
      <div>
        <p class="eyebrow">Validação em lote</p>
        <h1>Importador de Tabelas de Frete</h1>
      </div>
      <button class="ghost" type="button" :disabled="loading" @click="refreshImports">Atualizar</button>
    </section>

    <section class="summary-grid">
      <div class="summary">
        <span>Total</span>
        <strong>{{ totals.total }}</strong>
      </div>
      <div class="summary">
        <span>Válidas</span>
        <strong>{{ totals.validas }}</strong>
      </div>
      <div class="summary">
        <span>Inválidas</span>
        <strong>{{ totals.invalidas }}</strong>
      </div>
    </section>

    <section class="upload-panel">
      <label class="dropzone">
        <input type="file" accept=".csv,text/csv" multiple @change="onFilesChange" />
        <span>{{ files.length ? `${files.length} arquivo(s) selecionado(s)` : 'Selecionar CSV' }}</span>
      </label>
      <button type="button" :disabled="uploading || !files.length" @click="upload">
        {{ uploading ? 'Enviando...' : 'Importar' }}
      </button>
      <p v-if="message" class="message">{{ message }}</p>
    </section>

    <section class="content-grid">
      <div class="panel">
        <div class="panel-head">
          <h2>Importações</h2>
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
                <td class="mono">{{ item.id }}</td>
                <td><span class="badge" :class="statusClass(item.status)">{{ item.status }}</span></td>
                <td>
                  <div class="progress">
                    <span :style="{ width: `${Math.min(item.progresso, 100)}%` }"></span>
                  </div>
                  <small>{{ formatPercent(item.progresso) }}</small>
                </td>
                <td>{{ item.linhas_processadas }} / {{ item.total_linhas }}</td>
                <td>{{ item.validas }}</td>
                <td>{{ item.invalidas }}</td>
              </tr>
              <tr v-if="!imports.length">
                <td colspan="6" class="empty">Nenhuma importação criada.</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="panel">
        <div class="panel-head">
          <h2>Erros</h2>
          <div class="panel-actions">
            <button class="ghost small" type="button" :disabled="!selectedImport" @click="exportValidRows">
              Exportar válidas
            </button>
            <span>{{ selectedImport ? selectedImport.invalidas : 0 }}</span>
          </div>
        </div>
        <div class="selected-line" v-if="selectedImport">
          <strong>{{ selectedImport.status }}</strong>
          <span>{{ selectedImport.validas }} válidas, {{ selectedImport.invalidas }} inválidas, {{ selectedImport.total_linhas }} total</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Linha</th>
                <th>Dados</th>
                <th>Motivo</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="error in errors" :key="`${error.numero_linha}-${error.motivo}`">
                <td>{{ error.numero_linha }}</td>
                <td class="mono">{{ error.dados_originais.join(', ') }}</td>
                <td>{{ error.motivo }}</td>
              </tr>
              <tr v-if="!selectedImport">
                <td colspan="3" class="empty">Selecione uma importação para ver os erros.</td>
              </tr>
              <tr v-else-if="!errors.length">
                <td colspan="3" class="empty">Sem erros para exibir.</td>
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
      </div>
    </section>
  </main>
</template>
