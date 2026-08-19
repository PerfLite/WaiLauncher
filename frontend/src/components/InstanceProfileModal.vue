<script setup>
import {ref, watch, computed} from 'vue'
import {t} from '../i18n'
import {toast} from '../store'
import {
  GetInstalledMods,
  ToggleMod,
  DeleteMod,
  SearchModrinthMods,
  InstallModrinthMod,
  GetInstanceLogs,
  OpenInstanceDir
} from '../../wailsjs/go/main/App'

const props = defineProps({
  inst: {type: Object, default: null},
  open: {type: Boolean, default: false}
})

const emit = defineEmits(['close', 'play'])

const tab = ref('mods') // 'mods' | 'catalog' | 'logs'
const installedMods = ref([])
const loadingMods = ref(false)

const modQuery = ref('')
const catalogQuery = ref('')
const catalogResults = ref([])
const loadingCatalog = ref(false)
const installingMap = ref({})

const logsText = ref('')
const loadingLogs = ref(false)

const isVanilla = computed(() => !props.inst || props.inst.loader === 'vanilla')

const filteredInstalledMods = computed(() => {
  const q = modQuery.value.trim().toLowerCase()
  if (!q) return installedMods.value
  return installedMods.value.filter(m => m.name.toLowerCase().includes(q) || m.filename.toLowerCase().includes(q))
})

watch(() => props.open, (isOpen) => {
  if (isOpen && props.inst) {
    tab.value = 'mods'
    loadMods()
    if (catalogResults.value.length === 0 && !isVanilla.value) {
      searchCatalog()
    }
  }
})

watch(tab, (newTab) => {
  if (newTab === 'mods') {
    loadMods()
  } else if (newTab === 'catalog') {
    if (catalogResults.value.length === 0 && !isVanilla.value) {
      searchCatalog()
    }
  } else if (newTab === 'logs') {
    loadLogs()
  }
})

async function loadMods() {
  if (!props.inst) return
  loadingMods.value = true
  try {
    const list = await GetInstalledMods(props.inst.id)
    installedMods.value = list || []
  } catch (e) {
    console.error('Failed to load mods:', e)
  } finally {
    loadingMods.value = false
  }
}

async function onToggleMod(m) {
  if (!props.inst) return
  const targetState = !m.enabled
  try {
    await ToggleMod(props.inst.id, m.filename, targetState)
    m.enabled = targetState
    if (targetState) {
      m.filename = m.filename.replace(/\.disabled$/, '')
    } else if (!m.filename.endsWith('.disabled')) {
      m.filename += '.disabled'
    }
  } catch (e) {
    toast(t('inst.err') + e, true)
  }
}

async function onDeleteMod(m) {
  if (!props.inst) return
  try {
    await DeleteMod(props.inst.id, m.filename)
    installedMods.value = installedMods.value.filter(item => item.filename !== m.filename)
    toast(t('mods.removed').replace('{m}', m.name))
  } catch (e) {
    toast(t('inst.err') + e, true)
  }
}

let catalogTimer = null
function onCatalogInput() {
  clearTimeout(catalogTimer)
  catalogTimer = setTimeout(() => {
    searchCatalog()
  }, 400)
}

async function searchCatalog() {
  if (!props.inst || isVanilla.value) return
  loadingCatalog.value = true
  try {
    const res = await SearchModrinthMods(
      catalogQuery.value,
      props.inst.loader,
      props.inst.versionId,
      0,
      30
    )
    catalogResults.value = (res && res.hits) ? res.hits : []
  } catch (e) {
    toast(t('inst.err') + e, true)
    catalogResults.value = []
  } finally {
    loadingCatalog.value = false
  }
}

function isModInstalled(hit) {
  const title = (hit.title || '').toLowerCase().replace(/[^a-z0-9]/g, '')
  const slug = (hit.slug || '').toLowerCase().replace(/[^a-z0-9]/g, '')
  return installedMods.value.some(m => {
    const clean = m.name.toLowerCase().replace(/[^a-z0-9]/g, '')
    return (title && clean.includes(title)) || (slug && clean.includes(slug))
  })
}

