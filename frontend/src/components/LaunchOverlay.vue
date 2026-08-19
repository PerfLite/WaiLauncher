<script setup>
import {ref, watch} from 'vue'
import {store} from '../store'
import {t} from '../i18n'

const visible = ref(false)
const pct = ref(0)
const status = ref(t('overlay.preparing'))

watch(() => store.launch, (ev) => {
  if (ev.state === 'working' && ev.stage === 'start') {
    visible.value = true
    pct.value = 40
    status.value = t('overlay.jvm')
    requestAnimationFrame(() => { pct.value = 85 })
  } else if (ev.state === 'playing') {
    pct.value = 100
    status.value = t('overlay.started')
    setTimeout(() => { visible.value = false }, 800)
  } else if (ev.state === 'idle') {
    visible.value = false
  }
})
</script>

<template>
  <div class="launch-overlay" :class="{show: visible}">
    <svg class="launch-logo" viewBox="0 0 8 8" shape-rendering="crispEdges">
      <rect width="8" height="8" fill="#55d24a"/>
      <rect x="1" y="2" width="2" height="2" fill="#0a160b"/>
      <rect x="5" y="2" width="2" height="2" fill="#0a160b"/>
      <rect x="3" y="4" width="2" height="2" fill="#0a160b"/>
      <rect x="2" y="5" width="2" height="3" fill="#0a160b"/>
      <rect x="4" y="5" width="2" height="3" fill="#0a160b"/>
    </svg>
    <div class="launch-title">MINECRAFT</div>
    <div class="launch-bar"><i :style="{width: pct + '%'}"></i></div>
    <div class="launch-status">{{ status }}</div>
  </div>
</template>
