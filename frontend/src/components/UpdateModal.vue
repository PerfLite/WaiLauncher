<script setup>
import {computed} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {DownloadLauncherUpdate, OpenURL} from '../../wailsjs/go/main/App'

const up = computed(() => store.launcherUpdate)
const info = computed(() => up.value.info)

function close() {
  if (up.value.downloading || up.value.restarting) return
  store.launcherUpdate.modalOpen = false
}

async function install() {
  if (up.value.downloading || up.value.restarting) return
  store.launcherUpdate.downloading = true
  store.launcherUpdate.percent = 0
  store.launcherUpdate.message = ''
  try {
    await DownloadLauncherUpdate()
  } catch (e) {
    store.launcherUpdate.downloading = false
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

function openRelease() {
  if (info.value && info.value.releaseUrl) {
    OpenURL(info.value.releaseUrl)
  }
}

function formatDate(s) {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleDateString(store.settings.language === 'en' ? 'en-GB' : 'ru-RU', {year: 'numeric', month: 'long', day: 'numeric'})
}
</script>

<template>
  <div class="modal-root" v-if="up.modalOpen && info">
    <div class="modal-backdrop" @click="close"></div>
    <div class="modal-box update-modal">
      <div class="modal-header">
        <h3 class="modal-title">{{ t('update.title') }}</h3>
        <button class="modal-close" @click="close" :disabled="up.downloading || up.restarting">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
        </button>
      </div>

      <div class="modal-body update-body">
        <div class="update-versions">
          <div class="update-ver-card">
            <span class="update-ver-label">{{ t('update.current') }}</span>
            <span class="update-ver-value">v{{ info.currentVersion }}</span>
          </div>
          <svg class="update-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14M13 6l6 6-6 6"/></svg>
          <div class="update-ver-card new">
            <span class="update-ver-label">{{ t('update.latest') }}</span>
            <span class="update-ver-value">v{{ info.latestVersion }}</span>
            <span class="update-date" v-if="info.publishedAt">{{ formatDate(info.publishedAt) }}</span>
          </div>
        </div>

        <div class="update-notes" v-if="info.releaseNotes">
          <pre>{{ info.releaseNotes }}</pre>
        </div>

        <div v-if="up.downloading || up.restarting" class="update-progress">
          <div class="java-progress-bar">
            <i :style="{width: Math.max(2, up.percent) + '%'}"></i>
          </div>
          <div class="java-progress-text">
            <span>{{ up.restarting ? t('update.restarting') : (up.message || t('update.downloading')) }}</span>
            <span class="java-pct-val">{{ Math.floor(up.percent) }}%</span>
          </div>
        </div>
        <p v-else-if="up.error" class="update-error">{{ up.error }}</p>
      </div>

      <div class="modal-foot">
        <button class="btn-sec" @click="close" :disabled="up.downloading || up.restarting">{{ t('inst.cancel') }}</button>
        <button class="btn-sec" @click="openRelease" :disabled="up.downloading || up.restarting" v-if="info.releaseUrl">
          {{ t('update.releasePage') }}
        </button>
        <button class="btn-primary" @click="install" :disabled="up.downloading || up.restarting">
          <span v-if="up.downloading">{{ t('update.installing') }}</span>
          <span v-else-if="up.restarting">{{ t('update.restarting') }}</span>
          <span v-else>{{ t('update.install') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
