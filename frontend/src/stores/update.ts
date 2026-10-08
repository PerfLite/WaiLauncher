import { defineStore } from 'pinia'
import { ref } from 'vue'
import { CheckLauncherUpdate, DownloadLauncherUpdate } from 'bindings/WaiLauncher/app'

export interface LauncherUpdateState {
  info: any
  modalOpen: boolean
  downloading: boolean
  percent: number
  message: string
  restarting: boolean
  error: string
}

export const useUpdateStore = defineStore('update', () => {
  const launcherUpdate = ref<LauncherUpdateState>({
    info: null,
    modalOpen: false,
    downloading: false,
    percent: 0,
    message: '',
    restarting: false,
    error: '',
  })

  async function check() {
    try {
      const info = await CheckLauncherUpdate()
      if (info && info.updateAvailable) {
        launcherUpdate.value.info = info
        launcherUpdate.value.error = ''
        return info
      }
    } catch (_) {}
    return null
  }

  async function download() {
    launcherUpdate.value.downloading = true
    launcherUpdate.value.percent = 0
    launcherUpdate.value.message = ''
    try {
      await DownloadLauncherUpdate()
    } catch (e: any) {
      launcherUpdate.value.downloading = false
      launcherUpdate.value.error = e?.message || String(e)
      throw e
    }
  }

  return {
    launcherUpdate,
    check,
    download,
  }
})
