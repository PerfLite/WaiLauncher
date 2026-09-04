<script setup>
import {onMounted, onUnmounted} from 'vue'
import {store} from '../store'
import {t} from '../i18n'
import {OpenURL} from '../../wailsjs/go/main/App'

function close() {
  store.aboutModalOpen = false
}

function openExternal(url) {
  if (url) {
    try {
      OpenURL(url)
    } catch (e) {
      window.open(url, '_blank')
    }
  }
}

function onKeydown(e) {
  if (e.key === 'Escape' && store.aboutModalOpen) {
    close()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="modal-root" v-if="store.aboutModalOpen">
    <div class="modal-backdrop" @click="close"></div>
    <div class="modal-box about-modal-box">
      <div class="modal-header">
        <h3 class="modal-title">{{ t('about.title') || 'О программе WaiLauncher' }}</h3>
        <button class="modal-close" @click="close">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
        </button>
      </div>
      <div class="modal-body about-modal-body">
        <div class="about-hero">
          <h2 class="about-app-title">Wai<span class="highlight">Launcher</span></h2>
          <p class="about-app-version">v{{ store.launcherVer || '1.1.2' }} &mdash; {{ t('about.desc') || 'Современный лаунчер Minecraft нового поколения' }}</p>
        </div>

        <!-- Links & Developer section -->
        <div class="about-card-section">
          <div class="about-sec-title">{{ t('about.developer') || 'Разработчик и проект' }}</div>
          <div class="about-links-col">
            <div class="about-link-row" @click="openExternal('https://github.com/PerfLite')">
              <svg class="about-link-icon" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/></svg>
              <span class="about-link-label">GitHub: <strong class="about-highlight">PerfLite</strong></span>
            </div>
            <div class="about-link-row" @click="openExternal('https://t.me/bashakul')">
              <svg class="about-link-icon" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221l-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.446 1.394c-.14.18-.357.295-.6.295-.002 0-.003 0-.005 0l.213-3.054 5.56-5.022c.24-.213-.054-.334-.373-.121l-6.869 4.326-2.96-.924c-.643-.204-.657-.643.136-.953l11.57-4.458c.538-.196 1.006.128.832.943z"/></svg>
              <span class="about-link-label">Telegram: <strong class="about-highlight">PerfLite</strong></span>
            </div>
            <div class="about-link-row about-link-issues" @click="openExternal('https://github.com/PerfLite/WaiLauncher/issues')">
              <div class="about-link-label-group">
                <span class="about-link-label"><strong class="about-highlight-issue">{{ t('about.issues') || 'Сообщить о баге / GitHub Issues' }}</strong></span>
                <span class="about-link-sub">github.com/PerfLite/WaiLauncher/issues</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Built with section -->
        <div class="about-card-section">
          <div class="about-sec-title">{{ t('about.builtWith') || 'Технологии' }}</div>
          <div class="about-tags-row">
            <span class="about-tag tag-go">Go 1.22+</span>
            <span class="about-tag tag-wails">Wails v2</span>
            <span class="about-tag tag-vue">Vue 3</span>
            <span class="about-tag tag-vite">Vite</span>
            <span class="about-tag tag-webview">WebView2</span>
            <span class="about-tag tag-modrinth">Modrinth API</span>
            <span class="about-tag tag-curseforge">CurseForge API</span>
            <span class="about-tag tag-ftb">FTB API</span>
          </div>
        </div>
      </div>
      <div class="modal-foot">
        <button class="btn-primary" style="margin-left:auto" @click="close">{{ t('about.close') || 'Закрыть' }}</button>
      </div>
    </div>
  </div>
</template>
