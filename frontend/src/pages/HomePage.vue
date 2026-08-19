<script setup>
import {computed, onMounted, ref} from 'vue'
import {store, toast} from '../store'
import {
  Play, CancelPlay, StopGame,
  SetActiveInstance, DeleteInstance,
  OpenInstanceDir,
} from '../../wailsjs/go/main/App'
import {t} from '../i18n'
import heroBg from '../assets/hero-bg.png'

const stageNames = computed(() => ({
  manifest: t('stage.manifest'),
  client: t('stage.client'),
  libraries: t('stage.libraries'),
  loader: t('stage.loader'),
  java: t('stage.java'),
  natives: t('stage.natives'),
  assets: t('stage.assets'),
  start: t('stage.start'),
}))
const typeNames = computed(() => ({
  release: t('type.release'),
  snapshot: t('type.snapshot'),
  old_beta: t('type.old_beta'),
  old_alpha: t('type.old_alpha'),
}))

/* ---- builds (instances) ---- */
const hasInstances = computed(() => store.instances.length > 0)
const activeInst = computed(() =>
  store.instances.find(i => i.id === store.settings.activeInstance) || store.instances[0] || null
)

const ddOpen = ref(false)

const launch = computed(() => store.launch)
const isWorking = computed(() => launch.value.state === 'working')
const isPlaying = computed(() => launch.value.state === 'playing')

const playLabel = computed(() => {
  if (isPlaying.value) return t('home.running')
  if (isWorking.value) {
    const p = Math.floor(launch.value.percent || 0)
    return p > 0 ? p + '%' : (stageNames.value[launch.value.stage] || t('home.loading'))
  }
  return hasInstances.value ? t('home.play') : t('home.create')
})
const playFill = computed(() => isWorking.value ? (launch.value.percent || 0) + '%' : '0%')
const playClass = computed(() => ({downloading: isWorking.value, playing: isPlaying.value}))

const heroStatus = computed(() => {
  if (isWorking.value) {
    const name = stageNames.value[launch.value.stage] || t('home.loading')
    const p = Math.floor(launch.value.percent || 0)
    const msg = launch.value.message ? ' • ' + launch.value.message : ''
    return `${name}… ${p > 0 ? p + '%' : ''}${msg}`.trim()
  }
  if (isPlaying.value) return t('home.playing') + ' • ' + (activeInst.value ? activeInst.value.name : '')
  const base = t('home.ready')
  return activeInst.value ? base + ' • ' + activeInst.value.name : base
})

async function onPlay() {
  if (isWorking.value) {
    CancelPlay().catch(() => {})
    return
  }
  if (isPlaying.value) {
    StopGame().catch(() => {})
    return
  }
  if (!activeInst.value) {
    store.createInstanceModalOpen = true
    return
  }
  try {
    await Play(activeInst.value.id)
  } catch (e) {
    toast(t('home.launchErr') + e, true)
  }
}

async function chooseInstance(inst) {
  ddOpen.value = false
  if (inst.id === store.settings.activeInstance) return
  try {
    await SetActiveInstance(inst.id)
    store.settings.activeInstance = inst.id
    store.settings.selectedVersion = inst.versionId
  } catch (e) {
    toast(t('inst.err') + e, true)
  }
}

/* instance context menu */
const ctxMenuVisible = ref(false)
const ctxMenuPos = ref({x: 0, y: 0})
const ctxMenuInst = ref(null)

function onInstContextMenu(e, inst) {
  e.preventDefault()
  ctxMenuInst.value = inst
  const menuWidth = 220
  const menuHeight = 180
  let x = e.clientX
  let y = e.clientY
  if (x + menuWidth > window.innerWidth) {
    x = Math.max(10, window.innerWidth - menuWidth - 10)
  }
  if (y + menuHeight > window.innerHeight) {
    y = Math.max(10, y - menuHeight)
  }
  ctxMenuPos.value = {x, y}
  ctxMenuVisible.value = true
}

function closeContextMenu() {
  ctxMenuVisible.value = false
}

async function ctxPlay() {
  const inst = ctxMenuInst.value
  closeContextMenu()
  if (inst) {
    await chooseInstance(inst)
    onPlay()
  }
}

function ctxProfile() {
  const inst = ctxMenuInst.value
  closeContextMenu()
  if (inst) {
    chooseInstance(inst)
    store.page = 'instances'
  }
}

