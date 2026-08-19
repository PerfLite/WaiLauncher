<script setup>
import {ref, computed, watch, onMounted} from 'vue'
import {store, toast, reloadNews, activeAccount, getAccountAvatar} from '../store'
import {t, setLang} from '../i18n'
import {SaveSettings, GetSettings, PickJavaPath, PickDataDir, OpenDataDir, OpenLogsFolder, OpenURL, GetJavaRuntimesStatus, InstallJavaRuntime, UninstallJavaRuntime, CheckLauncherUpdate, CheckJavaUpdates} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import skinFallback from '../assets/skin.png'
import logoImg from '../assets/logo.png'

const resolutions = ['1280 × 720', '1600 × 900', '1920 × 1080', '2560 × 1440', '3840 × 2160']

const currentAcc = computed(() => activeAccount())
const avatarUrl = computed(() => getAccountAvatar(currentAcc.value))

const ramGb = computed({
  get: () => Math.round(store.settings.ramMb / 1024),
  set: (gb) => { store.settings.ramMb = gb * 1024 },
})

/* About Modal */
const aboutModalOpen = ref(false)
function openExternal(url) {
  if (url) OpenURL(url).catch(() => {})
}

/* Java Runtimes */
const javaRuntimes = ref([])
const loadingJavaRuntimes = ref(false)
const installingJavaMap = ref({})
const javaProgress = ref({})
const javaToDelete = ref(null)
const deletingJava = ref(false)
const javaUpdates = ref({}) // major -> {installedVersion, latestVersion}
const checkingJavaUpdates = ref(false)

async function loadJavaRuntimes() {
  loadingJavaRuntimes.value = true
  try {
    const list = await GetJavaRuntimesStatus()
    javaRuntimes.value = list || []
  } catch (e) {
    javaRuntimes.value = []
  } finally {
    loadingJavaRuntimes.value = false
  }
}

async function checkJavaUpdates() {
  checkingJavaUpdates.value = true
  try {
    const list = await CheckJavaUpdates()
    const map = {}
    for (const u of (list || [])) {
      if (u.updateAvailable) map[u.major] = u
    }
    javaUpdates.value = map
    if (Object.keys(map).length > 0) {
      toast(t('settings.javaUpdateFound') || 'Доступны обновления Java')
    } else {
      toast(t('settings.javaUpToDate') || 'Установлены последние версии Java')
    }
  } catch (e) {
    /* silent */
  } finally {
    checkingJavaUpdates.value = false
  }
}

async function onUpdateJava(major) {
  const upd = javaUpdates.value[major]
  if (!upd || installingJavaMap.value[major]) return
  delete javaUpdates.value[major]
  installingJavaMap.value[major] = true
  javaProgress.value[major] = 0
  try {
    await InstallJavaRuntime(major)
    await loadJavaRuntimes()
    toast(t('settings.javaUpdateDone') || ('Java ' + major + ' обновлена'))
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    installingJavaMap.value[major] = false
  }
}

async function onInstallJava(major) {
  installingJavaMap.value[major] = true
  javaProgress.value[major] = 0
  try {
    await InstallJavaRuntime(major)
    await loadJavaRuntimes()
    toast(t('settings.javaInstalledSuccess'))
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    installingJavaMap.value[major] = false
    delete javaProgress.value[major]
  }
}

function promptDeleteJava(runtime) {
  javaToDelete.value = runtime
}

async function confirmDeleteJava() {
  if (!javaToDelete.value || deletingJava.value) return
  const major = javaToDelete.value.major
  deletingJava.value = true
  try {
    await UninstallJavaRuntime(major)
    javaToDelete.value = null
    await loadJavaRuntimes()
    toast(t('settings.javaUninstalled').replace('{major}', major))
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    deletingJava.value = false
  }
}

onMounted(() => {
  loadJavaRuntimes()
  EventsOn('java-install-progress', (ev) => {
    if (ev && ev.major) {
      javaProgress.value[ev.major] = Math.floor(ev.percent || 0)
    }
  })
})

async function browseJava() {
  try {
    const p = await PickJavaPath()
    if (p) store.settings.javaPath = p
  } catch (e) { /* preview mode */ }
}

async function browseDataDir() {
  try {
    const p = await PickDataDir()
    if (p) {
      store.settings.dataDir = p
      store.dataDir = p
    }
  } catch (e) { /* preview mode */ }
}

