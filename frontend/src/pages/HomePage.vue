<script setup>
import {computed, onMounted, ref} from 'vue'
import {store, toast} from '../store'
import {
  Play, CancelPlay, StopGame,
  SetActiveInstance, DeleteInstance,
  OpenInstanceDir, UpdateInstanceGroup, ReorderInstances,
  CreateGroup, RenameGroup, DeleteGroup
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
const searchQuery = ref('')
const activeFilter = ref('all')
const collapsedGroups = ref({})

/* Folder Modal State */
const folderModalOpen = ref(false)
const folderModalMode = ref('create') // 'create' | 'rename'
const folderModalName = ref('')
const folderModalOldName = ref('')
const folderModalSelectedInsts = ref([])
const folderToDelete = ref(null)

/* Drag and drop state */
const draggedInst = ref(null)
const dragOverCardId = ref(null)
const dragOverGroupName = ref(null)

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

/* Full-text search and filtering */
const filteredInstances = computed(() => {
  let list = store.instances || []
  if (activeFilter.value !== 'all') {
    list = list.filter(i => (i.loader || 'vanilla') === activeFilter.value)
  }
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(i => {
      const name = (i.name || '').toLowerCase()
      const ver = (i.versionId || '').toLowerCase()
      const ldr = (i.loader || '').toLowerCase()
      const ldrVer = (i.loaderVersion || '').toLowerCase()
      const grp = (i.group || '').toLowerCase()
      const srv = (i.serverAddress || '').toLowerCase()
      return name.includes(q) || ver.includes(q) || ldr.includes(q) || ldrVer.includes(q) || grp.includes(q) || srv.includes(q)
    })
  }
  return list
})

/* All configured groups (from settings + instance groups) */
const allConfiguredGroups = computed(() => {
  const set = new Set()
  if (store.settings && store.settings.groups) {
    for (const g of store.settings.groups) {
      if (g && g.trim()) set.add(g.trim())
    }
  }
  for (const ins of store.instances) {
    if (ins.group && ins.group.trim()) {
      set.add(ins.group.trim())
    }
  }
  return Array.from(set).sort()
})

/* Grouped instances for display (including empty folders!) */
const groupedSections = computed(() => {
  const list = filteredInstances.value
  const groupsMap = new Map()
  const noGroup = []

  for (const g of allConfiguredGroups.value) {
    groupsMap.set(g, [])
  }

  for (const inst of list) {
    const grp = (inst.group || '').trim()
    if (!grp) {
      noGroup.push(inst)
    } else {
      if (!groupsMap.has(grp)) {
        groupsMap.set(grp, [])
      }
      groupsMap.get(grp).push(inst)
    }
  }

  const result = []
  const groupNames = Array.from(groupsMap.keys()).sort()
  for (const name of groupNames) {
    if (searchQuery.value.trim() && groupsMap.get(name).length === 0) {
      continue
    }
    result.push({
      name,
      isDefault: false,
      instances: groupsMap.get(name)
    })
  }

  if (noGroup.length > 0 || groupNames.length === 0) {
    result.push({
      name: groupNames.length > 0 ? (t('inst.noGroup') || 'Без папки') : '',
      isDefault: true,
      instances: noGroup
    })
  }

  return result
})

const existingGroups = computed(() => allConfiguredGroups.value)

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

async function quickPlay(inst) {
  if (isWorking.value || isPlaying.value) return
  if (inst.id !== store.settings.activeInstance) {
    await chooseInstance(inst)
  }
  onPlay()
}

/* Drag and Drop implementation */
function onDragStart(e, inst) {
  draggedInst.value = inst
  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', inst.id)
}

function onDragEnd() {
  draggedInst.value = null
  dragOverCardId.value = null
  dragOverGroupName.value = null
}

function onDragOverCard(e, targetInst) {
  e.preventDefault()
  if (draggedInst.value && draggedInst.value.id !== targetInst.id) {
    dragOverCardId.value = targetInst.id
  }
}

function onDragLeaveCard(e, targetInst) {
  if (dragOverCardId.value === targetInst.id) {
    dragOverCardId.value = null
  }
}

