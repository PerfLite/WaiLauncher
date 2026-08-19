<script setup>
import {computed, onMounted, ref, watch} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {
  SearchModpacks,
  GetModpackDetails,
  InstallModpack,
  SearchModrinthMods,
  InstallModrinthMod,
} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'

const source = ref('modrinth') // 'modrinth' | 'curseforge'
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
  {id: 'quilt', label: 'Quilt'},
]

/* Detail Modal */
const detailOpen = ref(false)
const selectedPack = ref(null)
const packDetails = ref(null)
const loadingDetails = ref(false)
const chosenVersionId = ref('')
const customInstanceName = ref('')

/* Install Progress */
const installing = ref(false)
const installProgress = ref({stage: '', percent: 0, message: '', current: 0, total: 0})

onMounted(() => {
  search()
  loadPopular()
  EventsOn('modpack-progress', (p) => {
    installProgress.value = p
    if (p.stage === 'done') {
      installing.value = false
      detailOpen.value = false
      toast(t('pack.installSuccess') || 'Сборка успешно установлена!')
      // Switch to instances page
      store.page = 'instances'
    } else if (p.stage === 'error') {
      installing.value = false
      toast((t('pack.installErr') || 'Ошибка установки сборки: ') + p.message, true)
    }
  })
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
  detailOpen.value = true
  loadingDetails.value = true
  packDetails.value = null
  chosenVersionId.value = ''

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

function closeDetails() {
  if (installing.value) return
  detailOpen.value = false
  selectedPack.value = null
  packDetails.value = null
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
    const created = await InstallModpack(
      selectedPack.value.source,
      chosenVersion.value.download_url,
      name
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

function formatNumber(num) {
  if (!num) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return String(num)
}

/* Popular shaders & resource packs for the active instance */
const activeInst = computed(() =>
  store.instances.find(i => i.id === store.settings.activeInstance) || store.instances[0] || null
)
const popularContent = ref([]) // {slides: 'shader'|'resourcepack', items: [...] }
const loadingPopular = ref(false)
const installingPopMap = ref({})

async function loadPopular() {
  if (!activeInst.value) {
    popularContent.value = []
    return
  }
  loadingPopular.value = true
  try {
    const [sh, rp] = await Promise.all([
      SearchModrinthMods('', 'shader', '', '', 0, 8).catch(() => null),
      SearchModrinthMods('', 'resourcepack', '', '', 0, 8).catch(() => null)
    ])
    popularContent.value = [
      {kind: 'shader', title: t('cat.shaders'), items: (sh && sh.hits) || []},
      {kind: 'resourcepack', title: t('cat.resourcepacks'), items: (rp && rp.hits) || []},
    ].filter(g => g.items.length > 0)
  } catch (e) {
    popularContent.value = []
  } finally {
    loadingPopular.value = false
  }
}

async function installPopular(item, kind) {
  if (!activeInst.value || installingPopMap.value[item.project_id]) return
  installingPopMap.value[item.project_id] = true
  try {
    await InstallModrinthMod(activeInst.value.id, item.project_id, kind)
    toast((t('mr.installed') || 'Установлено: ') + item.title)
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    installingPopMap.value[item.project_id] = false
  }
}

watch(activeInst, () => loadPopular())
</script>

<template>
  <section class="page page-modpacks">
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
          <span class="source-icon modrinth"></span>
          <span>Modrinth</span>
        </button>
        <button
          class="source-tab"
          :class="{active: source === 'curseforge'}"
          @click="source = 'curseforge'"
        >
          <span class="source-icon curseforge"></span>
          <span>CurseForge</span>
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

    <!-- Popular shaders / resourcepacks for the active build -->
    <div v-if="activeInst && popularContent.length" class="popular-section">
      <div v-for="group in popularContent" :key="group.kind" class="popular-group">
        <div class="popular-group-title">
          <span>{{ group.kind === 'shader' ? '✨ ' : '🖌️ ' }}{{ group.title }} — {{ t('pop.for') || 'для сборки' }} «{{ activeInst.name }}»</span>
        </div>
        <div class="popular-scroll">
          <div v-for="item in group.items" :key="item.project_id" class="pop-card">
            <img v-if="item.icon_url" :src="item.icon_url" class="pop-icon" alt="" loading="lazy">
            <div v-else class="pop-icon pop-icon-fallback">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="3"/></svg>
            </div>
            <div class="pop-info">
              <div class="pop-name" :title="item.title">{{ item.title }}</div>
              <div class="pop-dl">↓ {{ formatNumber(item.downloads) }}</div>
            </div>
            <button
              class="pop-install-btn"
              :disabled="installingPopMap[item.project_id]"
              :title="t('pack.installBtn') || 'Установить'"
              @click="installPopular(item, group.kind)"
            >
              <svg v-if="!installingPopMap[item.project_id]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
              <span v-else>…</span>
            </button>
          </div>
        </div>
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
        <div class="pack-banner-wrap" v-if="pack.banner_url">
          <img :src="pack.banner_url" class="pack-banner" alt="" loading="lazy">
          <div class="pack-source-badge" :class="pack.source">
            {{ pack.source === 'modrinth' ? 'Modrinth' : 'CurseForge' }}
          </div>
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

    <!-- Modpack Detail & Install Modal -->
    <div class="modal-root" v-if="detailOpen">
      <div class="modal-backdrop" @click="closeDetails"></div>
      <div class="modal-box pack-detail-modal">
        <div class="modal-header">
          <div class="modal-title-group">
            <img v-if="selectedPack?.icon_url" :src="selectedPack.icon_url" class="pack-modal-icon" alt="">
            <div>
              <h3 class="modal-title">{{ selectedPack?.title }}</h3>
              <div class="modal-subtitle">
                {{ t('profile.author') }} {{ selectedPack?.author }} •
                <span class="pack-source-tag" :class="selectedPack?.source">
                  {{ selectedPack?.source === 'modrinth' ? 'Modrinth' : 'CurseForge' }}
                </span>
              </div>
            </div>
          </div>
          <button class="modal-close" :disabled="installing" @click="closeDetails">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="modal-body pack-modal-body">
          <div v-if="loadingDetails" class="profile-loading">
            <span class="bar-mini"><i></i></span>
          </div>

          <template v-else>
            <!-- Name & Version Config -->
            <div class="pack-config-box">
              <div class="pack-fld-row">
                <label class="fld-label">{{ t('inst.name') }}</label>
                <input class="txt-in full-w" v-model="customInstanceName" :placeholder="t('inst.namePh')" :disabled="installing">
              </div>

              <div class="pack-fld-row" v-if="packDetails?.versions?.length">
                <label class="fld-label">{{ t('pack.selectVer') || 'Версия модпака' }}</label>
                <select class="sel full-w" v-model="chosenVersionId" :disabled="installing">
                  <option
                    v-for="v in packDetails.versions"
                    :key="v.id"
                    :value="v.id"
                  >
                    {{ v.name || v.version_number }} (MC: {{ v.game_versions?.join(', ') || 'Auto' }})
                  </option>
                </select>
              </div>
            </div>

            <!-- Install Progress Overlay if active -->
            <div v-if="installing" class="pack-install-progress-card">
              <div class="progress-title-row">
                <span class="progress-msg">{{ installProgress.message || 'Установка…' }}</span>
                <span class="progress-pct">{{ Math.floor(installProgress.percent * 100) }}%</span>
              </div>
              <div class="pack-progress-bar">
                <i :style="{width: (installProgress.percent * 100) + '%'}"></i>
              </div>
              <div class="progress-status-sub" v-if="installProgress.total > 0">
                Загружено модов: {{ installProgress.current }} из {{ installProgress.total }}
              </div>
            </div>

            <!-- Description -->
            <div class="pack-modal-desc">
              <h4>{{ t('pack.about') || 'Описание сборки' }}</h4>
              <p class="pack-modal-summary">{{ selectedPack?.description }}</p>
            </div>
          </template>
        </div>

        <div class="modal-foot">
          <button class="btn-sec" :disabled="installing" @click="closeDetails">{{ t('inst.cancel') }}</button>
          <button
            class="btn-primary btn-do-install"
            :disabled="installing || !chosenVersion"
            @click="doInstall"
          >
            <svg v-if="!installing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
            <span>{{ installing ? (t('profile.installing') || 'Установка…') : (t('pack.installBtn') || 'Установить сборку') }}</span>
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
