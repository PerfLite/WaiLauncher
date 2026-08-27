<script setup>
import {ref, watch, computed, onMounted, nextTick} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {
  Play, CancelPlay, StopGame,
  SetActiveInstance, CreateInstance, DeleteInstance,
  GetInstanceAllContent, ToggleInstanceContent, DeleteInstanceContent,
  CheckInstanceModUpdates, UpdateInstanceMod,
  GetInstanceWorlds, DeleteInstanceWorld,
  SearchModrinthMods, InstallModrinthMod, CheckModDependencies, InstallModWithDependencies,
  SearchCurseForgeMods, InstallCurseForgeMod, CheckCurseForgeDependencies, InstallCurseForgeModWithDependencies,
  GetInstanceLogs, OpenInstanceDir, OpenInstanceSubFolder, ShowFileInExplorer, GetLoaderVersions,
  PickInstanceIcon,
  GetInstanceScreenshots, DeleteInstanceScreenshot, OpenScreenshotsFolder,
  ExportInstance, ImportInstanceDialog, ImportInstanceFile,
  UpdateInstanceSettings, CloneInstance, GetInstanceCrashReports,
  UpdateInstanceLaunchConfig, PickJavaPath
} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'

const currentTab = ref('content') // 'content' | 'worlds' | 'screenshots' | 'logs' | 'crashes'
const contentTypeFilter = ref('all') // 'all' | 'mod' | 'resourcepack' | 'shaderpack' | 'datapack'
const searchQuery = ref('')
const sortOrder = ref('name_asc') // 'name_asc' | 'name_desc' | 'size'
const sortDropdownOpen = ref(false)

const allContent = ref([])
const loadingContent = ref(false)
const updatesMap = ref({})
const checkingUpdates = ref(false)
const updatingMap = ref({})
const activeRowMenu = ref(null)

const worldsList = ref([])
const loadingWorlds = ref(false)

/* Screenshots */
const screenshotsList = ref([])
const loadingScreenshots = ref(false)
const activeScreenshot = ref(null)
const screenshotToDelete = ref(null)

/* Export & Import */
const exportingInst = ref(false)
const importingInst = ref(false)

/* Edit Instance Settings */
const editSettingsOpen = ref(false)
const editName = ref('')
const editServer = ref('')
const editVersion = ref('')
const editLoader = ref('vanilla')
const editLoaderVersion = ref('')
const editLoaderVerList = ref([])
const editLoaderVerLoading = ref(false)
const editLoaderVerErr = ref(false)
const editVerQuery = ref('')
const savingSettings = ref(false)

const loaders = [
  { id: 'vanilla' },
  { id: 'fabric' },
  { id: 'forge' },
  { id: 'neoforge' }
]

const typeNames = computed(() => ({
  release: t('type.release'),
  snapshot: t('type.snapshot'),
  old_beta: t('type.old_beta'),
  old_alpha: t('type.old_alpha'),
}))

const modalEditVersions = computed(() => {
  const q = editVerQuery.value.trim().toLowerCase()
  return store.versions.filter(v => {
    if (!store.settings.showSnapshots && v.type !== 'release') return false
    if (!q) return true
    return v.id.toLowerCase().includes(q)
  })
})

async function fetchEditLoaderVersions(loader, mcVer) {
  if (loader === 'vanilla' || !mcVer) {
    editLoaderVerList.value = []
    editLoaderVersion.value = ''
    editLoaderVerErr.value = false
    return
  }
  editLoaderVerLoading.value = true
  editLoaderVerErr.value = false
  try {
    const list = await GetLoaderVersions(loader, mcVer)
    editLoaderVerList.value = list || []
    if (list && list.length) {
      if (!editLoaderVersion.value || !list.some(v => v.version === editLoaderVersion.value)) {
        const rec = list.find(v => v.label === 'recommended')
        editLoaderVersion.value = rec ? rec.version : list[0].version
      }
    } else {
      editLoaderVersion.value = ''
    }
  } catch (e) {
    editLoaderVerErr.value = true
    editLoaderVerList.value = []
    editLoaderVersion.value = ''
  } finally {
    editLoaderVerLoading.value = false
  }
}

watch([editLoader, editVersion], ([ld, mc]) => {
  if (editSettingsOpen.value) {
    fetchEditLoaderVersions(ld, mc)
  }
})

const logsText = ref('')
const loadingLogs = ref(false)

/* Crash Reports */
const crashReports = ref([])
const loadingCrashes = ref(false)

/* Log search (Ctrl+F) */
const logSearchOpen = ref(false)
const logSearchQuery = ref('')

/* Per-instance launch config (edited inside the unified settings modal) */
const editRAMGB = ref(0) // 0 = inherit
const editJavaPath = ref('')
const editJVMPreset = ref('')
const editJVMArgs = ref('')
const editUseCustomWindow = ref(false)
const editFullscreen = ref(false)
const editWinW = ref(854)
const editWinH = ref(480)

/* Selection */
const selectedFiles = ref({})

/* In-place Find Projects View (Modrinth Style) */
const viewMode = ref('manage') // 'manage' | 'browse'
const browseSource = ref('modrinth') // 'modrinth' | 'curseforge'
const browseCategory = ref('mod') // 'mod' | 'resourcepack' | 'shader'
const browseQuery = ref('')
const browseResults = ref([])
const loadingBrowse = ref(false)
const installingModMap = ref({})

/* Current active/selected instance */
const selectedInst = computed(() => {
  if (!store.instances || store.instances.length === 0) return null
  if (store.selectedInstanceId) {
    const found = store.instances.find(i => i.id === store.selectedInstanceId)
    if (found) return found
  }
  return store.instances.find(i => i.id === store.settings.activeInstance) || store.instances[0]
})

const activeInst = computed(() =>
  store.instances.find(i => i.id === store.settings.activeInstance) || store.instances[0] || null
)

const isVanilla = computed(() => !selectedInst.value || selectedInst.value.loader === 'vanilla')

const filteredContent = computed(() => {
  let list = allContent.value || []
  if (contentTypeFilter.value !== 'all') {
    list = list.filter(item => item.type === contentTypeFilter.value)
  }
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(item =>
      (item.name && item.name.toLowerCase().includes(q)) ||
      (item.filename && item.filename.toLowerCase().includes(q))
    )
  }
  if (sortOrder.value === 'name_asc') {
    list.sort((a, b) => (a.name || '').localeCompare(b.name || ''))
  } else if (sortOrder.value === 'name_desc') {
    list.sort((a, b) => (b.name || '').localeCompare(a.name || ''))
  } else if (sortOrder.value === 'size') {
    list.sort((a, b) => (b.size || 0) - (a.size || 0))
  }
  return list
})

const availableUpdatesCount = computed(() => {
  return Object.keys(updatesMap.value).length
})

const isAllSelected = computed(() => {
  if (!filteredContent.value.length) return false
  return filteredContent.value.every(item => selectedFiles.value[item.filename])
})

const selectedCount = computed(() => {
  return Object.values(selectedFiles.value).filter(Boolean).length
})

watch(selectedInst, (inst) => {
  if (inst) {
    selectedFiles.value = {}
    activeRowMenu.value = null
    loadContent()
    if (viewMode.value === 'browse') {
      searchBrowseProjects()
    }
    if (currentTab.value === 'worlds') {
      loadWorlds()
    } else if (currentTab.value === 'screenshots') {
      loadScreenshots()
    } else if (currentTab.value === 'logs') {
      loadLogs()
    } else if (currentTab.value === 'crashes') {
      loadCrashes()
    }
  }
}, {immediate: true})

watch(currentTab, (tab) => {
  if (tab === 'content') {
    loadContent()
  } else if (tab === 'worlds') {
    loadWorlds()
  } else if (tab === 'screenshots') {
    loadScreenshots()
  } else if (tab === 'logs') {
    loadLogs()
  } else if (tab === 'crashes') {
    loadCrashes()
  }
})

async function loadContent() {
  if (!selectedInst.value) return
  loadingContent.value = true
  try {
    const list = await GetInstanceAllContent(selectedInst.value.id)
    allContent.value = list || []
    checkUpdatesSilent()
  } catch (e) {
    allContent.value = []
  } finally {
    loadingContent.value = false
  }
}

async function checkUpdatesSilent() {
  if (!selectedInst.value || isVanilla.value) return
  checkingUpdates.value = true
  try {
    const res = await CheckInstanceModUpdates(selectedInst.value.id)
    updatesMap.value = res || {}
    for (const item of allContent.value) {
      if (updatesMap.value[item.filename]) {
        const u = updatesMap.value[item.filename]
        item.hasUpdate = true
        item.updateVer = u.updateVer
        item.updateUrl = u.updateUrl
        item.updateFile = u.updateFile
      } else {
        item.hasUpdate = false
      }
    }
  } catch (e) {
    updatesMap.value = {}
  } finally {
    checkingUpdates.value = false
  }
}