function pickLang(l) {
  store.settings.language = l
  setLang(l)
  // Persist first so the backend localizes re-fetched news (tags, dates).
  SaveSettings(store.settings).then(reloadNews).catch(() => {})
}

// Autosave: persist any settings change (toggles, fields) with a small
// debounce so nothing is lost even without clicking "Save".
let autosaveTimer = null
watch(() => store.settings, () => {
  clearTimeout(autosaveTimer)
  autosaveTimer = setTimeout(() => {
    SaveSettings(store.settings).catch(() => {})
  }, 400)
}, {deep: true})

async function save() {
  try {
    await SaveSettings(store.settings)
    toast(t('settings.saved'))
  } catch (e) {
    toast(t('settings.saveErr') + e, true)
  }
}

async function reset() {
  try {
    store.settings = await GetSettings()
    setLang(store.settings.language)
    toast(t('settings.resetDone'), true)
  } catch (e) {
    toast(t('settings.noBackend'), true)
  }
}

function onAvatarError(e) {
  e.target.src = skinFallback
}

async function openLogs() {
  try {
    await OpenLogsFolder()
  } catch (e) {
    toast('Не удалось открыть папку с логами: ' + e, true)
  }
}

/* Launcher update check */
const checkingUpdate = ref(false)
async function checkUpdateNow() {
  if (checkingUpdate.value) return
  checkingUpdate.value = true
  try {
    const info = await CheckLauncherUpdate()
    if (info && info.error) {
      toast(t('update.checkErr') + info.error, true)
    } else if (info && info.updateAvailable) {
      store.launcherUpdate.info = info
      store.launcherUpdate.error = ''
      store.launcherUpdate.modalOpen = true
    } else {
      toast(t('update.notAvailable'))
    }
  } catch (e) {
    toast((t('update.checkErr') || 'Update check failed: ') + e, true)
  } finally {
    checkingUpdate.value = false
  }
}
</script>

