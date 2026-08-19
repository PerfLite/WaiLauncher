<script setup>
import {store} from '../store'
import {t} from '../i18n'
import {WindowMinimize, WindowToggleMaximize, WindowClose} from '../../wailsjs/go/main/App'
import logoImg from '../assets/logo.png'

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
    <div class="win-controls">
      <button class="win-btn" :title="t('tb.min')" @click="minimize">—</button>
      <button class="win-btn" :title="t('tb.max')" @click="toggleMax">▢</button>
      <button class="win-btn close" :title="t('tb.close')" @click="close">✕</button>
    </div>
  </header>
</template>