async function ctxOpenFolder() {
  const inst = ctxMenuInst.value
  closeContextMenu()
  if (inst) {
    try {
      await OpenInstanceDir(inst.id)
    } catch (e) {
      toast(t('inst.err') + e, true)
    }
  }
}

function ctxDelete() {
  const inst = ctxMenuInst.value
  closeContextMenu()
  if (inst) {
    promptDeleteInst(inst)
  }
}

/* instance delete modal */
const instToDelete = ref(null)

function promptDeleteInst(inst) {
  instToDelete.value = inst
}

async function confirmDeleteInst() {
  if (!instToDelete.value) return
  const inst = instToDelete.value
  instToDelete.value = null
  try {
    const newActive = await DeleteInstance(inst.id)
    store.instances = store.instances.filter(i => i.id !== inst.id)
    store.settings.activeInstance = newActive || ''
    toast(t('inst.deleted'))
  } catch (e) {
    toast(t('inst.err') + e, true)
  }
}

function openCreate() {
  store.createInstanceModalOpen = true
}

const ramChip = computed(() => Math.round(store.settings.ramMb / 1024) + ' ' + t('home.ram'))

/* частицы в hero */
const particlesBox = ref(null)
onMounted(() => {
  const colors = ['#55d24a', '#8aff7a', '#ffc94a', '#5db4ff', '#ffffff']
  for (let i = 0; i < 26; i++) {
    const s = document.createElement('span')
    const size = 3 + Math.floor(Math.random() * 5)
    s.style.width = size + 'px'
    s.style.height = size + 'px'
    s.style.left = (Math.random() * 100) + '%'
    s.style.background = colors[Math.floor(Math.random() * colors.length)]
    s.style.animationDuration = (6 + Math.random() * 9) + 's'
    s.style.animationDelay = (-Math.random() * 12) + 's'
    s.style.opacity = '0'
    particlesBox.value.appendChild(s)
  }
})
</script>

