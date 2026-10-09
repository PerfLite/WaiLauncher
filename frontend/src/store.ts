import { reactive } from 'vue'
import type { Account } from 'bindings/WaiLauncher/internal/launcher/auth/models'
import type { Instance } from 'bindings/WaiLauncher/models'
import {
  useUiStore,
  useAccountStore,
  useInstanceStore,
  useSettingsStore,
  useLaunchStore,
  useNewsStore,
  useUpdateStore,
} from './stores/index'

export type { ToastItem } from './stores/ui'

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
  toasts: any[]
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

export const store: StoreState = reactive({
  get page() { return useUiStore().page },
  set page(v: string) { useUiStore().page = v },
  get maximized() { return useUiStore().maximized },
  set maximized(v: boolean) { useUiStore().maximized = v },
  get ready() { return useUiStore().ready },
  set ready(v: boolean) { useUiStore().ready = v },
  get aboutModalOpen() { return useUiStore().aboutModalOpen },
  set aboutModalOpen(v: boolean) { useUiStore().aboutModalOpen = v },
  get createInstanceModalOpen() { return useUiStore().createInstanceModalOpen },
  set createInstanceModalOpen(v: boolean) { useUiStore().createInstanceModalOpen = v },
  get accountsModalOpen() { return useUiStore().accountsModalOpen },
  set accountsModalOpen(v: boolean) { useUiStore().accountsModalOpen = v },
  get toasts() { return useUiStore().toasts },
  set toasts(v: any[]) { useUiStore().toasts = v },

  get accounts() { return useAccountStore().accounts },
  set accounts(v: Account[]) { useAccountStore().accounts = v },
  get activeAccountId() { return useAccountStore().activeAccountId },
  set activeAccountId(v: string) { useAccountStore().activeAccountId = v },

  get instances() { return useInstanceStore().instances },
  set instances(v: Instance[]) { useInstanceStore().instances = v },
  get selectedInstanceId() { return useInstanceStore().selectedInstanceId },
  set selectedInstanceId(v: string) { useInstanceStore().selectedInstanceId = v },
  get modpackUpdates() { return useInstanceStore().modpackUpdates },
  set modpackUpdates(v: Record<string, any>) { useInstanceStore().modpackUpdates = v },

  get settings() { return useSettingsStore().settings },
  set settings(v: Record<string, any>) { useSettingsStore().settings = v },
  get dataDir() { return useSettingsStore().dataDir },
  set dataDir(v: string) { useSettingsStore().dataDir = v },
  get launcherVer() { return useSettingsStore().launcherVer },
  set launcherVer(v: string) { useSettingsStore().launcherVer = v },
  get versions() { return useSettingsStore().versions },
  set versions(v: any[]) { useSettingsStore().versions = v },
  get latestRelease() { return useSettingsStore().latestRelease },
  set latestRelease(v: string) { useSettingsStore().latestRelease = v },
  get latestSnapshot() { return useSettingsStore().latestSnapshot },
  set latestSnapshot(v: string) { useSettingsStore().latestSnapshot = v },
  get versionsErr() { return useSettingsStore().versionsErr },
  set versionsErr(v: string) { useSettingsStore().versionsErr = v },

  get launch() { return useLaunchStore().launch },
  set launch(v: any) { useLaunchStore().launch = v },
  get gameLog() { return useLaunchStore().gameLog },
  set gameLog(v: any) { useLaunchStore().gameLog = v },

  get news() { return useNewsStore().news },
  set news(v: any[]) { useNewsStore().news = v },
  get newsLoaded() { return useNewsStore().newsLoaded },
  set newsLoaded(v: boolean) { useNewsStore().newsLoaded = v },
  get activeArticleUrl() { return useNewsStore().activeArticleUrl },
  set activeArticleUrl(v: string) { useNewsStore().activeArticleUrl = v },

  get launcherUpdate() { return useUpdateStore().launcherUpdate },
  set launcherUpdate(v: any) { useUpdateStore().launcherUpdate = v },

  mods: {},
})

export function activeAccount() {
  return useAccountStore().activeAccount
}

export function getAccountAvatar(acc?: any) {
  return useAccountStore().getAvatar(acc)
}

export async function reloadAccounts() {
  return await useAccountStore().reloadAccounts()
}

export async function switchActiveAccount(id: string) {
  return await useAccountStore().switchActiveAccount(id)
}

export async function deleteAccount(id: string) {
  return await useAccountStore().deleteAccount(id)
}

export function toast(text: string, gold = false) {
  useUiStore().addToast(text, gold)
}

export function installedModsCount() {
  return Object.values(store.mods).filter(Boolean).length
}

export function reloadNews() {
  useNewsStore().reloadNews()
}