async function updateMod(item) {
  if (!selectedInst.value || !item.updateUrl) return
  updatingMap.value[item.filename] = true
  try {
    await UpdateInstanceMod(
      selectedInst.value.id,
      item.filename,
      item.updateUrl,
      item.updateFile || item.filename
    )
    toast(t('mods.added').replace('{m}', item.name) || `Мод «${item.name}» обновлён`)
    // In-place update without reloading the page / without spinner
    const oldFn = item.filename
    if (item.updateVer) item.version = item.updateVer
    if (item.updateFile) item.filename = item.updateFile
    item.hasUpdate = false
    delete updatesMap.value[oldFn]
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    updatingMap.value[item.filename] = false
  }
}

async function updateAllMods() {
  const updates = Object.values(updatesMap.value)
  if (!updates.length || !selectedInst.value) return
  for (const u of updates) {
    try {
      await UpdateInstanceMod(selectedInst.value.id, u.filename, u.updateUrl, u.updateFile || u.filename)
      const item = allContent.value.find(i => i.filename === u.filename)
      if (item) {
        if (u.updateVer) item.version = u.updateVer
        if (u.updateFile) item.filename = u.updateFile
        item.hasUpdate = false
      }
      delete updatesMap.value[u.filename]
    } catch (e) {}
  }
  toast('Все доступные моды успешно обновлены!')
}