async function onDropOnCard(e, targetInst) {
  e.preventDefault()
  dragOverCardId.value = null
  const src = draggedInst.value
  if (!src || src.id === targetInst.id) return

  const currentList = [...store.instances]
  const fromIdx = currentList.findIndex(i => i.id === src.id)
  const toIdx = currentList.findIndex(i => i.id === targetInst.id)
  if (fromIdx === -1 || toIdx === -1) return

  const targetGroup = targetInst.group || ''
  if ((src.group || '') !== targetGroup) {
    src.group = targetGroup
    try {
      await UpdateInstanceGroup(src.id, targetGroup)
    } catch (err) {}
  }

  currentList.splice(fromIdx, 1)
  currentList.splice(toIdx, 0, src)
  store.instances = currentList

  try {
    const orderedIDs = currentList.map(i => i.id)
    await ReorderInstances(orderedIDs)
  } catch (err) {}
}

function onDragOverGroup(e, groupName) {
  e.preventDefault()
  dragOverGroupName.value = groupName
}

function onDragLeaveGroup(e, groupName) {
  if (dragOverGroupName.value === groupName) {
    dragOverGroupName.value = null
  }
}

async function onDropOnGroup(e, groupName) {
  e.preventDefault()
  dragOverGroupName.value = null
  const src = draggedInst.value
  if (!src) return

  const targetGroup = (groupName === (t('inst.noGroup') || 'Без папки')) ? '' : groupName
  if ((src.group || '') === targetGroup) return

  src.group = targetGroup
  try {
    await UpdateInstanceGroup(src.id, targetGroup)
  } catch (err) {
    toast(t('inst.err') + err, true)
  }
}

function toggleGroupCollapse(groupName) {
  collapsedGroups.value[groupName] = !collapsedGroups.value[groupName]
}

/* Custom Folder Modal Handlers */
function openCreateFolderModal() {
  folderModalMode.value = 'create'
  folderModalName.value = ''
  folderModalSelectedInsts.value = []
  folderModalOpen.value = true
}

function openRenameFolderModal(groupName) {
  folderModalMode.value = 'rename'
  folderModalOldName.value = groupName
  folderModalName.value = groupName
  folderModalOpen.value = true
}

async function submitFolderModal() {
  const name = folderModalName.value.trim()
  if (!name) return

  if (folderModalMode.value === 'create') {
    try {
      await CreateGroup(name)
      if (!store.settings.groups) store.settings.groups = []
      if (!store.settings.groups.includes(name)) {
        store.settings.groups.push(name)
      }
      for (const instId of folderModalSelectedInsts.value) {
        const inst = store.instances.find(i => i.id === instId)
        if (inst) {
          await assignInstanceToGroup(inst, name)
        }
      }
      toast(`Папка «${name}» создана`)
      folderModalOpen.value = false
    } catch (e) {
      toast(t('inst.err') + e, true)
    }
  } else if (folderModalMode.value === 'rename') {
    try {
      await RenameGroup(folderModalOldName.value, name)
      if (store.settings.groups) {
        const idx = store.settings.groups.indexOf(folderModalOldName.value)
        if (idx !== -1) store.settings.groups[idx] = name
      }
      for (const ins of store.instances) {
        if (ins.group === folderModalOldName.value) {
          ins.group = name
        }
      }
      toast(`Папка переименована в «${name}»`)
      folderModalOpen.value = false
    } catch (e) {
      toast(t('inst.err') + e, true)
    }
  }
}

function promptDeleteFolder(groupName) {
  folderToDelete.value = groupName
}

async function confirmDeleteFolder() {
  if (!folderToDelete.value) return
  const name = folderToDelete.value
  folderToDelete.value = null
  try {
    await DeleteGroup(name)
    if (store.settings.groups) {
      store.settings.groups = store.settings.groups.filter(g => g !== name)
    }
    for (const ins of store.instances) {
      if (ins.group === name) {
        ins.group = ''
      }
    }
    toast(`Папка «${name}» удалена`)
  } catch (e) {
    toast(t('inst.err') + e, true)
  }
}

async function assignInstanceToGroup(inst, groupName) {
  try {
    const updated = await UpdateInstanceGroup(inst.id, groupName)
    if (updated) {
      inst.group = updated.group
      const inStore = store.instances.find(i => i.id === inst.id)
      if (inStore) inStore.group = updated.group
      toast(`Сборка перенесена в папку «${groupName || t('inst.noGroup')}»`)
    }
  } catch (e) {
    toast(t('inst.err') + e, true)
  }
}

/* instance context menu */
const ctxMenuVisible = ref(false)
const ctxMenuPos = ref({x: 0, y: 0})
const ctxMenuInst = ref(null)
const groupSubmenuOpen = ref(false)