async function installMod(hit) {
  if (!props.inst) return
  const id = hit.project_id || hit.slug
  installingMap.value[id] = true
  try {
    const mod = await InstallModrinthMod(props.inst.id, id)
    if (mod) {
      installedMods.value.push(mod)
      toast(t('mods.added').replace('{m}', hit.title))
    }
  } catch (e) {
    toast(t('inst.err') + e, true)
  } finally {
    installingMap.value[id] = false
  }
}

async function loadLogs() {
  if (!props.inst) return
  loadingLogs.value = true
  try {
    const s = await GetInstanceLogs(props.inst.id)
    logsText.value = s || ''
  } catch (e) {
    logsText.value = ''
  } finally {
    loadingLogs.value = false
  }
}

function copyLogs() {
  if (!logsText.value) return
  navigator.clipboard.writeText(logsText.value).then(() => {
    toast(t('profile.logsCopied'))
  }).catch(() => {})
}

function openFolder() {
  if (props.inst) {
    OpenInstanceDir(props.inst.id)
  }
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatNumber(num) {
  if (!num) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return String(num)
}
</script>

<template>
  <div class="modal-root" v-if="open && inst">
    <div class="modal-backdrop" @click="emit('close')"></div>
    <div class="modal-box profile-modal">
      <!-- Profile Header -->
      <div class="profile-header">
        <div class="profile-avatar">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
          </svg>
        </div>
        <div class="profile-info">
          <div class="profile-title-row">
            <h2 class="profile-name">{{ inst.name }}</h2>
            <span class="chip gold">{{ t('loader.' + inst.loader) }}{{ inst.loaderVersion ? ' ' + inst.loaderVersion : '' }}</span>
            <span class="chip gray">{{ inst.versionId }}</span>
          </div>
          <div class="profile-meta">
            <span>{{ t('profile.modsTab') }}: <b>{{ installedMods.length }}</b></span>
          </div>
        </div>
        <div class="profile-actions">
          <button class="btn-sec profile-btn-folder" :title="t('profile.folder')" @click="openFolder">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>
            </svg>
            <span>{{ t('profile.folder') }}</span>
          </button>
          <button class="btn-primary profile-btn-play" @click="emit('play', inst)">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
            <span>{{ t('profile.play') }}</span>
          </button>
          <button class="modal-close" @click="emit('close')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
      </div>

      <!-- Navigation Tabs -->
      <div class="profile-tabs">
        <button class="tab-btn" :class="{active: tab === 'mods'}" @click="tab = 'mods'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v20 M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
          {{ t('profile.modsTab') }} ({{ installedMods.length }})
        </button>
        <button class="tab-btn" :class="{active: tab === 'catalog'}" @click="tab = 'catalog'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>
          {{ t('profile.catalogTab') }}
        </button>
        <button class="tab-btn" :class="{active: tab === 'logs'}" @click="tab = 'logs'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>
          {{ t('profile.logsTab') }}
        </button>
      </div>

      <!-- Tab Content Area -->
      <div class="profile-content">
        <!-- TAB 1: INSTALLED MODS -->
        <div v-if="tab === 'mods'" class="tab-pane">
          <div class="tab-toolbar">
            <div class="search-box">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
              <input type="text" v-model="modQuery" :placeholder="t('mods.search')">
            </div>
            <button v-if="!isVanilla" class="btn-primary btn-add-mods" @click="tab = 'catalog'">
              {{ t('profile.addMods') }}
            </button>
          </div>

          <div v-if="isVanilla" class="profile-vanilla-notice">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
            <span>{{ t('profile.vanillaNotice') }}</span>
          </div>

          <div v-else-if="loadingMods" class="profile-loading">
            <span class="bar-mini"><i></i></span>
          </div>

          <div v-else-if="filteredInstalledMods.length === 0" class="profile-empty">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>
            <p>{{ t('profile.emptyMods') }}</p>
            <button class="btn-primary" @click="tab = 'catalog'">{{ t('profile.addMods') }}</button>
          </div>

          <div v-else class="mod-list">
            <div v-for="m in filteredInstalledMods" :key="m.filename" class="mod-item-row" :class="{disabled: !m.enabled}">
              <div class="mod-item-info">
                <div class="mod-item-name">{{ m.name }}</div>
                <div class="mod-item-meta">
                  <span class="mod-item-file">{{ m.filename }}</span>
                  <span class="mod-item-size">{{ formatBytes(m.size) }}</span>
                </div>
              </div>
              <div class="mod-item-actions">
                <label class="switch" :title="m.enabled ? 'Отключить' : 'Включить'">
                  <input type="checkbox" :checked="m.enabled" @change="onToggleMod(m)">
                  <i></i>
                </label>
                <button class="mod-btn-del" :title="t('profile.deleteMod')" @click="onDeleteMod(m)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M3 6h18 M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6 M10 11v6 M14 11v6"/>
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- TAB 2: MODRINTH CATALOG -->
        <div v-if="tab === 'catalog'" class="tab-pane">
          <div class="tab-toolbar">
            <div class="search-box full">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
              <input type="text" v-model="catalogQuery" @input="onCatalogInput" :placeholder="t('profile.searchPh')">
            </div>
          </div>

          <div v-if="isVanilla" class="profile-vanilla-notice">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
            <span>{{ t('profile.vanillaNotice') }}</span>
          </div>

          <div v-else-if="loadingCatalog" class="profile-loading">
            <span class="bar-mini"><i></i></span>
          </div>

          <div v-else-if="catalogResults.length === 0" class="profile-empty">
            <p>{{ t('news.error') }}</p>
          </div>

          <div v-else class="catalog-grid">
            <div v-for="hit in catalogResults" :key="hit.project_id" class="catalog-card">
              <div class="catalog-card-header">
                <img v-if="hit.icon_url" :src="hit.icon_url" class="catalog-icon" alt="">
                <div v-else class="catalog-icon-fallback">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2 2 7l10 5 10-5-10-5z M2 17l10 5 10-5 M2 12l10 5 10-5"/></svg>
                </div>
                <div class="catalog-card-info">
                  <div class="catalog-card-title">{{ hit.title }}</div>
                  <div class="catalog-card-author">{{ t('profile.author') }} {{ hit.author }}</div>
                </div>
              </div>
              <p class="catalog-card-desc">{{ hit.description }}</p>
              <div class="catalog-card-footer">
                <div class="catalog-stats">
                  <span class="stat-item" :title="t('profile.downloads')">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
                    {{ formatNumber(hit.downloads) }}
                  </span>
                </div>
                <button
                  class="btn-install-mod"
                  :class="{installed: isModInstalled(hit), loading: installingMap[hit.project_id || hit.slug]}"
                  :disabled="installingMap[hit.project_id || hit.slug]"
                  @click="installMod(hit)"
                >
                  <span v-if="installingMap[hit.project_id || hit.slug]">{{ t('profile.installing') }}</span>
                  <span v-else-if="isModInstalled(hit)">{{ t('profile.installed') }}</span>
                  <span v-else>+ {{ t('profile.install') }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- TAB 3: LOGS -->
        <div v-if="tab === 'logs'" class="tab-pane logs-pane">
          <div class="tab-toolbar">
            <button class="btn-sec" @click="loadLogs">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/></svg>
              {{ t('profile.refreshLogs') }}
            </button>
            <button class="btn-sec" @click="copyLogs" :disabled="!logsText">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
              {{ t('profile.copyLogs') }}
            </button>
          </div>

          <div v-if="loadingLogs" class="profile-loading">
            <span class="bar-mini"><i></i></span>
          </div>
          <div v-else-if="!logsText" class="profile-empty">
            <p>{{ t('profile.logEmpty') }}</p>
          </div>
          <pre v-else class="logs-viewer">{{ logsText }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>
