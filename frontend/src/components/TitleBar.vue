<script setup>
import {computed} from 'vue'
import {store} from '../store'
import {t} from '../i18n'
import {WindowMinimize, WindowToggleMaximize, WindowClose} from '../../wailsjs/go/main/App'
import logoImg from '../assets/logo.png'

const hasUpdate = computed(() => {
  const u = store.launcherUpdate
  return !!(u.info && u.info.updateAvailable && !u.restarting && !u.downloading)
})

function openUpdateModal() {
  if (hasUpdate.value) store.launcherUpdate.modalOpen = true
}
async function toggleMax() {
  try {
    store.maximized = await WindowToggleMaximize()
  } catch (e) {
    store.maximized = !store.maximized
  }
}
function minimize() { WindowMinimize().catch(() => {}) }
function close() { WindowClose().catch(() => {}) }
</script>

<template>
  <header class="titlebar">
    <div class="logo">
      <img :src="logoImg" alt="WaiLauncher Logo" class="logo-icon-img">
      <span class="logo-text">AI<b>LAUNCHER</b></span>
    </div>
    <button
      class="titlebar-about-btn"
      :title="t('about.title') || 'О программе WaiLauncher'"
      @click="store.aboutModalOpen = true"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="16" x2="12" y2="12"/>
        <line x1="12" y1="8" x2="12.01" y2="8"/>
      </svg>
    </button>
    <div class="win-controls">
      <button
        v-if="hasUpdate"
        class="win-btn update"
        :title="t('update.tbTitle')"
        @click="openUpdateModal"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="9"/>
          <path d="M12 16V8"/>
          <path d="m8.5 11.5 3.5-3.5 3.5 3.5"/>
        </svg>
      </button>
      <button class="win-btn" :title="t('tb.min')" @click="minimize">—</button>
      <button class="win-btn" :title="t('tb.max')" @click="toggleMax">▢</button>
      <button class="win-btn close" :title="t('tb.close')" @click="close">✕</button>
    </div>
  </header>
</template>
