<script setup lang="ts">
import {computed} from 'vue'
import {
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
} from 'reka-ui'
import {store, toast} from '@/store'
import {t} from '@/i18n'
import {DownloadLauncherUpdate, OpenURL} from 'bindings/WaiLauncher/app'
import {renderMarkdown, handleMarkdownClick} from '@/utils/markdown'
import {Button} from '@/components/ui/button'

const up = computed(() => store.launcherUpdate)
const info = computed(() => up.value.info)
const renderedNotes = computed(() => renderMarkdown(info.value?.releaseNotes || ''))

const isOpen = computed({
  get: () => up.value.modalOpen && !!info.value,
  set: (val: boolean) => {
    if (!val && !isBusy.value) {
      store.launcherUpdate.modalOpen = false
    }
  },
})

const isBusy = computed(() => up.value.downloading || up.value.restarting)

function close() {
  if (isBusy.value) return
  isOpen.value = false
}

function handleInteractOutside(e: Event) {
  if (isBusy.value) {
    e.preventDefault()
  }
}

async function install() {
  if (isBusy.value) return
  store.launcherUpdate.downloading = true
  store.launcherUpdate.percent = 0
  store.launcherUpdate.message = ''
  try {
    await DownloadLauncherUpdate()
  } catch (e: any) {
    store.launcherUpdate.downloading = false
    toast((t('inst.err') || 'Ошибка: ') + (e?.message || e), true)
  }
}

function openRelease() {
  if (info.value && info.value.releaseUrl) {
    OpenURL(info.value.releaseUrl)
  }
}

function formatDate(s?: string) {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleDateString(store.settings.language === 'en' ? 'en-GB' : 'ru-RU', {year: 'numeric', month: 'long', day: 'numeric'})
}
</script>

<template>
  <DialogRoot v-model:open="isOpen">
    <DialogPortal>
      <div class="modal-root" v-if="info">
        <DialogOverlay class="modal-backdrop" />
        <DialogContent
          class="modal-box update-modal outline-none"
          @pointer-down-outside="handleInteractOutside"
          @escape-key-down="handleInteractOutside"
        >
          <div class="modal-header">
            <h3 class="modal-title">{{ t('update.title') }}</h3>
            <button class="modal-close" @click="close" :disabled="isBusy">
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

            <div
              class="update-notes md-body"
              v-if="info.releaseNotes"
              v-html="renderedNotes"
              @click="handleMarkdownClick"
            ></div>

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
            <Button variant="secondary" @click="close" :disabled="isBusy">{{ t('inst.cancel') }}</Button>
            <Button variant="secondary" @click="openRelease" :disabled="isBusy" v-if="info.releaseUrl">
              {{ t('update.releasePage') }}
            </Button>
            <Button variant="default" @click="install" :disabled="isBusy">
              <span v-if="up.downloading">{{ t('update.installing') }}</span>
              <span v-else-if="up.restarting">{{ t('update.restarting') }}</span>
              <span v-else>{{ t('update.install') }}</span>
            </Button>
          </div>
        </DialogContent>
      </div>
    </DialogPortal>
  </DialogRoot>
</template>
