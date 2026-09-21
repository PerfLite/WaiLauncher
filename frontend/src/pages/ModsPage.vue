<script setup>
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {
  SearchModpacks,
  GetModpackDetails,
  InstallModpack,
  CancelInstallModpack,
  OpenURL,
} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {renderMarkdown, handleMarkdownClick} from '../utils/markdown'
import modrinthIcon from '../assets/modrinth-icon.png'
import curseforgeIcon from '../assets/curseforge-icon.png'
import ftbIcon from '../assets/ftb-icon.png'

const currentView = ref('catalog') // 'catalog' | 'details'
const source = ref('modrinth') // 'modrinth' | 'curseforge' | 'ftb'
const query = ref('')
const selectedVersion = ref('all')
const selectedLoader = ref('all')
const results = ref([])
const loading = ref(false)

const mcVersions = [
  {id: 'all', label: 'Все версии'},
  {id: '1.21.4', label: '1.21.4'},
  {id: '1.21.1', label: '1.21.1'},
  {id: '1.20.4', label: '1.20.4'},
  {id: '1.20.1', label: '1.20.1'},
  {id: '1.19.4', label: '1.19.4'},
  {id: '1.19.2', label: '1.19.2'},
  {id: '1.18.2', label: '1.18.2'},
  {id: '1.16.5', label: '1.16.5'},
  {id: '1.12.2', label: '1.12.2'},
  {id: '1.7.10', label: '1.7.10'},
]

const loaders = [
  {id: 'all', label: 'Все загрузчики'},
  {id: 'fabric', label: 'Fabric'},
  {id: 'neoforge', label: 'NeoForge'},
  {id: 'forge', label: 'Forge'},
]

/* Detail Page View */
const selectedPack = ref(null)
const packDetails = ref(null)
const loadingDetails = ref(false)
const chosenVersionId = ref('')
const verDropdownOpen = ref(false)
const verSearchQuery = ref('')
const customInstanceName = ref('')
const detailTab = ref('description') // 'description' | 'versions' | 'gallery'
const previewImageIdx = ref(null)

/* Install Progress */
const installing = ref(false)
const installProgress = ref({stage: '', percent: 0, message: '', current: 0, total: 0})

function openGalleryPreview(idx) {
  previewImageIdx.value = idx
}

function closeGalleryPreview() {
  previewImageIdx.value = null
}

function nextGalleryImg() {
  if (!packDetails.value?.gallery?.length) return
  if (previewImageIdx.value < packDetails.value.gallery.length - 1) {
    previewImageIdx.value++
  } else {
    previewImageIdx.value = 0
  }
}

function prevGalleryImg() {
  if (!packDetails.value?.gallery?.length) return
  if (previewImageIdx.value > 0) {
    previewImageIdx.value--
  } else {
    previewImageIdx.value = packDetails.value.gallery.length - 1
  }
}

function onKeydown(e) {
  if (verDropdownOpen.value && e.key === 'Escape') {
    verDropdownOpen.value = false
    return
  }
  if (previewImageIdx.value === null) return
  if (e.key === 'Escape') closeGalleryPreview()
  else if (e.key === 'ArrowRight') nextGalleryImg()
  else if (e.key === 'ArrowLeft') prevGalleryImg()
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('mods-reset-view', backToCatalog)
  search()
  EventsOn('modpack-progress', (p) => {
    installProgress.value = p
    if (p.stage === 'done') {
      installing.value = false
      toast(t('pack.installSuccess') || 'Сборка успешно установлена!')
      // Switch to instances page
      store.page = 'instances'
    } else if (p.stage === 'error') {
      installing.value = false
      toast((t('pack.installErr') || 'Ошибка установки сборки: ') + p.message, true)
    }
  })
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('mods-reset-view', backToCatalog)
})

let searchTimer = null
function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    search()
  }, 400)
}

watch([source, selectedVersion, selectedLoader], () => {
  search()
})