async function onToggleItem(item) {
  if (!selectedInst.value) return
  const targetState = !item.enabled
  try {
    await ToggleInstanceContent(selectedInst.value.id, item.type, item.filename, targetState)
    item.enabled = targetState
    if (targetState) {
      item.filename = item.filename.replace(/\.disabled$/, '')
    } else if (!item.filename.endsWith('.disabled')) {
      item.filename += '.disabled'
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

const modToDelete = ref(null)
const batchDeleteModal = ref(false)

function promptDeleteMod(item) {
  activeRowMenu.value = null
  modToDelete.value = item
}

async function confirmDeleteMod() {
  if (!modToDelete.value || !selectedInst.value) return
  const item = modToDelete.value
  modToDelete.value = null
  try {
    await DeleteInstanceContent(selectedInst.value.id, item.type, item.filename)
    allContent.value = allContent.value.filter(i => i.filename !== item.filename)
    delete selectedFiles.value[item.filename]
    toast(t('mods.removed').replace('{m}', item.name))
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

function promptBatchDelete() {
  if (selectedCount.value > 0) {
    batchDeleteModal.value = true
  }
}

async function confirmBatchDelete() {
  batchDeleteModal.value = false
  if (!selectedInst.value) return
  const selected = filteredContent.value.filter(item => selectedFiles.value[item.filename])
  for (const item of selected) {
    try {
      await DeleteInstanceContent(selectedInst.value.id, item.type, item.filename)
      allContent.value = allContent.value.filter(i => i.filename !== item.filename)
      delete selectedFiles.value[item.filename]
    } catch (e) {}
  }
  selectedFiles.value = {}
  toast('Выбранные файлы удалены')
}

async function showInExplorer(item) {
  if (!selectedInst.value) return
  activeRowMenu.value = null
  try {
    await ShowFileInExplorer(selectedInst.value.id, item.type, item.filename)
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

function toggleSelectAll() {
  const willSelect = !isAllSelected.value
  for (const item of filteredContent.value) {
    selectedFiles.value[item.filename] = willSelect
  }
}

async function batchToggle(enable) {
  if (!selectedInst.value) return
  const selected = filteredContent.value.filter(item => selectedFiles.value[item.filename])
  for (const item of selected) {
    if (item.enabled !== enable) {
      await onToggleItem(item)
    }
  }
}

/* Worlds Management */
const worldToDelete = ref(null)

async function loadWorlds() {
  if (!selectedInst.value) return
  loadingWorlds.value = true
  try {
    const list = await GetInstanceWorlds(selectedInst.value.id)
    worldsList.value = list || []
  } catch (e) {
    worldsList.value = []
  } finally {
    loadingWorlds.value = false
  }
}

function promptDeleteWorld(w) {
  worldToDelete.value = w
}

async function confirmDeleteWorld() {
  if (!worldToDelete.value || !selectedInst.value) return
  const w = worldToDelete.value
  worldToDelete.value = null
  try {
    await DeleteInstanceWorld(selectedInst.value.id, w.folderName)
    worldsList.value = worldsList.value.filter(i => i.folderName !== w.folderName)
    toast('Мир удалён')
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

function openWorldsFolder() {
  if (selectedInst.value) {
    OpenInstanceSubFolder(selectedInst.value.id, 'saves')
  }
}

/* Logs */
const logViewerRef = ref(null)

function scrollLogsToBottom() {
  nextTick(() => {
    if (logViewerRef.value) {
      logViewerRef.value.scrollTop = logViewerRef.value.scrollHeight
    }
  })
}

onMounted(() => {
  EventsOn('gamelog', (line) => {
    if (!line) return
    if (!logsText.value) {
      logsText.value = line + '\n'
    } else {
      logsText.value += line + '\n'
    }
    if (logsText.value.length > 300000) {
      logsText.value = logsText.value.slice(-200000)
    }
    if (currentTab.value === 'logs') {
      scrollLogsToBottom()
    }
  })

  // Ctrl+F opens log search when viewing the logs tab
  window.addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f' && currentTab.value === 'logs') {
      e.preventDefault()
      logSearchOpen.value = true
    }
  })

  // Drag-and-drop import of .mrpack/.zip build archives
  EventsOn('file-drop-import', async (p) => {
    if (!p) return
    try {
      const inst = await ImportInstanceFile(p)
      if (inst) {
        toast(t('inst.importSuccess'))
        store.selectedInstanceId = inst.id
      }
    } catch (e) {
      toast((t('inst.err') || 'Ошибка: ') + e, true)
    }
  })
})

async function loadLogs() {
  if (!selectedInst.value) return
  loadingLogs.value = true
  try {
    const s = await GetInstanceLogs(selectedInst.value.id)
    logsText.value = s || ''
    scrollLogsToBottom()
  } catch (e) {
    logsText.value = ''
  } finally {
    loadingLogs.value = false
  }
}

function copyLogs() {
  if (!logsText.value) return
  navigator.clipboard.writeText(logsText.value).then(() => {
    toast(t('profile.logsCopied'))
  }).catch(() => {})
}

function toggleLogSearch() {
  logSearchOpen.value = !logSearchOpen.value
  logSearchQuery.value = ''
}

async function loadCrashes() {
  if (!selectedInst.value) return
  loadingCrashes.value = true
  try {
    const list = await GetInstanceCrashReports(selectedInst.value.id)
    crashReports.value = list || []
  } catch (e) {
    crashReports.value = []
  } finally {
    loadingCrashes.value = false
  }
}

function openCrashReportsFolder() {
  if (selectedInst.value) {
    OpenInstanceSubFolder(selectedInst.value.id, 'crash-reports')
  }
}

async function onCloneInstance() {
  if (!selectedInst.value) return
  try {
    const created = await CloneInstance(selectedInst.value.id)
    if (created) {
      toast(t('inst.cloned') || 'Сборка скопирована')
      store.selectedInstanceId = created.id
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

/* Folder Actions */
function openInstanceFolder() {
  if (selectedInst.value) {
    OpenInstanceDir(selectedInst.value.id)
  }
}

function openModsFolder() {
  if (selectedInst.value) {
    OpenInstanceSubFolder(selectedInst.value.id, 'mods')
  }
}

/* In-place Find Projects (Browse view) */
let browseTimer = null
function onBrowseInput() {
  clearTimeout(browseTimer)
  browseTimer = setTimeout(() => {
    searchBrowseProjects()
  }, 350)
}

function openBrowse() {
  viewMode.value = 'browse'
  if (browseResults.value.length === 0 && !isVanilla.value) {
    searchBrowseProjects()
  }
}

function closeBrowse() {
  viewMode.value = 'manage'
}

watch([browseCategory, browseSource], () => {
  if (viewMode.value === 'browse') {
    searchBrowseProjects()
  }
})

async function searchBrowseProjects() {
  if (!selectedInst.value) return
  loadingBrowse.value = true
  try {
    let res
    if (browseSource.value === 'curseforge') {
      res = await SearchCurseForgeMods(
        browseQuery.value,
        browseCategory.value,
        selectedInst.value.loader,
        selectedInst.value.versionId,
        0,
        40
      )
    } else {
      res = await SearchModrinthMods(
        browseQuery.value,
        browseCategory.value,
        selectedInst.value.loader,
        selectedInst.value.versionId,
        0,
        40
      )
    }
    browseResults.value = (res && res.hits) ? res.hits : []
  } catch (e) {
    browseResults.value = []
  } finally {
    loadingBrowse.value = false
  }
}

function isModInstalled(hit) {
  const title = (hit.title || '').toLowerCase().replace(/[^a-z0-9]/g, '')
  const slug = (hit.slug || '').toLowerCase().replace(/[^a-z0-9]/g, '')
  return allContent.value.some(m => {
    const clean = (m.name || '').toLowerCase().replace(/[^a-z0-9]/g, '')
    const fileClean = (m.filename || '').toLowerCase().replace(/[^a-z0-9]/g, '')
    return (title && clean.includes(title)) || (slug && clean.includes(slug)) || (slug && fileClean.includes(slug))
  })
}

// Dependency Resolver Dialog State
const depModalOpen = ref(false)
const depTargetHit = ref(null)
const depList = ref([])
const depSelectedUrls = ref([])

async function installBrowseMod(hit) {
  if (!selectedInst.value) return
  const id = hit.project_id || hit.slug

  if (browseCategory.value === 'mod') {
    installingModMap.value[id] = true
    try {
      let deps
      if (browseSource.value === 'curseforge') {
        deps = await CheckCurseForgeDependencies(selectedInst.value.id, id)
      } else {
        deps = await CheckModDependencies(selectedInst.value.id, id)
      }
      const missing = (deps || []).filter(d => !d.alreadyInstalled)
      if (missing.length > 0) {
        depTargetHit.value = hit
        depList.value = missing
        const reqs = missing.filter(d => d.dependencyType === 'required').map(d => d.downloadUrl)
        depSelectedUrls.value = reqs.length > 0 ? reqs : missing.map(d => d.downloadUrl)
        depModalOpen.value = true
        installingModMap.value[id] = false
        return
      }
    } catch (e) {
      console.warn('Dependency check error:', e)
    }
  }

  await executeModInstall(hit, [])
}

async function executeModInstall(hit, depUrls) {
  if (!selectedInst.value || !hit) return
  const id = hit.project_id || hit.slug
  installingModMap.value[id] = true
  depModalOpen.value = false
  try {
    let mod
    if (browseSource.value === 'curseforge') {
      if (depUrls && depUrls.length > 0) {
        mod = await InstallCurseForgeModWithDependencies(selectedInst.value.id, id, browseCategory.value, depUrls)
      } else {
        mod = await InstallCurseForgeMod(selectedInst.value.id, id, browseCategory.value)
      }
    } else {
      if (depUrls && depUrls.length > 0) {
        mod = await InstallModWithDependencies(selectedInst.value.id, id, browseCategory.value, depUrls)
      } else {
        mod = await InstallModrinthMod(selectedInst.value.id, id, browseCategory.value)
      }
    }
    if (mod) {
      await loadContent()
      toast(t('mods.added').replace('{m}', hit.title))
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    installingModMap.value[id] = false
  }
}

/* Launching */
const launch = computed(() => store.launch)
const isWorking = computed(() => launch.value.state === 'working')
const isPlaying = computed(() => launch.value.state === 'playing')
const playFill = computed(() => isWorking.value ? (launch.value.percent || 0) + '%' : '0%')

async function onPlay() {
  if (isWorking.value) {
    CancelPlay().catch(() => {})
    return
  }
  if (isPlaying.value) {
    StopGame().catch(() => {})
    return
  }
  if (!selectedInst.value) return
  try {
    await SetActiveInstance(selectedInst.value.id)
    store.settings.activeInstance = selectedInst.value.id
    store.settings.selectedVersion = selectedInst.value.versionId
    await Play(selectedInst.value.id)
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

async function changeInstanceIcon() {
  if (!selectedInst.value) return
  try {
    const newIcon = await PickInstanceIcon(selectedInst.value.id)
    if (newIcon) {
      selectedInst.value.icon = newIcon
      const inStore = store.instances.find(i => i.id === selectedInst.value.id)
      if (inStore) inStore.icon = newIcon
      toast('Иконка сборки обновлена')
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

/* Delete Instance */
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
    if (store.selectedInstanceId === inst.id) {
      store.selectedInstanceId = newActive || (store.instances[0] ? store.instances[0].id : '')
    }
    toast(t('inst.deleted'))
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatDate(ts) {
  if (!ts) return 'Недавно'
  const d = new Date(ts * 1000)
  return d.toLocaleDateString()
}

function formatNumber(num) {
  if (!num) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + ' млн'
  if (num >= 1000) return (num / 1000).toFixed(1) + ' тыс.'
  return String(num)
}

function formatPlaytime(seconds) {
  if (!seconds || seconds <= 0) return ''
  const hours = (seconds / 3600).toFixed(1)
  if (hours < 1) {
    const mins = Math.max(1, Math.floor(seconds / 60))
    return mins + ' мин.'
  }
  return hours + ' ч.'
}

function formatLastPlayed(timestamp) {
  if (!timestamp || timestamp <= 0) return ''
  const d = new Date(timestamp * 1000)
  const now = new Date()
  const isToday = d.toDateString() === now.toDateString()
  const timeStr = d.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'})
  if (isToday) {
    return 'Сегодня, ' + timeStr
  }
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (d.toDateString() === yesterday.toDateString()) {
    return 'Вчера, ' + timeStr
  }
  return d.toLocaleDateString([], {day: '2-digit', month: '2-digit'}) + ' ' + timeStr
}

/* Screenshots Management */
async function loadScreenshots() {
  if (!selectedInst.value) return
  loadingScreenshots.value = true
  try {
    const res = await GetInstanceScreenshots(selectedInst.value.id)
    screenshotsList.value = res || []
  } catch (e) {
    screenshotsList.value = []
  } finally {
    loadingScreenshots.value = false
  }
}

async function copyScreenshotImage(item) {
  if (!item || !item.dataUrl) return
  try {
    const res = await fetch(item.dataUrl)
    const blob = await res.blob()
    await navigator.clipboard.write([
      new ClipboardItem({[blob.type]: blob})
    ])
    toast(t('inst.screenshotCopied'))
  } catch (e) {
    toast('Не удалось скопировать: ' + e, true)
  }
}

function promptDeleteScreenshot(item) {
  screenshotToDelete.value = item
}

async function confirmDeleteScreenshot() {
  if (!selectedInst.value || !screenshotToDelete.value) return
  const item = screenshotToDelete.value
  screenshotToDelete.value = null
  try {
    await DeleteInstanceScreenshot(selectedInst.value.id, item.filename)
    if (activeScreenshot.value && activeScreenshot.value.filename === item.filename) {
      activeScreenshot.value = null
    }
    await loadScreenshots()
    toast(t('inst.deleteScreenshot') + ' ✓')
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

function openScreenshotsFolder() {
  if (selectedInst.value) {
    OpenScreenshotsFolder(selectedInst.value.id).catch(() => {})
  }
}

/* Export Instance */
async function onExportInstance() {
  if (!selectedInst.value || exportingInst.value) return
  exportingInst.value = true
  try {
    const savedPath = await ExportInstance(selectedInst.value.id)
    if (savedPath) {
      toast(t('inst.exportSuccess'))
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    exportingInst.value = false
  }
}

/* Import Instance */
async function onImportInstance() {
  if (importingInst.value) return
  importingInst.value = true
  try {
    const inst = await ImportInstanceDialog()
    if (inst) {
      toast(t('inst.importSuccess'))
      store.selectedInstanceId = inst.id
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    importingInst.value = false
  }
}

/* Edit Instance Settings (unified: general + launch config) */
const settingsTab = ref('general') // 'general' | 'launch'

function openEditSettings() {
  if (!selectedInst.value) return
  const ins = selectedInst.value
  editName.value = ins.name
  editServer.value = ins.serverAddress || ''
  editVersion.value = ins.versionId || ''
  editLoader.value = ins.loader || 'vanilla'
  editLoaderVersion.value = ins.loaderVersion || ''
  editVerQuery.value = ''
  fetchEditLoaderVersions(editLoader.value, editVersion.value)
  editRAMGB.value = ins.ramMb ? Math.round(ins.ramMb / 1024) : 0
  editJavaPath.value = ins.javaPath || ''
  editJVMPreset.value = ins.jvmPreset || ''
  editJVMArgs.value = ins.jvmArgs || ''
  editUseCustomWindow.value = !!ins.useCustomWindow
  editFullscreen.value = !!ins.fullscreen
  editWinW.value = ins.windowWidth || 854
  editWinH.value = ins.windowHeight || 480
  settingsTab.value = 'general'
  editSettingsOpen.value = true
}

async function browseInstanceJava() {
  try {
    const p = await PickJavaPath()
    if (p) editJavaPath.value = p
  } catch (e) { /* preview mode */ }
}

async function saveInstanceSettings() {
  if (!selectedInst.value || savingSettings.value) return
  savingSettings.value = true
  try {
    const lv = editLoader.value === 'vanilla' ? '' : editLoaderVersion.value
    const updated = await UpdateInstanceSettings(
      selectedInst.value.id,
      editName.value,
      editServer.value,
      editVersion.value,
      editLoader.value,
      lv
    )
    if (updated) {
      selectedInst.value.name = updated.name
      selectedInst.value.serverAddress = updated.serverAddress
      selectedInst.value.versionId = updated.versionId
      selectedInst.value.loader = updated.loader
      selectedInst.value.loaderVersion = updated.loaderVersion
      const inStore = store.instances.find(i => i.id === updated.id)
      if (inStore) {
        Object.assign(inStore, updated)
      }
    }
    const ramMB = editRAMGB.value > 0 ? editRAMGB.value * 1024 : 0
    const cfg = await UpdateInstanceLaunchConfig(
      selectedInst.value.id,
      ramMB,
      editJavaPath.value,
      editJVMArgs.value,
      editJVMPreset.value,
      editUseCustomWindow.value,
      editFullscreen.value,
      editWinW.value,
      editWinH.value
    )
    if (cfg) {
      const inStore = store.instances.find(i => i.id === cfg.id)
      if (inStore) Object.assign(inStore, cfg)
    }
    toast(t('settings.saved'))
    editSettingsOpen.value = false
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    savingSettings.value = false
  }
}
</script>

<template>
  <section class="page page-instances" @click="activeRowMenu = null">
    <div class="mr-instance-view" v-if="selectedInst">

      <!-- ============================================== -->
      <!-- SUB-VIEW A: BROWSE & INSTALL CONTENT (Modrinth) -->
      <!-- ============================================== -->
      <div v-if="viewMode === 'browse'" class="mr-browse-container">
        <!-- Top Browse Header -->
        <div class="mr-browse-header">
          <button class="mr-btn-back" @click="closeBrowse" title="Назад к сборке">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="m15 18-6-6 6-6"/></svg>
          </button>

          <div class="mr-browse-header-info">
            <h2 class="mr-browse-title">{{ selectedInst.name }}</h2>
            <div class="mr-browse-sub">
              <span>{{ t('mr.browseContent') }}</span>
              <span class="mr-meta-dot">•</span>
              <span class="mr-browse-tag">🎮 Minecraft {{ selectedInst.versionId }}</span>
              <span class="mr-meta-dot">•</span>
              <span class="mr-browse-tag">🔨 {{ t('loader.' + selectedInst.loader) }}</span>
            </div>
          </div>
        </div>

        <!-- Browse Category Pills & Source Switcher -->
        <div class="mr-browse-cats">
          <div class="mr-source-switch">
            <button
              class="mr-source-btn"
              :class="{active: browseSource === 'modrinth'}"
              @click="browseSource = 'modrinth'"
            >
              Modrinth
            </button>
            <button
              class="mr-source-btn"
              :class="{active: browseSource === 'curseforge'}"
              @click="browseSource = 'curseforge'"
            >
              CurseForge
            </button>
          </div>

          <div class="mr-cat-pills-group">
            <button
              class="mr-browse-cat-pill"
              :class="{active: browseCategory === 'mod'}"
              @click="browseCategory = 'mod'"
            >
              {{ t('cat.mods') }}
            </button>
            <button
              class="mr-browse-cat-pill"
              :class="{active: browseCategory === 'resourcepack'}"
              @click="browseCategory = 'resourcepack'"
            >
              {{ t('cat.resourcepacks') }}
            </button>
            <button
              class="mr-browse-cat-pill"
              :class="{active: browseCategory === 'shader'}"
              @click="browseCategory = 'shader'"
            >
              {{ t('cat.shaders') }}
            </button>
          </div>
        </div>

        <!-- Browse Search & Controls Bar -->
        <div class="mr-browse-controls">
          <div class="mr-search-box full-w">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
            <input
              type="text"
              v-model="browseQuery"
              @input="onBrowseInput"
              :placeholder="browseSource === 'curseforge' ? 'Поиск на CurseForge…' : t('mr.searchOnModrinth')"
            >
            <button v-if="browseQuery" class="search-clear" @click="browseQuery = ''; searchBrowseProjects()">✕</button>
          </div>

          <div class="mr-browse-filter-line">
            <div class="mr-browse-badges">
              <span class="mr-filter-chip">🔒 {{ selectedInst.versionId }}</span>
              <span class="mr-filter-chip">🔒 {{ t('loader.' + selectedInst.loader) }}</span>
            </div>
          </div>
        </div>

        <!-- Browse Projects List -->
        <div class="mr-browse-list-wrap">
          <div v-if="loadingBrowse" class="profile-loading">
            <span class="bar-mini"><i></i></span>
          </div>

          <div v-else-if="browseResults.length === 0" class="mr-empty-pane">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
            <p>Ничего не найдено</p>
          </div>

          <div v-else class="mr-browse-grid">
            <div
              v-for="hit in browseResults"
              :key="hit.project_id"
              class="mr-browse-card"
            >
              <div class="mr-bcard-icon">
                <img v-if="hit.icon_url" :src="hit.icon_url" alt="" loading="lazy">
                <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20 M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
              </div>

              <div class="mr-bcard-main">
                <div class="mr-bcard-title-row">
                  <h3 class="mr-bcard-title">{{ hit.title }}</h3>
                  <span class="mr-bcard-author">by {{ hit.author }}</span>
                </div>
                <p class="mr-bcard-desc">{{ hit.description }}</p>

                <div class="mr-bcard-bottom">
                  <div class="mr-bcard-tags">
                    <span v-for="cat in (hit.categories || []).slice(0, 3)" :key="cat" class="mr-tag-pill">
                      {{ cat }}
                    </span>
                  </div>

                  <div class="mr-bcard-stats">
                    <span class="mr-stat">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
                      {{ formatNumber(hit.downloads) }}
                    </span>
                    <span class="mr-stat">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2 M12 3a4 4 0 1 0 0 8 4 4 0 0 0 0-8z"/></svg>
                      {{ formatNumber(hit.follows) }}
                    </span>
                  </div>
                </div>
              </div>

              <div class="mr-bcard-action">
                <button
                  v-if="isModInstalled(hit)"
                  class="mr-btn-installed-state"
                  disabled
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                  <span>Установлено</span>
                </button>
                <button
                  v-else
                  class="mr-btn-install-action"
                  :disabled="installingModMap[hit.project_id || hit.slug]"
                  @click="installBrowseMod(hit)"
                >
                  <svg v-if="!installingModMap[hit.project_id || hit.slug]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                  <span>{{ installingModMap[hit.project_id || hit.slug] ? 'Установка…' : 'Установить' }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- SUB-VIEW B: INSTANCE PROFILE & INSTALLED MODS   -->
      <!-- ============================================== -->
      <div v-else class="mr-instance-main-container">
        <!-- 1. MODRINTH-STYLE HEADER -->
        <div class="mr-header">
          <div class="mr-header-left">
            <div
              class="mr-header-avatar"
              :class="['loader-' + (selectedInst.loader || 'vanilla')]"
              @click="changeInstanceIcon"
              title="Нажмите, чтобы изменить иконку сборки"
            >
              <img v-if="selectedInst.icon" :src="selectedInst.icon" alt="" class="mr-header-avatar-img">
              <div v-else class="loader-avatar-fallback">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
                </svg>
              </div>
              <div class="avatar-edit-overlay">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M12 20h9M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>
              </div>
            </div>
            <div class="mr-header-info">
              <h1 class="mr-header-title">{{ selectedInst.name }}</h1>
              <div class="mr-header-meta">
                <span class="mr-meta-pill">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-mini-icon"><path d="M12 2v20 M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
                  {{ t('loader.' + selectedInst.loader) }} {{ selectedInst.versionId }}
                </span>
                <span v-if="selectedInst.playTime" class="mr-meta-pill" :title="'Общее время в игре: ' + formatPlaytime(selectedInst.playTime)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-mini-icon"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  {{ formatPlaytime(selectedInst.playTime) }}
                </span>
                <span v-if="selectedInst.serverAddress" class="mr-meta-pill" :title="'Сервер для быстрого входа: ' + selectedInst.serverAddress">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-mini-icon"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/></svg>
                  {{ selectedInst.serverAddress }}
                </span>
                <span class="mr-meta-dot">•</span>
                <span class="mr-meta-sub">{{ t('mr.projectsCount', {n: allContent.length}) }}</span>
                <span v-if="selectedInst.lastPlayed" class="mr-meta-sub">• {{ formatLastPlayed(selectedInst.lastPlayed) }}</span>
              </div>
            </div>
          </div>

          <div class="mr-header-actions">
            <button
              class="mr-btn-play"
              :class="{downloading: isWorking, playing: isPlaying}"
              @click="onPlay"
              :title="isWorking ? t('home.cancel') : isPlaying ? t('home.stopHint') : ''"
            >
              <span class="mr-btn-play-fill" :style="{width: playFill}"></span>
              
              <svg v-if="!isWorking && !isPlaying" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
              <svg v-else-if="isWorking" class="mr-btn-cancel-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
              <span v-else-if="isPlaying" class="mr-btn-pulse-dot"></span>
              <svg v-if="isPlaying" class="mr-btn-stop-icon" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>

              <span class="mr-btn-play-label">
                <template v-if="isPlaying">
                  <span class="label-running">{{ t('home.running') }}</span>
                  <span class="label-stop">{{ t('home.stop') }}</span>
                </template>
                <template v-else-if="isWorking">
                  <span class="label-downloading">{{ (launch.percent > 0 ? Math.floor(launch.percent) + '%' : t('home.loading')) }}</span>
                  <span class="label-stop">{{ t('home.cancel') }}</span>
                </template>
                <template v-else>{{ t('profile.play') }}</template>
              </span>
            </button>

            <button class="mr-btn-icon" :title="t('inst.editSettings')" @click="openEditSettings">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
            </button>

            <button class="mr-btn-icon" :title="t('profile.folder')" @click="openInstanceFolder">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
            </button>

            <button class="mr-btn-icon" :title="t('inst.deleteTitle')" @click="promptDeleteInst(selectedInst)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M3 6h18 M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>
            </button>
          </div>
        </div>

        <!-- 2. PILLS NAVIGATION TABS -->
        <div class="mr-nav-tabs">
          <button
            class="mr-tab-pill"
            :class="{active: currentTab === 'content'}"
            @click="currentTab = 'content'"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="m9 12 2 2 4-4"/></svg>
            <span>{{ t('tabs.content') }}</span>
          </button>

          <button
            class="mr-tab-pill"
            @click="openInstanceFolder"
            :title="t('profile.folder')"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
            <span>{{ t('tabs.files') }}</span>
          </button>

          <button
            class="mr-tab-pill"
            :class="{active: currentTab === 'worlds'}"
            @click="currentTab = 'worlds'"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/></svg>
            <span>{{ t('tabs.worlds') }}</span>
          </button>

          <button
            class="mr-tab-pill"
            :class="{active: currentTab === 'screenshots'}"
            @click="currentTab = 'screenshots'"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg>
            <span>{{ t('tabs.screenshots') }}</span>
          </button>

          <button
            class="mr-tab-pill"
            :class="{active: currentTab === 'logs'}"
            @click="currentTab = 'logs'"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
            <span>{{ t('tabs.logs') }}</span>
          </button>

          <button
            class="mr-tab-pill"
            :class="{active: currentTab === 'crashes'}"
            @click="currentTab = 'crashes'"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m8 2 1.88 1.88 M14.12 3.88 16 2 M9 7.13v-1a3.003 3.003 0 1 1 6 0v1"/><path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6v2"/><path d="M12 22v-2"/><circle cx="10" cy="13" r=".5" fill="currentColor"/><circle cx="14" cy="13" r=".5" fill="currentColor"/></svg>
            <span>{{ t('tabs.crashes') }}</span>
          </button>
        </div>

        <!-- 3. TAB CONTENT -->
        <div class="mr-view-body">
          <!-- TAB: CONTENT -->
          <div v-if="currentTab === 'content'" class="mr-content-pane">
            <!-- Top Content Toolbar -->
            <div class="mr-toolbar-row">
              <div class="mr-search-box">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
                <input
                  type="text"
                  v-model="searchQuery"
                  :placeholder="t('mr.searchProjects', {n: allContent.length})"
                >
                <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">✕</button>
              </div>

              <div class="mr-toolbar-buttons">
                <button class="mr-btn-secondary" @click="openModsFolder">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
                  <span>Добавить файлы</span>
                </button>

                <button class="mr-btn-green" @click="openBrowse">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><path d="M12 8v8M8 12h8"/></svg>
                  <span>Найти проекты</span>
                </button>
              </div>
            </div>

            <!-- Filter & Action Row -->
            <div class="mr-filter-row">
              <!-- Custom Sleek Dark Sort Dropdown -->
              <div class="custom-dropdown-wrap">
                <button class="custom-dropdown-btn" @click.stop="sortDropdownOpen = !sortDropdownOpen">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-mini-icon"><path d="m3 16 4 4 4-4M7 20V4M21 8l-4-4-4 4M17 4v16"/></svg>
                  <span>{{ sortOrder === 'name_asc' ? 'Name (A-Z)' : sortOrder === 'name_desc' ? 'Name (Z-A)' : 'Size' }}</span>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" class="dd-arrow"><path d="m6 9 6 6 6-6"/></svg>
                </button>
                <div v-if="sortDropdownOpen" class="custom-dropdown-menu" @click="sortDropdownOpen = false">
                  <div class="dd-item" :class="{selected: sortOrder === 'name_asc'}" @click="sortOrder = 'name_asc'">Name (A-Z)</div>
                  <div class="dd-item" :class="{selected: sortOrder === 'name_desc'}" @click="sortOrder = 'name_desc'">Name (Z-A)</div>
                  <div class="dd-item" :class="{selected: sortOrder === 'size'}" @click="sortOrder = 'size'">Size</div>
                </div>
              </div>

              <!-- Category Pills -->
              <div class="mr-type-pills">
                <button
                  class="mr-type-pill"
                  :class="{active: contentTypeFilter === 'all'}"
                  @click="contentTypeFilter = 'all'"
                >
                  {{ t('cat.all') }}
                </button>
                <button
                  class="mr-type-pill"
                  :class="{active: contentTypeFilter === 'mod'}"
                  @click="contentTypeFilter = 'mod'"
                >
                  {{ t('cat.mods') }}
                </button>
                <button
                  class="mr-type-pill"
                  :class="{active: contentTypeFilter === 'resourcepack'}"
                  @click="contentTypeFilter = 'resourcepack'"
                >
                  {{ t('cat.resourcepacks') }}
                </button>
                <button
                  class="mr-type-pill"
                  :class="{active: contentTypeFilter === 'shaderpack'}"
                  @click="contentTypeFilter = 'shaderpack'"
                >
                  {{ t('cat.shaders') }}
                </button>
                <button
                  class="mr-type-pill"
                  :class="{active: contentTypeFilter === 'datapack'}"
                  @click="contentTypeFilter = 'datapack'"
                >
                  {{ t('cat.datapacks') }}
                </button>
              </div>

              <!-- Right Toolbar: Update All & Animated Refresh -->
              <div class="mr-filter-right">
                <button
                  v-if="availableUpdatesCount > 0"
                  class="mr-btn-update-all"
                  @click="updateAllMods"
                  :title="`Обновить ${availableUpdatesCount} модов`"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
                  <span>Обновить всё ({{ availableUpdatesCount }})</span>
                </button>

                <button
                  class="mr-btn-refresh"
                  :class="{'is-spinning': loadingContent || checkingUpdates}"
                  @click="loadContent"
                  :title="`Перезагрузить`"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 21h5v-5"/></svg>
                  <span>Перезагрузить</span>
                </button>
              </div>
            </div>

            <!-- Batch Action Bar if selected -->
            <div v-if="selectedCount > 0" class="mr-batch-bar">
              <span>Выбрано: <b>{{ selectedCount }}</b></span>
              <div class="mr-batch-actions">
                <button class="btn-sec btn-sm" @click="batchToggle(true)">Включить</button>
                <button class="btn-sec btn-sm" @click="batchToggle(false)">Отключить</button>
                <button class="btn-primary btn-danger btn-sm" @click="promptBatchDelete">Удалить</button>
              </div>
            </div>

            <!-- Table Header -->
            <div class="mr-table-head">
              <div class="mr-th-project">
                <button
                  class="mr-custom-checkbox"
                  :class="{checked: isAllSelected}"
                  @click="toggleSelectAll"
                >
                  <svg v-if="isAllSelected" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5"><polyline points="20 6 9 17 4 12"/></svg>
                </button>
                <span>Проект</span>
              </div>
              <div class="mr-th-version">Версия</div>
              <div class="mr-th-actions">Действия</div>
            </div>

            <!-- Loading -->
            <div v-if="loadingContent" class="profile-loading">
              <span class="bar-mini"><i></i></span>
            </div>

            <!-- Empty state -->
            <div v-else-if="filteredContent.length === 0" class="mr-empty-pane">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
              </svg>
              <p>{{ searchQuery ? 'Ничего не найдено' : 'В этой сборке пока нет файлов.' }}</p>
              <button class="mr-btn-green" @click="openBrowse">Найти проекты</button>
            </div>

            <!-- Modrinth Table Rows -->
            <div v-else class="mr-table-body">
              <div
                v-for="item in filteredContent"
                :key="item.filename"
                class="mr-table-row"
                :class="{disabled: !item.enabled, selected: selectedFiles[item.filename]}"
              >
                <!-- Project Column -->
                <div class="mr-td-project">
                  <button
                    class="mr-custom-checkbox"
                    :class="{checked: selectedFiles[item.filename]}"
                    @click.stop="selectedFiles[item.filename] = !selectedFiles[item.filename]"
                  >
                    <svg v-if="selectedFiles[item.filename]" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5"><polyline points="20 6 9 17 4 12"/></svg>
                  </button>

                  <div class="mr-row-icon">
                    <img v-if="item.iconUrl" :src="item.iconUrl" alt="" loading="lazy">
                    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                      <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
                    </svg>
                  </div>
                  <div class="mr-row-text">
                    <div class="mr-row-title">{{ item.name }}</div>
                    <div class="mr-row-uploader">
                      <img v-if="item.authorAvatar" :src="item.authorAvatar" class="mr-author-avatar" alt="" loading="lazy">
                      <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-mini-icon"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M17 8l-5-5-5 5 M12 3v12"/></svg>
                      <span>{{ item.author || 'Uploaded' }}</span>
                    </div>
                  </div>
                </div>

                <!-- Version Column -->
                <div class="mr-td-version">
                  <div class="mr-ver-badge">{{ item.version || 'Custom' }}</div>
                  <div class="mr-ver-file" :title="item.filename">{{ item.filename }}</div>
                </div>

                <!-- Actions Column (Exact Modrinth App layout) -->
                <div class="mr-td-actions">
                  <!-- 1. Download Update Icon (only if update available) -->
                  <button
                    v-if="item.hasUpdate"
                    class="mr-btn-action-icon mr-btn-update"
                    :title="`Обновить мод до ${item.updateVer || 'новой версии'}`"
                    :disabled="updatingMap[item.filename]"
                    @click.stop="updateMod(item)"
                  >
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/>
                    </svg>
                  </button>

                  <!-- 2. Clean Green Toggle Switch -->
                  <button
                    class="mr-toggle-switch"
                    :class="{active: item.enabled}"
                    @click.stop="onToggleItem(item)"
                    :title="item.enabled ? 'Отключить' : 'Включить'"
                  >
                    <span class="mr-toggle-knob"></span>
                  </button>

                  <!-- 3. Trash Can Delete Button -->
                  <button
                    class="mr-btn-action-icon mr-btn-trash"
                    :title="t('profile.deleteMod')"
                    @click.stop="promptDeleteMod(item)"
                  >
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M3 6h18 M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2 M10 11v6 M14 11v6"/>
                    </svg>
                  </button>

                  <!-- 4. 3-Dots Context Menu with Show in Folder -->
                  <div class="mr-row-menu-wrap">
                    <button
                      class="mr-btn-action-icon"
                      title="Действия"
                      @click.stop="activeRowMenu = activeRowMenu === item.filename ? null : item.filename"
                    >
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                        <circle cx="12" cy="12" r="1.2"/><circle cx="12" cy="5" r="1.2"/><circle cx="12" cy="19" r="1.2"/>
                      </svg>
                    </button>
                    <div v-if="activeRowMenu === item.filename" class="mr-row-popup-menu" @click.stop="activeRowMenu = null">
                      <div class="mr-popup-item" @click="showInExplorer(item)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
                        <span>Показать в папке</span>
                      </div>
                      <div class="mr-popup-item danger" @click="promptDeleteMod(item)">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>
                        <span>Удалить</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- TAB: WORLDS -->
          <div v-else-if="currentTab === 'worlds'" class="mr-worlds-pane">
            <div class="mr-toolbar-row">
              <div class="mr-worlds-title">Сохранённые миры ({{ worldsList.length }})</div>
              <button class="mr-btn-secondary" @click="openWorldsFolder">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
                <span>Открыть папку saves</span>
              </button>
            </div>

            <div v-if="loadingWorlds" class="profile-loading">
              <span class="bar-mini"><i></i></span>
            </div>

            <div v-else-if="worldsList.length === 0" class="mr-empty-pane">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/></svg>
              <p>В этой сборке пока нет миров. Запустите игру и создайте мир!</p>
            </div>

            <div v-else class="mr-worlds-grid">
              <div v-for="w in worldsList" :key="w.folderName" class="mr-world-card">
                <div class="mr-world-icon">
                  <img v-if="w.iconBase64" :src="w.iconBase64" alt="">
                  <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/></svg>
                </div>
                <div class="mr-world-info">
                  <h4 class="mr-world-name">{{ w.name }}</h4>
                  <div class="mr-world-meta">
                    <span>{{ formatBytes(w.size) }}</span>
                    <span class="mr-meta-dot">•</span>
                    <span>{{ formatDate(w.lastPlayed) }}</span>
                  </div>
                </div>
                <button class="mr-btn-action-icon mr-btn-trash" title="Удалить мир" @click="promptDeleteWorld(w)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18 M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>
                </button>
              </div>
            </div>
          </div>

          <!-- TAB: SCREENSHOTS -->
          <div v-else-if="currentTab === 'screenshots'" class="mr-screenshots-pane">
            <div class="mr-toolbar-row">
              <div class="mr-worlds-title">Снимки экрана ({{ screenshotsList.length }})</div>
              <div class="mr-toolbar-buttons">
                <button class="mr-btn-secondary" @click="loadScreenshots" :title="t('profile.refreshLogs')">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 21h5v-5"/></svg>
                  <span>Обновить</span>
                </button>
                <button class="mr-btn-secondary" @click="openScreenshotsFolder">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
                  <span>Открыть папку screenshots</span>
                </button>
              </div>
            </div>

            <div v-if="loadingScreenshots" class="profile-loading">
              <span class="bar-mini"><i></i></span>
            </div>

            <div v-else-if="screenshotsList.length === 0" class="mr-empty-pane">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg>
              <p>{{ t('inst.screenshotsEmpty') }}</p>
              <span style="font-size: 12px; color: var(--muted);">{{ t('inst.screenshotsEmptyHint') }}</span>
            </div>

            <div v-else class="mr-screenshots-grid">
              <div
                v-for="s in screenshotsList"
                :key="s.filename"
                class="mr-screenshot-card"
                @click="activeScreenshot = s"
              >
                <div class="mr-screenshot-thumb-wrap">
                  <img :src="s.dataUrl" alt="" class="mr-screenshot-thumb" loading="lazy">
                  <div class="mr-screenshot-overlay">
                    <button class="mr-btn-thumb-action" :title="t('inst.copyScreenshot')" @click.stop="copyScreenshotImage(s)">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
                    </button>
                    <button class="mr-btn-thumb-action danger" :title="t('inst.deleteScreenshot')" @click.stop="promptDeleteScreenshot(s)">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18 M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>
                    </button>
                  </div>
                </div>
                <div class="mr-screenshot-info">
                  <span class="mr-screenshot-name" :title="s.filename">{{ s.filename }}</span>
                  <div class="mr-screenshot-meta">
                    <span>{{ formatBytes(s.size) }}</span>
                    <span class="mr-meta-dot">•</span>
                    <span>{{ formatDate(s.modTime) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- TAB: LOGS -->
          <div v-else-if="currentTab === 'logs'" class="mr-logs-pane">
            <div class="logs-toolbar">
              <div class="logs-title-row">
                <span class="logs-status-indicator" :class="{active: isPlaying, idle: !isPlaying}"></span>
                <span>latest.log</span>
                <span v-if="isPlaying" class="logs-live-badge">LIVE</span>
              </div>
              <div class="logs-btns">
                <button class="mr-btn-secondary logs-action-btn" @click="toggleLogSearch" :title="t('profile.searchLogs')" @keydown.enter.prevent>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
                  <span>{{ t('profile.searchLogs') }}</span>
                </button>
                <button class="mr-btn-secondary logs-action-btn" @click="loadLogs" :title="t('profile.refreshLogs')">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 21h5v-5"/></svg>
                  <span>{{ t('profile.refreshLogs') }}</span>
                </button>
                <button class="mr-btn-secondary logs-action-btn" @click="copyLogs" :disabled="!logsText" :title="t('profile.copyLogs')">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
                  <span>{{ t('profile.copyLogs') }}</span>
                </button>
              </div>
            </div>

            <div v-if="logSearchOpen" class="logs-search-row">
              <input class="txt-in logs-search-input" v-model="logSearchQuery" :placeholder="t('profile.searchLogsPh')" @keydown.esc="logSearchOpen = false" autofocus>
              <button class="mr-btn-primary logs-search-close" @click="logSearchOpen = false">✕</button>
            </div>

            <div v-if="loadingLogs" class="profile-loading">
              <span class="bar-mini"><i></i></span>
            </div>
            <template v-else-if="logsText">
              <pre v-if="!logSearchQuery" ref="logViewerRef" class="logs-viewer">{{ logsText }}</pre>
              <pre v-else class="logs-viewer"><span v-for="(seg, i) in logsText.split('\n').filter(l => l.toLowerCase().includes(logSearchQuery.toLowerCase())).slice(-500)" :key="i"><mark>{{ seg }}</mark>
</span></pre>
            </template>
            <div v-else class="profile-empty">
              <p>{{ t('profile.logEmpty') }}</p>
            </div>
          </div>

          <!-- TAB: CRASHES -->
          <div v-else-if="currentTab === 'crashes'" class="mr-crashes-pane">
            <div class="logs-toolbar">
              <div class="logs-title-row">
                <span>crash-reports/</span>
              </div>
              <div class="logs-btns">
                <button class="mr-btn-secondary logs-action-btn" @click="loadCrashes" :title="t('profile.refreshLogs')">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 21h5v-5"/></svg>
                  <span>{{ t('profile.refreshLogs') }}</span>
                </button>
                <button class="mr-btn-secondary logs-action-btn" @click="openCrashReportsFolder" :title="t('profile.openFolder')">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
                  <span>{{ t('profile.openFolder') }}</span>
                </button>
              </div>
            </div>

            <div v-if="loadingCrashes" class="profile-loading">
              <span class="bar-mini"><i></i></span>
            </div>
            <div v-else-if="crashReports.length === 0" class="profile-empty">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m8 2 1.88 1.88 M14.12 3.88 16 2 M9 7.13v-1a3.003 3.003 0 1 1 6 0v1"/><path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6v2"/><path d="M12 22v-2"/></svg>
              <p>{{ t('crash.empty') }}</p>
            </div>
            <div v-else class="crash-list">
              <details v-for="r in crashReports" :key="r.filename" class="crash-item">
                <summary>
                  <span class="crash-item-name">{{ r.filename }}</span>
                  <span class="crash-item-date">{{ formatDate(r.modTime) }}</span>
                  <span class="crash-item-summary" v-if="r.summary">{{ r.summary }}</span>
                </summary>
                <pre class="crash-content">{{ r.content }}</pre>
              </details>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty State if no instance exists -->
    <div v-else class="instances-empty-view">
      <p>{{ t('inst.empty') }}</p>
      <button class="btn-primary" @click="store.createInstanceModalOpen = true">+ {{ t('inst.createBtn') }}</button>
    </div>

    <!-- Confirm Delete Modal -->
    <div class="modal-root" v-if="instToDelete">
      <div class="modal-backdrop" @click="instToDelete = null"></div>
      <div class="modal-box">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('inst.deleteTitle') }}</h3>
          <button class="modal-close" @click="instToDelete = null">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>{{ t('inst.deleteConfirmText').replace('{name}', instToDelete.name) }}</p>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="instToDelete = null">{{ t('inst.deleteCancel') }}</button>
          <button class="btn-primary btn-danger" @click="confirmDeleteInst">{{ t('inst.deleteConfirm') }}</button>
        </div>
      </div>
    </div>

    <!-- Confirm Mod Delete Modal -->
    <div class="modal-root" v-if="modToDelete">
      <div class="modal-backdrop" @click="modToDelete = null"></div>
      <div class="modal-box">
        <div class="modal-header">
          <h3 class="modal-title">Удаление файла</h3>
          <button class="modal-close" @click="modToDelete = null">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>Вы уверены, что хотите удалить <b>«{{ modToDelete.name }}»</b> ({{ modToDelete.filename }})?</p>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="modToDelete = null">Отмена</button>
          <button class="btn-primary btn-danger" @click="confirmDeleteMod">Удалить</button>
        </div>
      </div>
    </div>

    <!-- Confirm Batch Delete Modal -->
    <div class="modal-root" v-if="batchDeleteModal">
      <div class="modal-backdrop" @click="batchDeleteModal = false"></div>
      <div class="modal-box">
        <div class="modal-header">
          <h3 class="modal-title">Удаление выбранных файлов</h3>
          <button class="modal-close" @click="batchDeleteModal = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>Вы уверены, что хотите удалить <b>{{ selectedCount }}</b> выбранных файлов?</p>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="batchDeleteModal = false">Отмена</button>
          <button class="btn-primary btn-danger" @click="confirmBatchDelete">Удалить</button>
        </div>
      </div>
    </div>

    <!-- Confirm World Delete Modal -->
    <div class="modal-root" v-if="worldToDelete">
      <div class="modal-backdrop" @click="worldToDelete = null"></div>
      <div class="modal-box">
        <div class="modal-header">
          <h3 class="modal-title">Удаление мира</h3>
          <button class="modal-close" @click="worldToDelete = null">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>Вы уверены, что хотите удалить мир <b>«{{ worldToDelete.name }}»</b>?</p>
          <p style="color: var(--muted); font-size: 12px; margin-top: 6px;">Все постройки и прогресс в этом мире будут безвозвратно удалены.</p>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="worldToDelete = null">Отмена</button>
          <button class="btn-primary btn-danger" @click="confirmDeleteWorld">Удалить мир</button>
        </div>
      </div>
    </div>

    <!-- Lightbox Screenshot Modal -->
    <div class="modal-root" v-if="activeScreenshot">
      <div class="modal-backdrop" @click="activeScreenshot = null"></div>
      <div class="modal-box screenshot-lightbox-box">
        <div class="modal-header">
          <h3 class="modal-title">{{ activeScreenshot.filename }}</h3>
          <div class="lightbox-head-actions">
            <button class="mr-btn-secondary" @click="copyScreenshotImage(activeScreenshot)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-mini-icon"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
              <span>{{ t('inst.copyScreenshot') }}</span>
            </button>
            <button class="modal-close" @click="activeScreenshot = null">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
            </button>
          </div>
        </div>
        <div class="screenshot-lightbox-body">
          <img :src="activeScreenshot.dataUrl" :alt="activeScreenshot.filename" class="lightbox-full-img">
        </div>
        <div class="modal-foot">
          <div class="lightbox-meta">
            <span>{{ formatBytes(activeScreenshot.size) }}</span>
            <span class="mr-meta-dot">•</span>
            <span>{{ formatDate(activeScreenshot.modTime) }}</span>
          </div>
          <button class="btn-sec btn-danger" @click="promptDeleteScreenshot(activeScreenshot)">{{ t('inst.deleteScreenshot') }}</button>
        </div>
      </div>
    </div>

    <!-- Confirm Screenshot Delete Modal -->
    <div class="modal-root" v-if="screenshotToDelete">
      <div class="modal-backdrop" @click="screenshotToDelete = null"></div>
      <div class="modal-box">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('inst.deleteScreenshot') }}</h3>
          <button class="modal-close" @click="screenshotToDelete = null">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>{{ t('inst.deleteScreenshotConfirm') }}</p>
          <p style="color: var(--muted); font-size: 12px; margin-top: 6px;">{{ screenshotToDelete.filename }}</p>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="screenshotToDelete = null">{{ t('inst.cancel') }}</button>
          <button class="btn-primary btn-danger" @click="confirmDeleteScreenshot">{{ t('inst.deleteScreenshot') }}</button>
        </div>
      </div>
    </div>

    <!-- Unified Instance Settings Modal (name/server + launch config + clone/export) -->
    <div class="modal-root" v-if="editSettingsOpen">
      <div class="modal-backdrop" @click="editSettingsOpen = false"></div>
      <div class="modal-box inst-settings-modal">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('inst.editSettings') }}</h3>
          <button class="modal-close" @click="editSettingsOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <!-- Tabs -->
        <div class="inst-settings-tabs">
          <button class="inst-settings-tab" :class="{active: settingsTab === 'general'}" @click="settingsTab = 'general'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
            <span>{{ t('inst.tabGeneral') || 'Основное' }}</span>
          </button>
          <button class="inst-settings-tab" :class="{active: settingsTab === 'launch'}" @click="settingsTab = 'launch'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="6 4 20 12 6 20 6 4"/></svg>
            <span>{{ t('inst.tabLaunch') || 'Запуск' }}</span>
          </button>
          <button class="inst-settings-tab" :class="{active: settingsTab === 'actions'}" @click="settingsTab = 'actions'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/></svg>
            <span>{{ t('inst.tabActions') || 'Действия' }}</span>
          </button>
        </div>

        <!-- TAB: General -->
        <div class="modal-body inst-settings-body" v-show="settingsTab === 'general'">
          <div class="fld-group">
            <label class="fld-label">{{ t('inst.name') }}</label>
            <input class="txt-in" v-model="editName" :placeholder="t('inst.namePh')" maxlength="40">
          </div>

          <div class="fld-group">
            <label class="fld-label">{{ t('inst.loader') }}</label>
            <div class="loader-row">
              <button
                v-for="ld in loaders"
                :key="ld.id"
                class="loader-tile"
                :class="{on: editLoader === ld.id}"
                @click="editLoader = ld.id"
              >
                <b>{{ t('loader.' + ld.id) }}</b>
              </button>
            </div>
          </div>

          <template v-if="editLoader !== 'vanilla'">
            <div class="fld-group">
              <label class="fld-label">{{ t('inst.loaderVersion') }}</label>
              <div v-if="editLoaderVerLoading" class="loader-ver-status">{{ t('inst.loaderLoading') }}</div>
              <div v-else-if="editLoaderVerErr" class="loader-ver-status err">{{ t('inst.loaderErr') }}</div>
              <ul v-else-if="editLoaderVerList.length" class="ver-pick-list short">
                <li
                  v-for="lv in editLoaderVerList.slice(0, 30)"
                  :key="lv.version"
                  :class="{sel: lv.version === editLoaderVersion}"
                  @click="editLoaderVersion = lv.version"
                >
                  {{ lv.version }}
                  <em v-if="lv.label" class="dd-tag" :class="{rel: lv.label === 'recommended', snap: lv.label === 'latest'}">
                    {{ t('inst.' + (lv.label === 'recommended' ? 'rec' : lv.label === 'latest' ? 'latest' : 'beta')) }}
                  </em>
                </li>
              </ul>
              <div v-else class="loader-ver-status">{{ t('home.noData') }}</div>
            </div>
          </template>

          <div class="fld-group">
            <label class="fld-label">{{ t('inst.version') }}</label>
            <input class="txt-in" v-model="editVerQuery" :placeholder="t('inst.search')">
            <ul class="ver-pick-list short">
              <li
                v-for="v in modalEditVersions"
                :key="v.id"
                :class="{sel: v.id === editVersion}"
                @click="editVersion = v.id"
              >
                {{ v.id }}
                <em class="dd-tag" :class="{rel: v.type === 'release', snap: v.type === 'snapshot'}">
                  {{ v.installed ? '✓ ' : '' }}{{ typeNames[v.type] || v.type }}
                </em>
              </li>
              <li v-if="!modalEditVersions.length" style="cursor:default;color:var(--muted)">{{ t('home.noData') }}</li>
            </ul>
          </div>

          <div class="fld-group">
            <label class="fld-label">{{ t('inst.serverAddress') }}</label>
            <input class="txt-in" v-model="editServer" :placeholder="t('inst.serverAddressPh')">
          </div>
        </div>

        <!-- TAB: Launch (RAM, Java, JVM args, window) -->
        <div class="modal-body inst-settings-body launch-cfg-body" v-show="settingsTab === 'launch'">
          <p class="launch-cfg-hint">{{ t('inst.launchConfigHint') }}</p>

          <div class="fld-group">
            <label class="fld-label">{{ t('inst.ramGb') }}</label>
            <div class="launch-cfg-ram-row">
              <input type="range" min="0" max="16" step="1" v-model.number="editRAMGB" class="launch-cfg-range">
              <span class="range-val">{{ editRAMGB === 0 ? t('inst.auto') : editRAMGB + ' ' + t('settings.gb') }}</span>
            </div>
          </div>

          <div class="fld-group">
            <label class="fld-label">Java</label>
            <div class="launch-cfg-java-row">
              <input class="txt-in full-w" v-model="editJavaPath" :placeholder="t('inst.javaAuto')">
              <button class="btn-sec" @click="browseInstanceJava">{{ t('settings.browse') }}</button>
              <button class="btn-sec" v-if="editJavaPath" @click="editJavaPath = ''">{{ t('settings.auto') }}</button>
            </div>
          </div>

          <div class="fld-group">
            <label class="fld-label">{{ t('settings.jvmPreset') }}</label>
            <select class="sel full-w" v-model="editJVMPreset">
              <option value="">{{ t('settings.jvmPresetGlobal') }}</option>
              <option value="aikar">{{ t('settings.jvmPresetAikar') }}</option>
              <option value="zgc">{{ t('settings.jvmPresetZGC') }}</option>
              <option value="shenandoah">{{ t('settings.jvmPresetShenandoah') }}</option>
              <option value="default">{{ t('settings.jvmPresetDefault') }}</option>
              <option value="none">{{ t('settings.jvmPresetNone') }}</option>
            </select>
          </div>

          <div class="fld-group">
            <label class="fld-label">{{ t('settings.extraJvmArgs') }}</label>
            <input class="txt-in full-w mono-in" v-model="editJVMArgs" :placeholder="t('settings.extraJvmArgsPh') || t('inst.jvmArgsPh')">
          </div>

          <div class="launch-cfg-toggle-card">
            <div class="launch-cfg-toggle-row">
              <div class="launch-cfg-toggle-info">
                <span class="launch-cfg-toggle-name">{{ t('settings.windowOverride') }}</span>
                <span class="launch-cfg-toggle-desc">{{ t('settings.windowOverrideDesc') }}</span>
              </div>
              <label class="switch"><input type="checkbox" v-model="editUseCustomWindow"><i></i></label>
            </div>

            <template v-if="editUseCustomWindow">
              <div class="launch-cfg-sub-panel">
                <div class="launch-cfg-toggle-row">
                  <div class="launch-cfg-toggle-info">
                    <span class="launch-cfg-toggle-name">{{ t('settings.fullscreen') }}</span>
                    <span class="launch-cfg-toggle-desc">{{ t('settings.fullscreenDesc') }}</span>
                  </div>
                  <label class="switch"><input type="checkbox" v-model="editFullscreen"><i></i></label>
                </div>
                <div class="launch-cfg-win-row" v-if="!editFullscreen">
                  <span class="launch-cfg-toggle-name" style="margin-right:auto">{{ t('settings.res') }}</span>
                  <input type="number" class="txt-in num-in" v-model.number="editWinW" min="320" max="7680" placeholder="854">
                  <span class="launch-cfg-mult">×</span>
                  <input type="number" class="txt-in num-in" v-model.number="editWinH" min="240" max="4320" placeholder="480">
                </div>
              </div>
            </template>
          </div>
        </div>

        <!-- TAB: Actions (clone, export) -->
        <div class="modal-body inst-settings-body" v-show="settingsTab === 'actions'">
          <button class="inst-action-row" @click="onCloneInstance(); editSettingsOpen = false">
            <div class="inst-action-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
            </div>
            <div class="inst-action-text">
              <span class="inst-action-name">{{ t('inst.clone') }}</span>
              <span class="inst-action-desc">{{ t('inst.cloneDesc') || 'Полная копия сборки с модами и настройками' }}</span>
            </div>
          </button>

          <button class="inst-action-row" :disabled="exportingInst" @click="onExportInstance(); editSettingsOpen = false">
            <div class="inst-action-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
            </div>
            <div class="inst-action-text">
              <span class="inst-action-name">{{ t('inst.export') }}</span>
              <span class="inst-action-desc">{{ t('inst.exportDesc') || 'Сохранить сборку в файл (.mrpack / .zip)' }}</span>
            </div>
          </button>

          <button class="inst-action-row" @click="openInstanceFolder">
            <div class="inst-action-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/></svg>
            </div>
            <div class="inst-action-text">
              <span class="inst-action-name">{{ t('profile.openFolder') }}</span>
              <span class="inst-action-desc">{{ t('inst.openFolderDesc') || 'Открыть папку сборки в проводнике' }}</span>
            </div>
          </button>
        </div>

        <div class="modal-foot">
          <button class="btn-sec" @click="editSettingsOpen = false">{{ t('inst.cancel') }}</button>
          <button class="btn-primary" :disabled="savingSettings" @click="saveInstanceSettings">
            {{ savingSettings ? 'Сохранение…' : t('inst.saveSettings') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Dependency Resolver Modal -->
    <div v-if="depModalOpen" class="modal-root">
      <div class="modal-backdrop" @click="depModalOpen = false"></div>
      <div class="modal-box dep-modal-box">
        <div class="modal-header">
          <div class="modal-title-group">
            <div class="modal-icon dep-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="m7.5 4.27 9 5.15"/>
                <path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/>
                <path d="m3.3 7 8.7 5 8.7-5"/>
                <path d="M12 22V12"/>
              </svg>
            </div>
            <div>
              <h2 class="modal-title">{{ t('deps.title') || 'Зависимости мода' }}</h2>
              <div class="modal-subtitle">
                {{ (t('deps.subtitle') || 'Для работы «{m}» требуются следующие библиотеки:').replace('{m}', depTargetHit?.title || '') }}
              </div>
            </div>
          </div>
          <button class="modal-close" @click="depModalOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="modal-body dep-modal-body">
          <div class="dep-list">
            <label
              v-for="dep in depList"
              :key="dep.projectId"
              class="dep-item-card"
              :class="{ checked: depSelectedUrls.includes(dep.downloadUrl) }"
            >
              <input
                type="checkbox"
                :value="dep.downloadUrl"
                v-model="depSelectedUrls"
                class="dep-checkbox"
              >
              <img v-if="dep.iconUrl" :src="dep.iconUrl" class="dep-item-icon" alt="icon">
              <div v-else class="dep-item-icon fallback">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect width="14" height="14" x="5" y="5" rx="2"/></svg>
              </div>
              <div class="dep-item-info">
                <div class="dep-item-title-row">
                  <span class="dep-item-title">{{ dep.projectTitle }}</span>
                  <span class="dep-badge" :class="dep.dependencyType">
                    {{ dep.dependencyType === 'required' ? (t('deps.required') || 'Обязательно') : (t('deps.optional') || 'Опционально') }}
                  </span>
                </div>
                <span class="dep-item-fn">{{ dep.fileName }}</span>
              </div>
            </label>
          </div>
        </div>

        <div class="modal-foot dep-modal-foot">
          <button class="btn-sec" @click="executeModInstall(depTargetHit, [])">
            {{ t('deps.onlyMod') || 'Только этот мод' }}
          </button>
          <button
            class="btn-primary"
            @click="executeModInstall(depTargetHit, depSelectedUrls)"
          >
            {{ t('deps.installAll') || 'Установить всё' }} ({{ depSelectedUrls.length + 1 }})
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
