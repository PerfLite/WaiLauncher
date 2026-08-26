<script setup>
import {onMounted, watch} from 'vue'
import {store, toast, reloadAccounts} from './store'
import {t, setLang} from './i18n'
import TitleBar from './components/TitleBar.vue'
import SideBar from './components/SideBar.vue'
import Toasts from './components/Toasts.vue'
import LaunchOverlay from './components/LaunchOverlay.vue'
import UpdateModal from './components/UpdateModal.vue'
import CreateInstanceModal from './components/CreateInstanceModal.vue'
import HomePage from './pages/HomePage.vue'
import InstancesPage from './pages/InstancesPage.vue'
import NewsPage from './pages/NewsPage.vue'
import ModsPage from './pages/ModsPage.vue'
import SettingsPage from './pages/SettingsPage.vue'
import AccountsPage from './pages/AccountsPage.vue'
import {GetState, RefreshVersions, GetNews, CheckLauncherUpdate} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'

function applyState(st) {
  if (!st) return
  if (st.settings) store.settings = st.settings
  setLang(store.settings.language)
  store.versions = st.versions || []
  store.latestRelease = st.latestRelease || ''
  store.latestSnapshot = st.latestSnapshot || ''
  store.launcherVer = st.launcherVer || store.launcherVer
  store.dataDir = st.dataDir || ''
  store.versionsErr = st.versionsErr || ''
  if (st.accounts) store.accounts = st.accounts
  if (st.activeId) store.activeAccountId = st.activeId
  store.instances = st.instances || []
  if (!store.settings.selectedVersion) {
    store.settings.selectedVersion = store.latestRelease
  }
}

onMounted(async () => {
  try {
    const st = await GetState()
    applyState(st)
    store.ready = true
    if (store.versionsErr) toast(t('app.versionsErr') + store.versionsErr, true)
    if (store.settings.autoUpdate) {
      RefreshVersions().then(applyState).catch(() => {})
    }
  } catch (e) {
    // Plain-browser preview (no wails backend): demo data
    store.ready = true
    store.versions = [
      {id: '1.21.4', type: 'release', installed: false},
      {id: '25w07a', type: 'snapshot', installed: false},
      {id: '1.20.6', type: 'release', installed: true},
    ]
    store.latestRelease = '1.21.4'
    store.settings.selectedVersion = '1.21.4'
    store.accounts = [
      {id: 'demo-1', type: 'offline', username: 'Player', uuid: '00000000-0000-0000-0000-000000000000'},
    ]
    store.activeAccountId = 'demo-1'
  }
  GetNews().then(n => {
    store.news = n || []
    store.newsLoaded = true
  }).catch(() => {
    store.newsLoaded = true
  })
  EventsOn('launch', (ev) => {
    const prev = store.launch.state
    store.launch = ev
    if (ev.state === 'playing' && prev !== 'playing') {
      toast(t('app.gameStarted'))
    } else if (ev.state === 'idle' && prev === 'playing') {
      toast(ev.message || t('app.gameClosed'))
    } else if (ev.state === 'idle' && prev === 'working' && ev.message && ev.message !== 'Отменено' && ev.message !== 'Cancelled') {
      toast(ev.message, true)
    }
  })

  EventsOn('update-progress', (ev) => {
    if (!ev) return
    store.launcherUpdate.percent = ev.percent || 0
    store.launcherUpdate.message = ev.message || ''
  })
  EventsOn('update-error', (msg) => {
    store.launcherUpdate.downloading = false
    store.launcherUpdate.error = msg || 'update error'
    toast((t('update.checkErr') || 'Update error: ') + msg, true)
  })
  EventsOn('update-done', () => {
    store.launcherUpdate.downloading = false
    store.launcherUpdate.restarting = true
  })

  // Self-update check (like gitdesktop): query GitHub Releases on start.
  // GitHub can return 404 for a fresh release for up to ~60s (the
  // /releases/latest endpoint lags), so retry a few times before giving up.
  (async function checkUpdates() {
    const delays = [3000, 15000, 40000, 90000]
    for (const delay of delays) {
      await new Promise(r => setTimeout(r, delay))
      try {
        const info = await CheckLauncherUpdate()
        if (info && info.updateAvailable) {
          store.launcherUpdate.info = info
          store.launcherUpdate.error = ''
          if (store.settings.launcherUpdates) {
            store.launcherUpdate.modalOpen = true
          }
          return
        }
        if (info && !info.error) return // check succeeded, no newer release
      } catch (e) { /* offline or GitHub not reachable yet — retry */ }
    }
  })()

  EventsOn('instances-updated', (list) => {
    if (Array.isArray(list)) {
      store.instances = list
    }
  })
})

watch(() => store.maximized, (v) => {
  document.body.classList.toggle('maximized', v)
})
watch(() => store.settings.language, (v) => {
  document.documentElement.lang = v === 'en' ? 'en' : 'ru'
})
watch(() => store.page, () => {
  const el = document.getElementById('mainScroll')
  if (el) el.scrollTop = 0
})
</script>

<template>
  <div class="app">
    <TitleBar/>
    <SideBar/>
    <main class="main" id="mainScroll">
      <HomePage :class="{active: store.page === 'home'}"/>
      <InstancesPage :class="{active: store.page === 'instances'}"/>
      <ModsPage :class="{active: store.page === 'mods'}"/>
      <NewsPage :class="{active: store.page === 'news'}"/>
      <SettingsPage :class="{active: store.page === 'settings'}"/>
      <AccountsPage :class="{active: store.page === 'accounts'}"/>
    </main>
  </div>
  <CreateInstanceModal/>
  <LaunchOverlay/>
  <UpdateModal/>
  <Toasts/>
</template>
