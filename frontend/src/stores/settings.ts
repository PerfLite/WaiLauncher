import { defineStore } from 'pinia'
import { ref } from 'vue'
import { SaveSettings, GetSettings } from 'bindings/WaiLauncher/app'
import { setLang } from '@/i18n'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Record<string, any>>({
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
  })
  const dataDir = ref('')
  const launcherVer = ref('1.2.1')
  const versions = ref<any[]>([])
  const latestRelease = ref('')
  const latestSnapshot = ref('')
  const versionsErr = ref('')

  async function save() {
    await SaveSettings(settings.value as any)
  }

  async function reload() {
    const s = await GetSettings()
    if (s) {
      settings.value = s
      setLang(settings.value.language)
    }
  }

  return {
    settings,
    dataDir,
    launcherVer,
    versions,
    latestRelease,
    latestSnapshot,
    versionsErr,
    save,
    reload,
  }
})
