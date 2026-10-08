import {reactive} from 'vue'
import {GetNews, GetAccounts, SelectAccount, RemoveAccount} from 'bindings/WaiLauncher/app'
import type { Account } from 'bindings/WaiLauncher/internal/launcher/auth/models'
import type { Instance } from 'bindings/WaiLauncher/models'
import skinImg from './assets/skin.png'

export interface ToastItem {
  id: number
  text: string
  gold: boolean
  out: boolean
}

export interface StoreState {
  page: string
  maximized: boolean
  ready: boolean
  launcherVer: string
  dataDir: string
  versionsErr: string
  settings: Record<string, any>
  accounts: Account[]
  activeAccountId: string
  accountsModalOpen: boolean
  versions: any[]
  latestRelease: string
  latestSnapshot: string
  launch: {
    state: string
    stage: string
    percent: number
    message: string
  }
  toasts: ToastItem[]
  news: any[]
  newsLoaded: boolean
  activeArticleUrl: string
  instances: Instance[]
  selectedInstanceId: string
  createInstanceModalOpen: boolean
  aboutModalOpen: boolean
  mods: Record<string, boolean>
  launcherUpdate: {
    info: any
    modalOpen: boolean
    downloading: boolean
    percent: number
    message: string
    restarting: boolean
    error: string
  }
  modpackUpdates: Record<string, any>
  gameLog: {
    running: boolean
    lines: string[]
  }
}

let toastId = 0

export const store: StoreState = reactive({
  page: 'home',
  maximized: false,
  ready: false,
  launcherVer: '1.1.3',
  dataDir: '',
  versionsErr: '',
  settings: {
    username: 'Player',
    ramMb: 4096,
    resolution: '1920 × 1080',
    javaPath: '',
    jvmPreset: 'aikar',
    extraJvmArgs: '',
    closeOnLaunch: false,
    showSnapshots: false,
    language: 'ru',
    discordRpc: true,
    autoUpdate: true,
    launcherUpdates: true,
    selectedVersion: '',
  },
  accounts: [],
  activeAccountId: '',
  accountsModalOpen: false,
  versions: [],
  latestRelease: '',
  latestSnapshot: '',
  launch: {state: 'idle', stage: '', percent: 0, message: ''},
  toasts: [],
  news: [],
  newsLoaded: false,
  activeArticleUrl: '',
  instances: [], // user-created builds (Modrinth-style profiles)
  selectedInstanceId: '',
  createInstanceModalOpen: false,
  aboutModalOpen: false,
  mods: {}, // name -> installed
  launcherUpdate: {info: null, modalOpen: false, downloading: false, percent: 0, message: '', restarting: false, error: ''},
  modpackUpdates: {}, // instanceId -> ModpackUpdateInfo
  gameLog: {running: false, lines: []},
})

export function activeAccount() {
  if (!store.accounts || store.accounts.length === 0) {
    return {
      id: 'default',
      username: store.settings.username || 'Player',
      type: 'offline',
      avatarUrl: `https://mc-heads.net/avatar/${store.settings.username || 'Player'}/64`,
    }
  }
  const found = store.accounts.find(a => a.id === store.activeAccountId)
  return found || store.accounts[0]
}

export function getAccountAvatar(acc?: Account | any) {
  if (!acc) return skinImg
  if (acc.avatarUrl) return acc.avatarUrl
  if (acc.uuid) return `https://mc-heads.net/avatar/${acc.uuid}/64`
  if (acc.username) return `https://mc-heads.net/avatar/${acc.username}/64`
  return skinImg
}

export async function reloadAccounts() {
  try {
    const data = await GetAccounts()
    if (data) {
      store.accounts = (data.accounts || []) as Account[]
      store.activeAccountId = data.activeId || (store.accounts[0] ? store.accounts[0].id : '')
      const act = activeAccount()
      if (act && act.username) {
        store.settings.username = act.username
      }
    }
  } catch (e) {
    console.error('Failed to load accounts:', e)
  }
}

export async function switchActiveAccount(id: string) {
  try {
    const acc = await SelectAccount(id)
    if (acc) {
      store.activeAccountId = acc.id
      store.settings.username = acc.username
      await reloadAccounts()
      return acc
    }
  } catch (e) {
    throw e
  }
}

export async function deleteAccount(id: string) {
  try {
    await RemoveAccount(id)
    await reloadAccounts()
  } catch (e) {
    throw e
  }
}

export function toast(text: string, gold = false) {
  const id = ++toastId
  store.toasts.push({id, text, gold, out: false})
  setTimeout(() => {
    const t = store.toasts.find(t => t.id === id)
    if (t) t.out = true
  }, 2800)
  setTimeout(() => {
    const i = store.toasts.findIndex(t => t.id === id)
    if (i !== -1) store.toasts.splice(i, 1)
  }, 3300)
}

export function installedModsCount() {
  return Object.values(store.mods).filter(Boolean).length
}

// reloadNews re-fetches news so backend-localized fields (tags, dates)
// match the current UI language.
export function reloadNews() {
  GetNews().then(n => { store.news = n || [] }).catch(() => {})
}