function onInstContextMenu(e, inst) {
  e.preventDefault()
  ctxMenuInst.value = inst
  groupSubmenuOpen.value = false
  const menuWidth = 220
  const menuHeight = 220
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
  groupSubmenuOpen.value = false
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

function ctxMoveToGroup(groupName) {
  const inst = ctxMenuInst.value
  closeContextMenu()
  if (inst) {
    assignInstanceToGroup(inst, groupName)
  }
}

function ctxPromptNewGroup() {
  closeContextMenu()
  openCreateFolderModal()
}

function ctxRemoveFromGroup() {
  const inst = ctxMenuInst.value
  closeContextMenu()
  if (inst) {
    assignInstanceToGroup(inst, '')
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
              <div class="play-fill" :style="{width: playFill}"></div>
              <svg v-if="!isPlaying" class="play-icon" viewBox="0 0 24 24" fill="currentColor">
                <path d="M8 5v14l11-7z"/>
              </svg>
              <svg v-else class="play-icon" viewBox="0 0 24 24" fill="currentColor">
                <rect x="6" y="6" width="12" height="12" rx="2"/>
              </svg>
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

    <!-- Header with Full-text Search & Folder Actions -->
    <div class="home-inst-header">
      <div class="section-head" style="margin-bottom:0">
        <h2>{{ t('inst.title') }}</h2>
      </div>

      <div class="home-inst-toolbar" v-if="hasInstances">
        <div class="home-inst-search">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
          </svg>
          <input
            type="text"
            v-model="searchQuery"
            :placeholder="t('inst.searchAll')"
          >
          <button v-if="searchQuery" class="clear-btn" @click="searchQuery = ''">✕</button>
        </div>

        <div class="home-inst-actions">
          <button class="btn-folder-new" @click="openCreateFolderModal" :title="t('folder.newTitle')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>
              <line x1="12" y1="10" x2="12" y2="16"/><line x1="9" y1="13" x2="15" y2="13"/>
            </svg>
            <span>{{ t('inst.newGroup') }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Instance Groups & Folders -->
    <div class="inst-groups-container" v-if="hasInstances">
      <div
        v-for="group in groupedSections"
        :key="group.name"
        class="inst-group-section"
      >
        <!-- Group Folder Header (if name exists) -->
        <div
          v-if="group.name"
          class="inst-group-header"
          :class="{'drag-over': dragOverGroupName === group.name}"
          @click="toggleGroupCollapse(group.name)"
          @dragover="onDragOverGroup($event, group.name)"
          @dragleave="onDragLeaveGroup($event, group.name)"
          @drop="onDropOnGroup($event, group.name)"
        >
          <div class="inst-group-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>
            </svg>
          </div>
          <span class="inst-group-title">{{ group.name }}</span>
          <span class="inst-group-count">{{ group.instances.length }}</span>

          <div class="inst-group-actions" @click.stop>
            <button
              v-if="!group.isDefault"
              class="inst-group-action-btn"
              :title="t('ctx.renameGroup')"
              @click="openRenameFolderModal(group.name)"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/>
              </svg>
            </button>
            <button
              v-if="!group.isDefault"
              class="inst-group-action-btn danger"
              :title="t('ctx.deleteGroup')"
              @click="promptDeleteFolder(group.name)"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M3 6h18 M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
              </svg>
            </button>
            <button
              class="inst-group-action-btn"
              @click="toggleGroupCollapse(group.name)"
            >
              <svg
                class="inst-group-chevron"
                :class="{collapsed: !!collapsedGroups[group.name]}"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2.5"
                stroke-linecap="round"
              >
                <path d="m6 9 6 6 6-6"/>
              </svg>
            </button>
          </div>
        </div>

        <!-- Empty Group Drop Zone -->
        <div
          v-if="group.name && group.instances.length === 0 && !collapsedGroups[group.name]"
          class="inst-group-empty"
          :class="{'drag-over': dragOverGroupName === group.name}"
          @dragover="onDragOverGroup($event, group.name)"
          @dragleave="onDragLeaveGroup($event, group.name)"
          @drop="onDropOnGroup($event, group.name)"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>
          </svg>
          <span>{{ t('folder.emptyDropHint') }}</span>
        </div>

        <!-- Group Instance Grid -->
        <div
          class="inst-grid"
          v-show="group.instances.length > 0 && !collapsedGroups[group.name]"
        >
          <div
            v-for="inst in group.instances"
            :key="inst.id"
            class="inst-card"
            :class="{
              active: activeInst && inst.id === activeInst.id,
              dragging: draggedInst && draggedInst.id === inst.id,
              'drag-over': dragOverCardId === inst.id
            }"
            draggable="true"
            @dragstart="onDragStart($event, inst)"
            @dragend="onDragEnd"
            @dragover="onDragOverCard($event, inst)"
            @dragleave="onDragLeaveCard($event, inst)"
            @drop="onDropOnCard($event, inst)"
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
              <div class="inst-sub">{{ inst.versionId }} • {{ t('loader.' + inst.loader) }}</div>
            </div>
            <button
              class="inst-play-btn"
              :title="t('home.play')"
              @click.stop="quickPlay(inst)"
            >
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
            </button>
          </div>
        </div>
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

        <!-- Move to folder options -->
        <button class="ctx-item" @click="groupSubmenuOpen = !groupSubmenuOpen">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>
          </svg>
          <span>{{ t('inst.moveToGroup') }}</span>
        </button>
        <template v-if="groupSubmenuOpen">
          <button
            v-for="g in existingGroups"
            :key="g"
            class="ctx-item"
            style="padding-left: 28px; font-size: 12px;"
            @click="ctxMoveToGroup(g)"
          >
            <span>📁 {{ g }}</span>
          </button>
          <button class="ctx-item" style="padding-left: 28px; font-size: 12px; color: var(--green);" @click="ctxPromptNewGroup">
            <span>{{ t('inst.newGroup') }}</span>
          </button>
        </template>

        <button v-if="ctxMenuInst && ctxMenuInst.group" class="ctx-item" @click="ctxRemoveFromGroup">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M18 6 6 18M6 6l12 12"/>
          </svg>
          <span>{{ t('inst.removeFromGroup') }}</span>
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

    <!-- Custom In-App Folder Create / Rename Modal -->
    <div class="modal-root" v-if="folderModalOpen">
      <div class="modal-backdrop" @click="folderModalOpen = false"></div>
      <div class="modal-box create-modal" @click.stop>
        <div class="modal-header">
          <h3 class="modal-title">{{ folderModalMode === 'create' ? t('folder.newTitle') : t('folder.renameTitle') }}</h3>
          <button class="modal-close" @click="folderModalOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <div class="fld-group">
            <label class="fld-label">{{ t('folder.nameLabel') }}</label>
            <input
              class="txt-in full-w"
              v-model="folderModalName"
              :placeholder="t('folder.namePh')"
              maxlength="40"
              @keyup.enter="submitFolderModal"
              autofocus
            >
          </div>

          <div class="fld-group" v-if="folderModalMode === 'create' && store.instances.length > 0">
            <label class="fld-label">{{ t('folder.selectInstances') }}</label>
            <div class="folder-inst-picker">
              <label
                v-for="ins in store.instances"
                :key="ins.id"
                class="folder-inst-item"
              >
                <input
                  type="checkbox"
                  :value="ins.id"
                  v-model="folderModalSelectedInsts"
                >
                <span>{{ ins.name }}</span>
                <em style="margin-left:auto;font-size:11px;color:var(--muted)">{{ ins.group || t('inst.noGroup') }}</em>
              </label>
            </div>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="folderModalOpen = false">{{ t('inst.deleteCancel') }}</button>
          <button
            class="btn-primary"
            :disabled="!folderModalName.trim()"
            @click="submitFolderModal"
          >
            {{ folderModalMode === 'create' ? t('inst.createBtn') : t('settings.save') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Custom In-App Delete Folder Confirmation Modal -->
    <div v-if="folderToDelete" class="confirm-modal-backdrop" @click="folderToDelete = null">
      <div class="confirm-modal-box" @click.stop>
        <div class="confirm-icon-wrap">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/>
          </svg>
        </div>
        <h3 class="confirm-title">{{ t('ctx.deleteGroup') }}</h3>
        <p class="confirm-text">{{ t('folder.deleteConfirm', {name: folderToDelete}) }}</p>
        <div class="confirm-actions">
          <button class="btn-sec" @click="folderToDelete = null">{{ t('inst.deleteCancel') }}</button>
          <button class="btn-danger" @click="confirmDeleteFolder">{{ t('inst.deleteConfirm') }}</button>
        </div>
      </div>
    </div>
  </section>
</template>