<template>
  <section class="page">
    <div class="home-grid">
      <div class="hero">
        <div class="hero-bg" :style="{backgroundImage: `url(${heroBg})`}"></div>
        <div class="hero-overlay"></div>
        <div class="particles" ref="particlesBox"></div>
        <div class="hero-content">
          <p class="hero-sub">{{ t('home.heroSub') }}</p>
          <div class="hero-actions">
            <button
              class="play-btn"
              :class="playClass"
              @click="onPlay"
              :title="isWorking ? t('home.cancel') : isPlaying ? t('home.stopHint') : ''"
            >
              <span class="play-fill" :style="{width: playFill}"></span>
              <svg v-if="!isWorking && !isPlaying && !hasInstances" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
              <svg v-else-if="!isWorking && !isPlaying" viewBox="0 0 24 24" fill="currentColor"><path d="M7 4.5v15l13-7.5-13-7.5z"/></svg>
              <svg v-else-if="isWorking" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
              <span v-else-if="isPlaying" class="play-pulse-dot"></span>
              <svg v-if="isPlaying" class="stop-icon" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
              <span class="play-label">
                <template v-if="isPlaying">
                  <span class="label-running">{{ t('home.running') }}</span>
                  <span class="label-stop">{{ t('home.stop') }}</span>
                </template>
                <template v-else>{{ playLabel }}</template>
              </span>
            </button>
            <div v-if="hasInstances" class="dropdown" :class="{open: ddOpen}">
              <button class="dd-btn" @click="ddOpen = !ddOpen">
                <div v-if="activeInst" class="hero-dd-icon" :class="['loader-' + (activeInst.loader || 'vanilla')]">
                  <img v-if="activeInst.icon" :src="activeInst.icon" alt="" class="hero-dd-icon-img">
                  <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
                  </svg>
                </div>
                <span>{{ activeInst ? activeInst.name : '—' }}</span>
                <em v-if="activeInst" class="dd-tag rel">{{ activeInst.versionId }}</em>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="m6 9 6 6 6-6"/></svg>
              </button>
              <div v-if="ddOpen" class="dd-backdrop" @click="ddOpen = false"></div>
              <ul v-if="ddOpen" class="dd-list">
                <li
                  v-for="inst in store.instances"
                  :key="inst.id"
                  :class="{sel: inst.id === (activeInst && activeInst.id)}"
                  @click="chooseInstance(inst)"
                >
                  <div class="hero-dd-icon" :class="['loader-' + (inst.loader || 'vanilla')]">
                    <img v-if="inst.icon" :src="inst.icon" alt="" class="hero-dd-icon-img">
                    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
                    </svg>
                  </div>
                  <span class="hero-dd-name">{{ inst.name }}</span>
                  <em class="dd-tag rel">{{ inst.versionId }}</em>
                  <em class="dd-tag">{{ t('loader.' + inst.loader) }}</em>
                </li>
              </ul>
            </div>
            <button class="icon-btn" :title="t('inst.createTitle')" @click="openCreate">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
            </button>
          </div>
          <div class="hero-bottom-bar">
            <div class="hero-status">
              <span class="bar-mini" v-show="isWorking"><i></i></span>
              <span>{{ heroStatus }}</span>
            </div>
            <span class="chip gold hero-ram-badge">{{ ramChip }}</span>
          </div>
        </div>
      </div>

    </div>

    <div class="section-head">
      <h2>{{ t('inst.title') }}</h2>
    </div>
    <div class="inst-grid" v-if="hasInstances">
      <div
        v-for="inst in store.instances"
        :key="inst.id"
        class="inst-card"
        :class="{active: activeInst && inst.id === activeInst.id}"
        @click="chooseInstance(inst)"
        @dblclick="store.page = 'instances'"
        @contextmenu.prevent="onInstContextMenu($event, inst)"
      >
        <div class="inst-icon" :class="['loader-' + (inst.loader || 'vanilla')]">
          <img v-if="inst.icon" :src="inst.icon" alt="" class="inst-icon-img">
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
          </svg>
        </div>
        <div class="inst-meta">
          <div class="inst-name">{{ inst.name }}</div>
          <div class="inst-sub">{{ inst.versionId }} • {{ t('loader.' + inst.loader) }}{{ inst.loaderVersion ? ' ' + inst.loaderVersion : '' }}</div>
        </div>
        <span v-if="activeInst && inst.id === activeInst.id" class="inst-active">{{ t('inst.active') }}</span>
        <button
          class="inst-del"
          :title="t('accounts.delete')"
          @click.stop="promptDeleteInst(inst)"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18 M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6 M10 11v6 M14 11v6"/></svg>
        </button>
      </div>
    </div>
    <div v-else class="news-empty">
      <p>{{ t('inst.empty') }}</p>
      <button class="btn-primary" style="margin-top: 10px;" @click="store.page = 'instances'; store.createInstanceModalOpen = true">+ {{ t('inst.createBtn') }}</button>
    </div>

    <!-- Instance Right Click Context Menu -->
    <div
      v-if="ctxMenuVisible"
      class="ctx-backdrop"
      @click="closeContextMenu"
      @contextmenu.prevent="closeContextMenu"
    >
      <div
        class="ctx-menu"
        :style="{top: ctxMenuPos.y + 'px', left: ctxMenuPos.x + 'px'}"
        @click.stop
      >
        <div class="ctx-header">{{ ctxMenuInst ? ctxMenuInst.name : '' }}</div>
        <button class="ctx-item" @click="ctxPlay">
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
          <span>{{ t('ctx.play') }}</span>
        </button>
        <button class="ctx-item" @click="ctxProfile">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/></svg>
          <span>{{ t('ctx.profile') }}</span>
        </button>
        <button class="ctx-item" @click="ctxOpenFolder">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
          <span>{{ t('ctx.openFolder') }}</span>
        </button>
        <div class="ctx-divider"></div>
        <button class="ctx-item danger" @click="ctxDelete">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18 M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6 M10 11v6 M14 11v6"/></svg>
          <span>{{ t('ctx.delete') }}</span>
        </button>
      </div>
    </div>

    <!-- Custom In-App Instance Delete Confirmation Modal -->
    <div v-if="instToDelete" class="confirm-modal-backdrop" @click="instToDelete = null">
      <div class="confirm-modal-box" @click.stop>
        <div class="confirm-icon-wrap">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><path d="M10 11v6M14 11v6"/></svg>
        </div>
        <h3 class="confirm-title">{{ t('inst.deleteTitle') }}</h3>
        <p class="confirm-text">{{ t('inst.deleteConfirmText', {name: instToDelete.name}) }}</p>
        <div class="confirm-actions">
          <button class="btn-sec" @click="instToDelete = null">{{ t('inst.deleteCancel') }}</button>
          <button class="btn-danger" @click="confirmDeleteInst">{{ t('inst.deleteConfirm') }}</button>
        </div>
      </div>
    </div>
  </section>
</template>