async function search() {
  loading.value = true
  try {
    const list = await SearchModpacks(
      source.value,
      query.value,
      selectedVersion.value === 'all' ? '' : selectedVersion.value,
      selectedLoader.value === 'all' ? '' : selectedLoader.value,
      0,
      40
    )
    results.value = list || []
  } catch (e) {
    results.value = []
  } finally {
    loading.value = false
  }
}

async function openDetails(item) {
  selectedPack.value = item
  customInstanceName.value = item.title
  currentView.value = 'details'
  loadingDetails.value = true
  packDetails.value = null
  chosenVersionId.value = ''
  detailTab.value = 'description'

  try {
    const details = await GetModpackDetails(item.source, item.id)
    packDetails.value = details
    if (details && details.versions && details.versions.length > 0) {
      chosenVersionId.value = details.versions[0].id
    }
  } catch (e) {
    toast(t('inst.err') + e, true)
  } finally {
    loadingDetails.value = false
  }
}

function backToCatalog() {
  if (installing.value) {
    toast('Сборка устанавливается. Нажмите «Отмена» для прерывания.', true)
    return
  }
  currentView.value = 'catalog'
  selectedPack.value = null
  packDetails.value = null
  previewImageIdx.value = null
}

async function cancelInstall() {
  try {
    await CancelInstallModpack()
  } catch (e) {
    console.error('Failed to cancel modpack install:', e)
  }
  installing.value = false
  installProgress.value = {stage: '', percent: 0, message: '', current: 0, total: 0}
  toast(t('pack.installCancelled') || 'Установка сборки отменена')
}

const showInstallModal = ref(false)

function openInstallModal(verId) {
  verDropdownOpen.value = false
  verSearchQuery.value = ''
  if (verId) {
    chosenVersionId.value = verId
  } else if (!chosenVersionId.value && packDetails.value?.versions?.length) {
    chosenVersionId.value = packDetails.value.versions[0].id
  }
  if (!customInstanceName.value && selectedPack.value) {
    customInstanceName.value = selectedPack.value.title
  }
  showInstallModal.value = true
}

function toggleVerDropdown() {
  verDropdownOpen.value = !verDropdownOpen.value
  if (verDropdownOpen.value) {
    verSearchQuery.value = ''
  }
}

function selectVersion(verId) {
  chosenVersionId.value = verId
  verDropdownOpen.value = false
}

const filteredPackVersions = computed(() => {
  if (!packDetails.value?.versions) return []
  const q = verSearchQuery.value.trim().toLowerCase()
  if (!q) return packDetails.value.versions
  return packDetails.value.versions.filter(v => {
    const name = (v.name || '').toLowerCase()
    const num = (v.version_number || '').toLowerCase()
    const gv = (v.game_versions || []).join(' ').toLowerCase()
    const ld = (v.loaders || []).join(' ').toLowerCase()
    return name.includes(q) || num.includes(q) || gv.includes(q) || ld.includes(q)
  })
})

function confirmInstall() {
  showInstallModal.value = false
  doInstall()
}

const chosenVersion = computed(() => {
  if (!packDetails.value || !packDetails.value.versions) return null
  return packDetails.value.versions.find(v => v.id === chosenVersionId.value) || packDetails.value.versions[0]
})

async function doInstall() {
  if (!chosenVersion.value || installing.value) return
  installing.value = true
  installProgress.value = {
    stage: 'downloading',
    percent: 0.05,
    message: 'Начало установки модпака…',
    current: 0,
    total: 0,
  }

  try {
    const name = customInstanceName.value.trim() || selectedPack.value.title
    const vName = chosenVersion.value.version_number || chosenVersion.value.name || chosenVersion.value.id
    const created = await InstallModpack(
      selectedPack.value.source,
      chosenVersion.value.download_url,
      name,
      String(selectedPack.value.id),
      String(chosenVersion.value.id),
      String(vName)
    )
    if (created) {
      store.instances = [...store.instances.filter(i => i.id !== created.id), created]
      store.settings.activeInstance = created.id
      store.settings.selectedVersion = created.versionId
    }
  } catch (e) {
    installing.value = false
    toast((t('pack.installErr') || 'Ошибка установки: ') + e, true)
  }
}