<template>
  <section class="page">
    <div class="section-head" style="margin-top:0"><h2>{{ t('settings.title') }}</h2></div>
    <div class="settings-panel">

      <div class="set-group">
        <h3>{{ t('settings.profile') }}</h3>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.username') }}</div>
            <div class="set-desc">{{ t('settings.usernameDesc') }}</div>
          </div>
          <div class="set-ctrl">
            <div class="settings-account-card">
              <img :src="avatarUrl" @error="onAvatarError" alt="Avatar" class="settings-acc-avatar">
              <div class="settings-acc-meta">
                <span class="settings-acc-name">{{ currentAcc?.username || store.settings.username }}</span>
                <span class="acc-type-pill" :class="currentAcc?.type">
                  {{ currentAcc?.type === 'microsoft' ? t('accounts.type.microsoft') : t('accounts.type.offline') }}
                </span>
              </div>
              <button class="btn-sec" @click="store.accountsModalOpen = true">
                {{ t('settings.accountManage') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="set-group">
        <h3>{{ t('settings.game') }}</h3>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.windowOverride') }}</div>
            <div class="set-desc">{{ t('settings.windowOverrideDesc') }}</div>
          </div>
          <div class="set-ctrl"><label class="switch"><input type="checkbox" v-model="store.settings.windowCustom"><i></i></label></div>
        </div>

        <template v-if="store.settings.windowCustom">
          <div class="set-row">
            <div>
              <div class="set-name">{{ t('settings.fullscreen') }}</div>
              <div class="set-desc">{{ t('settings.fullscreenDesc') }}</div>
            </div>
            <div class="set-ctrl"><label class="switch"><input type="checkbox" v-model="store.settings.fullscreen"><i></i></label></div>
          </div>

          <template v-if="!store.settings.fullscreen">
            <div class="set-row">
              <div>
                <div class="set-name">{{ t('settings.width') }}</div>
                <div class="set-desc">{{ t('settings.widthDesc') }}</div>
              </div>
              <div class="set-ctrl">
                <input type="number" class="txt-in num-in" v-model.number="store.settings.windowWidth" min="320" max="7680" placeholder="854">
              </div>
            </div>
            <div class="set-row">
              <div>
                <div class="set-name">{{ t('settings.height') }}</div>
                <div class="set-desc">{{ t('settings.heightDesc') }}</div>
              </div>
              <div class="set-ctrl">
                <input type="number" class="txt-in num-in" v-model.number="store.settings.windowHeight" min="240" max="4320" placeholder="480">
              </div>
            </div>
          </template>
        </template>

        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.centerWindow') }}</div>
            <div class="set-desc">{{ t('settings.centerWindowDesc') }}</div>
          </div>
          <div class="set-ctrl"><label class="switch"><input type="checkbox" v-model="store.settings.centerWindow"><i></i></label></div>
        </div>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.dataDir') }}</div>
            <div class="set-desc">{{ t('settings.dataDirDesc') }}</div>
          </div>
          <div class="set-ctrl">
            <input class="txt-in" :value="store.settings.dataDir || store.dataDir" readonly>
            <button class="btn-sec" @click="browseDataDir">{{ t('settings.browse') }}</button>
            <button class="btn-sec" @click="OpenDataDir">{{ t('settings.open') }}</button>
          </div>
        </div>
      </div>

      <div class="set-group">
        <h3>{{ t('settings.perf') }}</h3>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.ram') }}</div>
            <div class="set-desc">{{ t('settings.ramDesc') }}</div>
          </div>
          <div class="set-ctrl">
            <input type="range" min="2" max="16" step="1" v-model.number="ramGb">
            <span class="range-val">{{ ramGb }} {{ t('settings.gb') }}</span>
          </div>
        </div>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.res') }}</div>
            <div class="set-desc">{{ t('settings.resDesc') }}</div>
          </div>
          <div class="set-ctrl">
            <select class="sel" v-model="store.settings.resolution">
              <option v-for="r in resolutions" :key="r">{{ r }}</option>
            </select>
          </div>
        </div>
      </div>

      <div class="set-group">
        <h3>{{ t('settings.launcher') }}</h3>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.langName') }}</div>
            <div class="set-desc">{{ t('settings.langDesc') }}</div>
          </div>
          <div class="set-ctrl">
            <div class="lang-switch">
              <button :class="{on: store.settings.language !== 'en'}" @click="pickLang('ru')">Русский</button>
              <button :class="{on: store.settings.language === 'en'}" @click="pickLang('en')">English</button>
            </div>
          </div>
        </div>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.autoUpdate') }}</div>
            <div class="set-desc">{{ t('settings.autoUpdateDesc') }}</div>
          </div>
          <div class="set-ctrl"><label class="switch"><input type="checkbox" v-model="store.settings.autoUpdate"><i></i></label></div>
        </div>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.launcherUpdates') }}</div>
            <div class="set-desc">{{ t('settings.launcherUpdatesDesc') }}</div>
          </div>
          <div class="set-ctrl update-ctrl">
            <button class="btn-sec update-check-btn" :disabled="checkingUpdate" @click="checkUpdateNow">
              <span v-if="checkingUpdate">{{ t('home.loading') }}</span>
              <span v-else>{{ t('settings.checkUpdates') }}</span>
            </button>
            <label class="switch"><input type="checkbox" v-model="store.settings.launcherUpdates"><i></i></label>
          </div>
        </div>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.snapshots') }}</div>
            <div class="set-desc">{{ t('settings.snapshotsDesc') }}</div>
          </div>
          <div class="set-ctrl"><label class="switch"><input type="checkbox" v-model="store.settings.showSnapshots"><i></i></label></div>
        </div>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.hide') }}</div>
            <div class="set-desc">{{ t('settings.hideDesc') }}</div>
          </div>
          <div class="set-ctrl"><label class="switch"><input type="checkbox" v-model="store.settings.closeOnLaunch"><i></i></label></div>
        </div>
        <div class="set-row">
          <div>
            <div class="set-name">{{ t('settings.discord') }}</div>
            <div class="set-desc">{{ t('settings.discordDesc') }}</div>
          </div>
          <div class="set-ctrl"><label class="switch"><input type="checkbox" v-model="store.settings.discordRpc"><i></i></label></div>
        </div>
        <div class="set-row" v-if="store.settings.discordRpc">
          <div>
            <div class="set-name">Discord Application ID</div>
            <div class="set-desc">{{ t('settings.discordAppIdDesc') }}</div>
          </div>
          <div class="set-ctrl">
            <input class="txt-in" style="width:220px" v-model="store.settings.discordAppId" placeholder="123456789012345678">
          </div>
        </div>
      </div>

      <div class="set-group">
        <div class="set-group-head">
          <h3>{{ t('settings.javaRuntimes') }}</h3>
          <button class="btn-sec java-check-btn" :disabled="checkingJavaUpdates" @click="checkJavaUpdates">
            {{ checkingJavaUpdates ? '…' : (t('settings.javaCheckUpdates') || 'Проверить обновления') }}
          </button>
        </div>
        <div class="java-runtimes-list">
          <div v-for="j in javaRuntimes" :key="j.major" class="java-runtime-card" :class="{downloading: installingJavaMap[j.major]}">
            <div class="java-runtime-info">
              <div class="java-runtime-title">
                <strong>OpenJDK {{ j.major }}</strong>
                <span class="java-runtime-tag" :class="j.found ? 'installed' : (installingJavaMap[j.major] ? 'downloading' : 'missing')">
                  {{ j.found ? t('settings.javaStatusInstalled') : (installingJavaMap[j.major] ? 'Загрузка ' + (javaProgress[j.major] || 0) + '%' : t('settings.javaStatusMissing')) }}
                </span>
              </div>
              <div class="java-runtime-path" :title="j.path">
                {{ j.found ? (j.managed ? 'WaiLauncher Managed' : j.path) : (j.major === 8 ? 'Minecraft 1.12.2 и старее' : j.major === 17 ? 'Minecraft 1.17 – 1.20.4' : 'Minecraft 1.20.5+') }}
              </div>

              <!-- Live Progress Bar when installing -->
              <div v-if="installingJavaMap[j.major]" class="java-progress-wrap">
                <div class="java-progress-bar">
                  <i :style="{width: Math.max(2, (javaProgress[j.major] || 0)) + '%'}"></i>
                </div>
                <div class="java-progress-text">
                  <span>{{ (javaProgress[j.major] >= 95 ? 'Распаковка OpenJDK…' : 'Скачивание OpenJDK Temurin ' + j.major + '…') }}</span>
                  <span class="java-pct-val">{{ javaProgress[j.major] || 0 }}%</span>
                </div>
              </div>
            </div>

            <div class="java-runtime-actions">
              <button
                v-if="!j.found"
                class="mr-btn-green"
                :class="{loading: installingJavaMap[j.major]}"
                style="height: 36px; padding: 0 16px; font-size: 13px;"
                :disabled="installingJavaMap[j.major]"
                @click="onInstallJava(j.major)"
              >
                <span v-if="installingJavaMap[j.major]" class="java-btn-spinner"></span>
                <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width: 14px; height: 14px;"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                <span>{{ installingJavaMap[j.major] ? (javaProgress[j.major] || 0) + '%' : t('settings.installJava') }}</span>
              </button>
              <div v-else class="java-installed-actions">
                <template v-if="javaUpdates[j.major]">
                  <span class="java-update-badge" :title="'Установлена ' + javaUpdates[j.major].installedVersion + ' → доступна ' + javaUpdates[j.major].latestVersion">
                    ↑ {{ javaUpdates[j.major].latestVersion }}
                  </span>
                  <button
                    v-if="j.managed"
                    class="mr-btn-green"
                    style="height: 36px; padding: 0 16px; font-size: 13px;"
                    :disabled="installingJavaMap[j.major]"
                    @click="onUpdateJava(j.major)"
                  >
                    <span v-if="installingJavaMap[j.major]" class="java-btn-spinner"></span>
                    <span v-else>{{ t('settings.javaUpdate') || 'Обновить' }}</span>
                  </button>
                </template>
                <span v-else class="java-installed-badge">✓ Готова к запуску</span>
                <button
                  v-if="j.managed"
                  class="mr-btn-action-icon mr-btn-trash"
                  :title="t('settings.uninstallJava')"
                  @click="promptDeleteJava(j)"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M3 6h18 M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="set-row" style="margin-top: 14px;">
          <div>
            <div class="set-name">{{ t('settings.javaPath') }}</div>
            <div class="set-desc">{{ t('settings.javaPathDesc') }}</div>
          </div>
          <div class="set-ctrl">
            <input class="txt-in" v-model="store.settings.javaPath" readonly :placeholder="t('settings.autoPh')">
            <button class="btn-sec" @click="browseJava">{{ t('settings.browse') }}</button>
            <button class="btn-sec" v-if="store.settings.javaPath" @click="store.settings.javaPath = ''">{{ t('settings.auto') }}</button>
          </div>
        </div>
      </div>

      <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap">
        <button class="btn-primary" @click="save">{{ t('settings.save') }}</button>
        <button class="btn-sec" style="height:46px" @click="reset">{{ t('settings.reset') }}</button>
        <button class="btn-sec" style="height:46px;display:inline-flex;align-items:center;gap:8px" @click="aboutModalOpen = true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:16px;height:16px"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
          <span>{{ t('settings.about') }}</span>
        </button>
        <button class="btn-sec" style="height:46px;display:inline-flex;align-items:center;gap:8px;margin-left:auto" @click="openLogs" title="Открыть папку с файлом launcher.log">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:16px;height:16px"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>
          <span>Логи лаунчера</span>
        </button>
      </div>
    </div>

    <!-- About Modal -->
    <div class="modal-root" v-if="aboutModalOpen">
      <div class="modal-backdrop" @click="aboutModalOpen = false"></div>
      <div class="modal-box about-modal-box">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('about.title') }}</h3>
          <button class="modal-close" @click="aboutModalOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body about-modal-body">
          <div class="about-hero">
            <img :src="logoImg" alt="WaiLauncher" class="about-logo-img">
            <h2 class="about-app-title">Wai<span class="highlight">Launcher</span></h2>
            <p class="about-app-version">v{{ store.launcherVer || '0.1.0' }} &mdash; {{ t('about.desc') }}</p>
          </div>

          <!-- Developer section -->
          <div class="about-card-section">
            <div class="about-sec-title">{{ t('about.developer') }}</div>
            <div class="about-links-col">
              <div class="about-link-row" @click="openExternal('https://github.com/PerfLite')">
                <svg class="about-link-icon" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/></svg>
                <span class="about-link-label">GitHub: <strong class="about-highlight">PerfLite</strong></span>
                <span class="about-link-arrow">↗</span>
              </div>
              <div class="about-link-row" @click="openExternal('https://t.me/bashakul')">
                <svg class="about-link-icon" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221l-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.446 1.394c-.14.18-.357.295-.6.295-.002 0-.003 0-.005 0l.213-3.054 5.56-5.022c.24-.213-.054-.334-.373-.121l-6.869 4.326-2.96-.924c-.643-.204-.657-.643.136-.953l11.57-4.458c.538-.196 1.006.128.832.943z"/></svg>
                <span class="about-link-label">Telegram: <strong class="about-highlight">PerfLite</strong></span>
                <span class="about-link-arrow">↗</span>
              </div>
            </div>
          </div>

          <!-- Built with section -->
          <div class="about-card-section">
            <div class="about-sec-title">{{ t('about.builtWith') }}</div>
            <div class="about-tags-row">
              <span class="about-tag tag-go">Go</span>
              <span class="about-tag tag-wails">Wails v2</span>
              <span class="about-tag tag-vue">Vue 3</span>
              <span class="about-tag tag-vite">Vite</span>
              <span class="about-tag tag-webview">WebView2</span>
              <span class="about-tag tag-modrinth">Modrinth API</span>
            </div>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn-primary" style="margin-left:auto" @click="aboutModalOpen = false">{{ t('about.close') }}</button>
        </div>
      </div>
    </div>

    <!-- Confirm Java Delete Modal -->
    <div class="modal-root" v-if="javaToDelete">
      <div class="modal-backdrop" @click="javaToDelete = null"></div>
      <div class="modal-box">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('settings.uninstallJava') }}</h3>
          <button class="modal-close" @click="javaToDelete = null">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>{{ t('settings.uninstallJavaConfirm').replace('{major}', javaToDelete.major) }}</p>
          <p style="color: var(--muted); font-size: 12px; margin-top: 6px;">{{ javaToDelete.path }}</p>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="javaToDelete = null">{{ t('inst.cancel') }}</button>
          <button class="btn-primary btn-danger" :disabled="deletingJava" @click="confirmDeleteJava">
            {{ deletingJava ? 'Удаление…' : t('settings.uninstallJava') }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