function openWebsite(url) {
  if (!url) return
  try {
    OpenURL(url)
  } catch {
    window.open(url, '_blank')
  }
}

function formatNumber(num) {
  if (!num) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return String(num)
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  try {
    const d = new Date(dateStr)
    return d.toLocaleDateString()
  } catch {
    return dateStr
  }
}

function formatBytes(bytes) {
  if (!bytes) return ''
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
</script>

<template>
  <!-- VIEW 1: MODPACKS CATALOG -->
  <section class="page page-modpacks" v-if="currentView === 'catalog'">
    <!-- Header & Source Switcher -->
    <div class="modpacks-header">
      <div class="modpacks-title-group">
        <h2>{{ t('mods.title') }}</h2>
        <p class="modpacks-sub">{{ t('pack.catalogSub') || 'Готовые сборки и модпаки от мирового сообщества Minecraft' }}</p>
      </div>

      <div class="source-switcher">
        <button
          class="source-tab"
          :class="{active: source === 'modrinth'}"
          @click="source = 'modrinth'"
        >
          <img :src="modrinthIcon" class="source-brand-img modrinth" alt="Modrinth" />
          <span>Modrinth</span>
        </button>
        <button
          class="source-tab"
          :class="{active: source === 'curseforge'}"
          @click="source = 'curseforge'"
        >
          <img :src="curseforgeIcon" class="source-brand-img curseforge" alt="CurseForge" />
          <span>CurseForge</span>
        </button>
        <button
          class="source-tab"
          :class="{active: source === 'ftb'}"
          @click="source = 'ftb'"
        >
          <img :src="ftbIcon" class="source-brand-img ftb" alt="FTB" />
          <span>FTB</span>
        </button>
      </div>
    </div>

    <!-- Filters Toolbar -->
    <div class="modpacks-toolbar">
      <div class="search-box">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
        <input
          type="text"
          v-model="query"
          @input="onSearchInput"
          :placeholder="t('mods.search')"
        >
        <button v-if="query" class="search-clear" @click="query = ''; search()">✕</button>
      </div>

      <div class="filter-group">
        <select class="filter-select" v-model="selectedVersion">
          <option v-for="v in mcVersions" :key="v.id" :value="v.id">{{ v.label }}</option>
        </select>

        <select class="filter-select" v-model="selectedLoader">
          <option v-for="ld in loaders" :key="ld.id" :value="ld.id">{{ ld.label }}</option>
        </select>
      </div>
    </div>

    <!-- Modpacks Grid -->
    <div v-if="loading" class="profile-loading">
      <span class="bar-mini"><i></i></span>
    </div>

    <div v-else-if="results.length === 0" class="profile-empty">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
      </svg>
      <p>{{ t('pack.noPacks') || 'Сборки не найдены. Попробуйте изменить фильтры или поисковый запрос.' }}</p>
    </div>

    <div v-else class="modpacks-grid">
      <div
        v-for="pack in results"
        :key="pack.id"
        class="modpack-card"
        @click="openDetails(pack)"
      >
        <!-- Banner without source badges as requested -->
        <div class="pack-banner-wrap" v-if="pack.banner_url">
          <img :src="pack.banner_url" class="pack-banner" alt="" loading="lazy">
        </div>

        <div class="pack-card-body">
          <div class="pack-head-row">
            <img v-if="pack.icon_url" :src="pack.icon_url" class="pack-avatar" alt="" loading="lazy">
            <div v-else class="pack-avatar-fallback">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/></svg>
            </div>
            <div class="pack-title-col">
              <h4 class="pack-title">{{ pack.title }}</h4>
              <div class="pack-author">{{ t('profile.author') }} <b>{{ pack.author || 'Community' }}</b></div>
            </div>
          </div>

          <p class="pack-desc">{{ pack.description }}</p>

          <div class="pack-card-footer">
            <div class="pack-stats">
              <span class="stat-item" :title="t('profile.downloads')">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
                {{ formatNumber(pack.downloads) }}
              </span>
            </div>

            <button class="btn-pack-install" @click.stop="openDetails(pack)">
              <span>{{ t('pack.detailsBtn') || 'Подробнее' }}</span>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m9 18 6-6-6-6"/></svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>

  <!-- VIEW 2: FULL MODPACK PAGE (ПОЛНОЦЕННАЯ СТРАНИЦА) -->
  <section class="page page-modpack-details" v-else-if="currentView === 'details'">
    <!-- Top Navigation Bar -->
    <div class="mp-full-nav">
      <button class="btn-sec mp-btn-back" @click="backToCatalog">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <path d="m15 18-6-6 6-6"/>
        </svg>
        <span>Каталог сборок</span>
      </button>

      <div class="mp-full-nav-right">
        <button
          v-if="packDetails?.webUrl"
          class="btn-sec mp-btn-web"
          @click="openWebsite(packDetails.webUrl)"
          :title="'Открыть на ' + (selectedPack?.source === 'modrinth' ? 'Modrinth' : 'CurseForge')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6 M15 3h6v6 M10 14 21 3"/></svg>
          <span>На сайт</span>
        </button>
      </div>
    </div>

    <!-- Scrollable Main Content Area -->
    <div class="mp-full-scroll">
      <!-- Hero Banner Wrap -->
      <div v-if="selectedPack?.banner_url || (packDetails?.gallery && packDetails.gallery.length > 0)" class="mp-full-banner-wrap">
        <img :src="selectedPack?.banner_url || packDetails.gallery[0]" class="mp-full-banner" alt="" loading="lazy">
        <div class="mp-full-banner-fade"></div>
      </div>

      <!-- Main Modpack Header Card (Header + Top Install Panel) -->
      <div class="mp-full-header" :class="{'has-banner': selectedPack?.banner_url || (packDetails?.gallery && packDetails.gallery.length > 0)}">
        <div class="mp-header-main-col">
          <div class="mp-header-top-info">
            <img v-if="selectedPack?.icon_url" :src="selectedPack.icon_url" class="mp-full-icon" alt="">
            <div v-else class="mp-full-icon-fallback">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/></svg>
            </div>

            <div class="mp-full-title-block">
              <div class="mp-full-title-row">
                <h1 class="mp-full-title">{{ selectedPack?.title }}</h1>
                <span class="pack-source-tag" :class="selectedPack?.source">
                  <img :src="selectedPack?.source === 'modrinth' ? modrinthIcon : (selectedPack?.source === 'curseforge' ? curseforgeIcon : ftbIcon)" alt="" class="pack-source-mini-img">
                  {{ selectedPack?.source === 'modrinth' ? 'Modrinth' : (selectedPack?.source === 'curseforge' ? 'CurseForge' : 'FTB') }}
                </span>
              </div>

              <div class="mp-full-meta-row">
                <span>{{ t('profile.author') }} <b>{{ selectedPack?.author || 'Community' }}</b></span>
                <span class="dot-sep">•</span>
                <span class="stat-badge" :title="t('profile.downloads')">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="sub-icon"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
                  {{ formatNumber(selectedPack?.downloads) }}
                </span>
                <template v-if="selectedPack?.follows">
                  <span class="dot-sep">•</span>
                  <span class="stat-badge" :title="t('profile.followers') || 'Подписчики'">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="sub-icon"><path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/></svg>
                    {{ formatNumber(selectedPack.follows) }}
                  </span>
                </template>
              </div>
            </div>
          </div>
        </div>

        <!-- TOP INSTALL PANEL (Moved to top as requested) -->
        <div class="mp-header-action-col">
          <!-- When installing: Progress with CANCEL button -->
          <div v-if="installing" class="mp-header-progress-box">
            <div class="mp-prog-head">
              <span class="mp-prog-msg">{{ installProgress.message || 'Установка сборки…' }}</span>
              <span class="mp-prog-pct">{{ Math.floor(installProgress.percent * 100) }}%</span>
            </div>
            <div class="mp-prog-track">
              <div class="mp-prog-fill" :style="{width: (installProgress.percent * 100) + '%'}"></div>
            </div>
            <div class="mp-prog-foot">
              <span class="mp-prog-sub" v-if="installProgress.total > 0">
                Загружено: {{ installProgress.current }} / {{ installProgress.total }}
              </span>
              <span class="mp-prog-sub" v-else>Подготовка…</span>
              <button class="btn-cancel-install" @click="cancelInstall">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M18 6 6 18M6 6l12 12"/></svg>
                <span>Отмена</span>
              </button>
            </div>
          </div>

          <!-- When idle: Just the single clean button as requested -->
          <button
            v-else
            class="btn-primary mp-btn-install-main"
            :disabled="loadingDetails || (!packDetails?.versions?.length && !loadingDetails)"
            @click="openInstallModal()"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
            <span>{{ t('pack.installBtn') || 'Установить сборку' }}</span>
          </button>
        </div>
      </div>

      <!-- Navigation Tabs -->
      <div class="mp-full-tabs">
        <button
          class="mp-tab-btn"
          :class="{active: detailTab === 'description'}"
          @click="detailTab = 'description'"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
          <span>Описание</span>
        </button>
        <button
          class="mp-tab-btn"
          :class="{active: detailTab === 'versions'}"
          @click="detailTab = 'versions'"
        >
          <!-- Fixed versions icon (Tag icon, NO dollar sign!) -->
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
          <span>Версии ({{ packDetails?.versions?.length || 0 }})</span>
        </button>
        <button
          v-if="packDetails?.gallery && packDetails.gallery.length > 0"
          class="mp-tab-btn"
          :class="{active: detailTab === 'gallery'}"
          @click="detailTab = 'gallery'"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>
          <span>Галерея ({{ packDetails.gallery.length }})</span>
        </button>
      </div>

      <!-- Tab Content Area -->
      <div class="mp-full-tab-content">
        <div v-if="loadingDetails" class="pack-details-loading">
          <span class="bar-mini"><i></i></span>
          <p>Загрузка полной информации и описания сборки…</p>
        </div>

        <template v-else>
          <!-- TAB 1: ОПИСАНИЕ -->
          <div v-if="detailTab === 'description'" class="mp-tab-pane">
            <!-- Categories / Loaders Chips -->
            <div class="pack-meta-chips-row" v-if="selectedPack?.categories?.length || selectedPack?.loaders?.length">
              <span class="meta-chip category" v-for="cat in (selectedPack.categories || []).slice(0, 8)" :key="cat">
                {{ cat }}
              </span>
              <span class="meta-chip loader" v-for="ld in (selectedPack.loaders || []).slice(0, 4)" :key="ld">
                {{ ld }}
              </span>
            </div>

            <!-- Formatted Markdown Description -->
            <div
              class="pack-markdown-body md-body"
              v-html="renderMarkdown(packDetails?.body || selectedPack?.description || 'Нет подробного описания.')"
              @click="handleMarkdownClick"
            ></div>
          </div>

          <!-- TAB 2: ВЕРСИИ -->
          <div v-else-if="detailTab === 'versions'" class="mp-tab-pane">
            <div v-if="!packDetails?.versions?.length" class="pack-empty-tab">
              <p>Список версий недоступен.</p>
            </div>
            <div v-else class="pack-versions-list">
              <div
                v-for="ver in packDetails.versions"
                :key="ver.id"
                class="pack-ver-row"
                :class="{selected: chosenVersionId === ver.id}"
                @click="chosenVersionId = ver.id"
              >
                <div class="pack-ver-radio">
                  <input type="radio" :value="ver.id" v-model="chosenVersionId" />
                </div>
                <div class="pack-ver-info">
                  <div class="pack-ver-name-row">
                    <span class="pack-ver-title">{{ ver.name || ver.version_number }}</span>
                    <span v-if="chosenVersionId === ver.id" class="pack-ver-selected-badge">Выбрана</span>
                  </div>
                  <div class="pack-ver-meta-row">
                    <span class="pack-ver-chip mc" v-for="gv in (ver.game_versions || []).slice(0, 4)" :key="gv">
                      MC: {{ gv }}
                    </span>
                    <span class="pack-ver-chip loader" v-for="ld in (ver.loaders || []).slice(0, 3)" :key="ld">
                      {{ ld }}
                    </span>
                    <span class="pack-ver-date" v-if="ver.date_published">{{ formatDate(ver.date_published) }}</span>
                    <span class="pack-ver-size" v-if="ver.file_size">{{ formatBytes(ver.file_size) }}</span>
                  </div>
                </div>
                <button
                  class="btn-primary btn-ver-install"
                  @click.stop="openInstallModal(ver.id)"
                  :disabled="installing"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
                  <span>Установить</span>
                </button>
              </div>
            </div>
          </div>

          <!-- TAB 3: ГАЛЕРЕЯ -->
          <div v-else-if="detailTab === 'gallery'" class="mp-tab-pane">
            <div class="pack-gallery-grid">
              <div
                v-for="(imgUrl, idx) in packDetails.gallery"
                :key="idx"
                class="pack-gallery-item"
                @click="openGalleryPreview(idx)"
              >
                <img :src="imgUrl" alt="" loading="lazy">
                <div class="pack-gallery-overlay">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5 M11 8v6 M8 11h6"/></svg>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Fullscreen Gallery Lightbox Modal (Teleported to body) -->
    <Teleport to="body">
      <div
        class="gallery-lightbox-overlay"
        v-if="previewImageIdx !== null && packDetails?.gallery?.length"
        @click.self="closeGalleryPreview"
      >
        <!-- Top bar with counter and close button -->
        <div class="gallery-lightbox-topbar">
          <span class="gallery-lightbox-counter">
            Изображение {{ previewImageIdx + 1 }} из {{ packDetails.gallery.length }}
          </span>
          <button class="gallery-lightbox-close" @click="closeGalleryPreview" title="Закрыть (Esc)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <!-- Stage with navigation arrows and centered image -->
        <div class="gallery-lightbox-stage" @click.self="closeGalleryPreview">
          <button
            v-if="packDetails.gallery.length > 1"
            class="gallery-nav-arrow prev"
            @click.stop="prevGalleryImg"
            title="Предыдущее (Стрелка влево)"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="m15 18-6-6 6-6"/></svg>
          </button>

          <div class="gallery-img-container" @click.self="closeGalleryPreview">
            <img :src="packDetails.gallery[previewImageIdx]" alt="" class="gallery-lightbox-img">
          </div>

          <button
            v-if="packDetails.gallery.length > 1"
            class="gallery-nav-arrow next"
            @click.stop="nextGalleryImg"
            title="Следующее (Стрелка вправо)"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="m9 18 6-6-6-6"/></svg>
          </button>
        </div>
      </div>
    </Teleport>

    <!-- Modpack Install Confirmation Modal -->
    <Teleport to="body">
      <div class="modal-root" v-if="showInstallModal">
        <div class="mp-install-modal-backdrop" @click="showInstallModal = false"></div>
        <div class="modal-box mp-install-modal-box">
          <div class="modal-header mp-install-modal-header">
            <div class="modal-title-group mp-install-header-group">
              <img v-if="selectedPack?.icon_url" :src="selectedPack.icon_url" class="mp-modal-header-icon" alt="">
              <div v-else class="mp-modal-header-fallback">
                <span>{{ (selectedPack?.title || 'MP').slice(0, 2).toUpperCase() }}</span>
              </div>
              <div class="mp-install-header-info">
                <h3 class="modal-title mp-install-header-title">{{ selectedPack?.title || 'Установка сборки' }}</h3>
                <p class="modal-subtitle mp-install-header-sub">Установка сборки • Автор: <b>{{ selectedPack?.author || 'Community' }}</b></p>
              </div>
            </div>
            <button class="modal-close" @click="showInstallModal = false">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
            </button>
          </div>

          <div class="modal-body mp-install-modal-body">
            <!-- Form: Name -->
            <div class="mp-form-row">
              <label class="fld-label">Название сборки</label>
              <input
                class="txt-in mp-form-input"
                v-model="customInstanceName"
                :placeholder="selectedPack?.title || 'Моя сборка'"
              />
              <span class="mp-form-hint">Имя для отображения в списке ваших сборок</span>
            </div>

            <!-- Form: Version -->
            <div class="mp-form-row mp-form-row-version" v-if="packDetails?.versions?.length">
              <label class="fld-label">Версия модпака</label>

              <div class="mp-custom-dropdown" :class="{ open: verDropdownOpen }">
                <!-- Trigger Button -->
                <button
                  type="button"
                  class="mp-dropdown-btn"
                  @click="toggleVerDropdown"
                >
                  <div class="mp-dd-btn-left" v-if="chosenVersion">
                    <span class="mp-dd-btn-title">{{ chosenVersion.name || chosenVersion.version_number }}</span>
                    <span class="mp-dd-chip mc-chip" v-if="chosenVersion.game_versions?.length">
                      MC {{ chosenVersion.game_versions.slice(0, 2).join(', ') }}
                    </span>
                    <span class="mp-dd-chip loader-chip" v-if="chosenVersion.loaders?.length">
                      {{ chosenVersion.loaders.join(', ') }}
                    </span>
                  </div>
                  <span v-else class="mp-dd-placeholder">Выберите версию...</span>

                  <svg class="mp-dd-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                    <polyline points="6 9 12 15 18 9"></polyline>
                  </svg>
                </button>

                <!-- Backdrop to close dropdown -->
                <div v-if="verDropdownOpen" class="mp-dd-backdrop" @click="verDropdownOpen = false"></div>

                <!-- Dropdown Menu -->
                <div v-if="verDropdownOpen" class="mp-dropdown-menu">
                  <!-- Search input -->
                  <div class="mp-dd-search-wrap">
                    <svg class="mp-dd-search-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="11" cy="11" r="8"></circle>
                      <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                    </svg>
                    <input
                      v-model="verSearchQuery"
                      class="mp-dd-search-input"
                      placeholder="Поиск версии (напр. 1.21)..."
                      @click.stop
                    />
                    <button
                      v-if="verSearchQuery"
                      type="button"
                      class="mp-dd-search-clear"
                      @click.stop="verSearchQuery = ''"
                    >
                      ✕
                    </button>
                  </div>

                  <!-- Versions List -->
                  <ul class="mp-dd-list">
                    <li
                      v-for="ver in filteredPackVersions"
                      :key="ver.id"
                      class="mp-dd-item"
                      :class="{ selected: ver.id === chosenVersionId }"
                      @click="selectVersion(ver.id)"
                    >
                      <div class="mp-dd-item-main">
                        <span class="mp-dd-item-title">{{ ver.name || ver.version_number }}</span>
                        <div class="mp-dd-item-chips">
                          <span class="mp-dd-chip mc-chip" v-if="ver.game_versions?.length">
                            MC {{ ver.game_versions.slice(0, 2).join(', ') }}
                          </span>
                          <span class="mp-dd-chip loader-chip" v-if="ver.loaders?.length">
                            {{ ver.loaders.join(', ') }}
                          </span>
                        </div>
                      </div>
                      <svg v-if="ver.id === chosenVersionId" class="mp-dd-check" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                        <polyline points="20 6 9 17 4 12"></polyline>
                      </svg>
                    </li>
                    <li v-if="filteredPackVersions.length === 0" class="mp-dd-empty">
                      Версии не найдены
                    </li>
                  </ul>
                </div>
              </div>

              <span class="mp-form-hint" v-if="chosenVersion">
                Загрузчик: {{ (chosenVersion.loaders || []).join(', ') || 'Auto' }} • Дата: {{ formatDate(chosenVersion.date_published) }}
              </span>
            </div>
          </div>

          <div class="modal-foot">
            <button class="btn-sec" @click="showInstallModal = false">Отмена</button>
            <button
              class="btn-primary btn-do-install-modal"
              :disabled="!chosenVersion"
              @click="confirmInstall"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
              <span>Установить сборку</span>
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </section>
</template>
