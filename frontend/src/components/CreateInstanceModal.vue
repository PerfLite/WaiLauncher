<script setup>
import {computed, onMounted, ref, watch} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {
  CreateInstance,
  GetLoaderVersions,
  ImportInstanceDialog,
  DetectInstalledLaunchers,
  PickLauncherFolder,
  ScanCustomLauncherDir,
  ImportSelectedInstances
} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'

const currentStep = ref('menu') // 'menu' | 'custom' | 'import_launchers'
const createSearch = ref('')
const creating = ref(false)
const importingInst = ref(false)
const newName = ref('')
const newVersion = ref('')
const newLoader = ref('vanilla')
const verQuery = ref('')
const loaderVerList = ref([])
const loaderVer = ref('')
const loaderVerLoading = ref(false)
const loaderVerErr = ref(false)

/* Launcher Import state */
const detectedLaunchers = ref([])
const loadingLaunchers = ref(false)
const importSearch = ref('')
const selectedInstPaths = ref([])
const expandedLaunchers = ref({})
const importingSelected = ref(false)
const importProgress = ref({
  instance: '',
  status: '',
  percent: 0,
  index: 0,
  total: 0,
})

const loaders = [
  {id: 'vanilla'},
  {id: 'fabric'},
  {id: 'forge'},
  {id: 'neoforge'},
]

const typeNames = computed(() => ({
  release: t('type.release'),
  snapshot: t('type.snapshot'),
  old_beta: t('type.old_beta'),
  old_alpha: t('type.old_alpha'),
}))

const modalVersions = computed(() => {
  const q = verQuery.value.trim().toLowerCase()
  return store.versions.filter(v => {
    if (!store.settings.showSnapshots && v.type !== 'release') return false
    if (!q) return true
    return v.id.toLowerCase().includes(q)
  })
})

const filteredLaunchers = computed(() => {
  const q = importSearch.value.trim().toLowerCase()
  if (!q) return detectedLaunchers.value

  const result = []
  for (const l of detectedLaunchers.value) {
    const matchingInsts = (l.instances || []).filter(inst => {
      return (inst.name || '').toLowerCase().includes(q) ||
             (inst.versionId || '').toLowerCase().includes(q) ||
             (inst.loader || '').toLowerCase().includes(q)
    })
    if (matchingInsts.length > 0 || (l.name || '').toLowerCase().includes(q)) {
      result.push({
        ...l,
        instances: matchingInsts.length > 0 ? matchingInsts : l.instances
      })
    }
  }
  return result
})

onMounted(() => {
  EventsOn('import-progress', (data) => {
    if (data) {
      importProgress.value = {
        instance: data.instance || importProgress.value.instance,
        status: data.status || '',
        percent: typeof data.percent === 'number' ? data.percent : importProgress.value.percent,
        index: data.index || importProgress.value.index,
        total: data.total || importProgress.value.total,
      }
    }
  })
})

function closeCreate() {
  if (!creating.value && !importingInst.value && !importingSelected.value) {
    store.createInstanceModalOpen = false
    currentStep.value = 'menu'
    createSearch.value = ''
    selectedInstPaths.value = []
  }
}

function goToCatalogWithSearch() {
  store.page = 'mods'
  store.createInstanceModalOpen = false
  currentStep.value = 'menu'
  createSearch.value = ''
}

async function openLauncherImport() {
  currentStep.value = 'import_launchers'
  importSearch.value = ''
  selectedInstPaths.value = []
  loadingLaunchers.value = true
  try {
    const list = await DetectInstalledLaunchers()
    detectedLaunchers.value = list || []
    for (const l of detectedLaunchers.value) {
      expandedLaunchers.value[l.id] = true
    }
  } catch (e) {
    detectedLaunchers.value = []
  } finally {
    loadingLaunchers.value = false
  }
}

async function onAddLauncherPath() {
  try {
    const dir = await PickLauncherFolder()
    if (dir) {
      const dl = await ScanCustomLauncherDir(dir)
      if (dl && dl.instances && dl.instances.length) {
        detectedLaunchers.value.push(dl)
        expandedLaunchers.value[dl.id] = true
        toast(`Найдено сборок: ${dl.instances.length}`)
      } else {
        toast('В выбранной папке не найдены сборки', true)
      }
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  }
}

function isLauncherAllSelected(launcher) {
  if (!launcher.instances || !launcher.instances.length) return false
  return launcher.instances.every(i => selectedInstPaths.value.includes(i.path))
}

function toggleLauncherSelectAll(launcher) {
  if (!launcher.instances || !launcher.instances.length) return
  const allSel = isLauncherAllSelected(launcher)
  if (allSel) {
    selectedInstPaths.value = selectedInstPaths.value.filter(p => !launcher.instances.some(i => i.path === p))
  } else {
    for (const inst of launcher.instances) {
      if (!selectedInstPaths.value.includes(inst.path)) {
        selectedInstPaths.value.push(inst.path)
      }
    }
  }
}

async function doImportSelected() {
  if (!selectedInstPaths.value.length || importingSelected.value) return
  importingSelected.value = true
  importProgress.value = {
    instance: '',
    status: 'Подготовка к импорту...',
    percent: 5,
    index: 1,
    total: selectedInstPaths.value.length,
  }

  try {
    const imported = await ImportSelectedInstances(selectedInstPaths.value)
    if (imported && imported.length) {
      importProgress.value.percent = 100
      importProgress.value.status = '✓ Импорт успешно завершен!'
      
      // Update local reactive store immediately
      for (const ins of imported) {
        store.instances = [...store.instances.filter(i => i.id !== ins.id), ins]
      }
      const last = imported[imported.length - 1]
      store.settings.activeInstance = last.id
      store.settings.selectedVersion = last.versionId

      setTimeout(() => {
        store.page = 'home'
        toast(t('import.success') + imported.length)
        importingSelected.value = false
        closeCreate()
      }, 700)
    } else {
      toast(t('inst.err') + ' Не удалось импортировать сборки', true)
      importingSelected.value = false
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
    importingSelected.value = false
  }
}

async function fetchLoaderVersions(loader, mcVer) {
  if (loader === 'vanilla' || !mcVer) {
    loaderVerList.value = []
    loaderVer.value = ''
    loaderVerErr.value = false
    return
  }
  loaderVerLoading.value = true
  loaderVerErr.value = false
  try {
    const list = await GetLoaderVersions(loader, mcVer)
    loaderVerList.value = list || []
    if (list && list.length) {
      const rec = list.find(v => v.label === 'recommended')
      loaderVer.value = rec ? rec.version : list[0].version
    } else {
      loaderVer.value = ''
    }
  } catch (e) {
    loaderVerErr.value = true
    loaderVerList.value = []
    loaderVer.value = ''
  } finally {
    loaderVerLoading.value = false
  }
}

watch(() => store.createInstanceModalOpen, (v) => {
  if (v) {
    currentStep.value = 'menu'
    createSearch.value = ''
    newName.value = ''
    newVersion.value = store.settings.selectedVersion || store.latestRelease || (store.versions[0] ? store.versions[0].id : '')
    newLoader.value = 'vanilla'
    verQuery.value = ''
    loaderVer.value = ''
    loaderVerList.value = []
    selectedInstPaths.value = []
    importingSelected.value = false
  }
})

watch([newLoader, newVersion], ([ld, mc]) => {
  if (store.createInstanceModalOpen) {
    fetchLoaderVersions(ld, mc)
  }
})

async function doCreate() {
  if (!newVersion.value || creating.value) return
  creating.value = true
  try {
    const lv = newLoader.value === 'vanilla' ? '' : loaderVer.value
    const inst = await CreateInstance(newName.value, newVersion.value, newLoader.value, lv)
    if (inst) {
      if (!store.instances.some(i => i.id === inst.id)) {
        store.instances = [...store.instances, inst]
      }
      store.settings.activeInstance = inst.id
      store.settings.selectedVersion = inst.versionId
      store.selectedInstanceId = inst.id
      toast(t('inst.created') + inst.name)
      closeCreate()
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    creating.value = false
  }
}

async function onImportMrpackFile() {
  if (importingInst.value) return
  importingInst.value = true
  try {
    const inst = await ImportInstanceDialog()
    if (inst) {
      if (!store.instances.some(i => i.id === inst.id)) {
        store.instances = [...store.instances, inst]
      }
      store.selectedInstanceId = inst.id
      store.settings.activeInstance = inst.id
      store.settings.selectedVersion = inst.versionId
      toast(t('inst.importSuccess'))
      closeCreate()
    }
  } catch (e) {
    toast((t('inst.err') || 'Ошибка: ') + e, true)
  } finally {
    importingInst.value = false
  }
}
</script>

<template>
  <div class="modal-root" v-if="store.createInstanceModalOpen">
    <div class="modal-backdrop" @click="closeCreate"></div>
    <div class="modal-box create-modal" :class="{'menu-mode': currentStep === 'menu'}">
      
      <!-- 1. MENU STEP (Modrinth style) -->
      <template v-if="currentStep === 'menu'">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('inst.createTitle') }}</h3>
          <button class="modal-close" @click="closeCreate">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="modal-body" style="padding-top: 6px;">
          <!-- Top Search -->
          <div class="create-search-wrap">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <input
              type="text"
              v-model="createSearch"
              :placeholder="t('inst.searchModalPh')"
              @keyup.enter="goToCatalogWithSearch"
            >
          </div>

          <div class="create-divider">
            <span>{{ t('inst.or') }}</span>
          </div>

          <div class="create-section-title">{{ t('inst.selectType') }}</div>

          <div class="create-options-list">
            <!-- Option 1: Собственная настройка -->
            <div class="create-option-card" @click="currentStep = 'custom'">
              <div class="create-option-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
                </svg>
              </div>
              <div class="create-option-info">
                <div class="create-option-title">{{ t('inst.customSetup') }}</div>
                <div class="create-option-desc">{{ t('inst.customSetupDesc') }}</div>
              </div>
            </div>

            <!-- Option 2: Загрузить модпак (.mrpack / .zip) -->
            <div class="create-option-card" @click="onImportMrpackFile">
              <div class="create-option-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                  <polyline points="17 8 12 3 7 8"/>
                  <line x1="12" y1="3" x2="12" y2="15"/>
                </svg>
              </div>
              <div class="create-option-info">
                <div class="create-option-title">{{ t('inst.uploadModpack') }}</div>
                <div class="create-option-desc">{{ t('inst.uploadModpackDesc') }}</div>
              </div>
            </div>

            <!-- Option 3: Импортировать сборку (Scan third-party launchers) -->
            <div class="create-option-card" @click="openLauncherImport">
              <div class="create-option-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                  <rect x="3" y="3" width="18" height="18" rx="2"/>
                  <path d="M12 8v8m-4-4 4 4 4-4"/>
                </svg>
              </div>
              <div class="create-option-info">
                <div class="create-option-title">{{ t('inst.importOther') }}</div>
                <div class="create-option-desc">{{ t('inst.importOtherDesc') }}</div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- 2. LAUNCHER IMPORT STEP (Modrinth style) -->
      <template v-else-if="currentStep === 'import_launchers'">
        <!-- Progress Overlay during Active Import -->
        <div v-if="importingSelected" class="import-progress-view">
          <div class="import-progress-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="7 10 12 15 17 10"/>
              <line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
          </div>
          <h3 class="import-progress-title">{{ importProgress.instance ? `Импорт: ${importProgress.instance}` : t('import.importing') }}</h3>
          <div class="import-progress-counter">Сборка {{ importProgress.index }} из {{ importProgress.total }}</div>
          
          <div class="import-progress-bar-wrap">
            <div class="import-progress-bar-fill" :style="{width: `${importProgress.percent}%`}"></div>
          </div>
          <div class="import-progress-footer-info">
            <span class="import-progress-status">{{ importProgress.status }}</span>
            <span class="import-progress-percent">{{ importProgress.percent }}%</span>
          </div>
        </div>

        <!-- Normal Selection View -->
        <template v-else>
          <div class="modal-header">
            <div class="modal-title-group">
              <button class="btn-back-link" @click="currentStep = 'menu'" :title="t('inst.back')">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="m15 18-6-6 6-6"/></svg>
              </button>
              <h3 class="modal-title">{{ t('import.title') }}</h3>
            </div>
            <button class="modal-close" @click="closeCreate">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
            </button>
          </div>

          <div class="modal-body">
            <div class="fld-label">{{ t('import.launcherBuilds') }}</div>

            <div class="import-search-wrap">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
              </svg>
              <input
                type="text"
                v-model="importSearch"
                :placeholder="t('import.searchPh')"
              >
              <button v-if="importSearch" class="clear-btn" @click="importSearch = ''">✕</button>
            </div>

            <div class="import-launchers-list">
              <div
                v-for="l in filteredLaunchers"
                :key="l.id"
                class="import-launcher-group"
              >
                <div class="import-launcher-header" @click="expandedLaunchers[l.id] = !expandedLaunchers[l.id]">
                  <svg
                    class="import-chevron"
                    :class="{collapsed: !expandedLaunchers[l.id]}"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2.5"
                    stroke-linecap="round"
                  >
                    <path d="m6 9 6 6 6-6"/>
                  </svg>
                  <label class="import-check-wrap" @click.stop>
                    <input
                      type="checkbox"
                      :checked="isLauncherAllSelected(l)"
                      @change="toggleLauncherSelectAll(l)"
                    >
                  </label>
                  <span class="import-launcher-name">{{ l.name }}</span>
                  <span class="import-launcher-count">{{ l.instances ? l.instances.length : 0 }}</span>
                </div>

                <div v-show="expandedLaunchers[l.id]" class="import-instances-list">
                  <label
                    v-for="inst in l.instances"
                    :key="inst.path"
                    class="import-instance-item"
                  >
                    <input
                      type="checkbox"
                      :value="inst.path"
                      v-model="selectedInstPaths"
                    >
                    <div class="import-inst-icon">
                      <img v-if="inst.icon" :src="inst.icon" alt="">
                      <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
                      </svg>
                    </div>
                    <div class="import-inst-info">
                      <div class="import-inst-name">{{ inst.name }}</div>
                      <div class="import-inst-sub">{{ inst.versionId }} • {{ t('loader.' + inst.loader) }}{{ inst.loaderVersion ? ' ' + inst.loaderVersion : '' }}</div>
                    </div>
                  </label>
                </div>
              </div>

              <div v-if="loadingLaunchers" class="import-loading">
                <span class="bar-mini"><i></i></span>
              </div>

              <div v-else-if="!filteredLaunchers.length" class="import-empty-hint">
                <p>{{ t('import.noLaunchersFound') }}</p>
              </div>
            </div>

            <button class="btn-add-path" @click="onAddLauncherPath">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 5v14M5 12h14"/>
              </svg>
              <span>{{ t('import.addLauncherPath') }}</span>
            </button>
          </div>

          <div class="modal-foot">
            <button class="btn-sec" @click="currentStep = 'menu'">{{ t('inst.back') }}</button>
            <div style="flex: 1;"></div>
            <button
              class="btn-primary btn-import"
              :disabled="!selectedInstPaths.length || importingSelected"
              @click="doImportSelected"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
              </svg>
              <span>{{ t('import.btn') + (selectedInstPaths.length ? ' (' + selectedInstPaths.length + ')' : '') }}</span>
            </button>
          </div>
        </template>
      </template>

      <!-- 3. CUSTOM STEP (Form) -->
      <template v-else-if="currentStep === 'custom'">
        <div class="modal-header">
          <div class="modal-title-group">
            <button class="btn-back-link" @click="currentStep = 'menu'" :title="t('inst.back')">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="m15 18-6-6 6-6"/></svg>
            </button>
            <h3 class="modal-title">{{ t('inst.customSetup') }}</h3>
          </div>
          <button class="modal-close" @click="closeCreate">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="modal-body">
          <label class="fld-label">{{ t('inst.name') }}</label>
          <input class="txt-in" v-model="newName" :placeholder="t('inst.namePh')" maxlength="40">

          <label class="fld-label">{{ t('inst.loader') }}</label>
          <div class="loader-row">
            <button
              v-for="ld in loaders"
              :key="ld.id"
              class="loader-tile"
              :class="{on: newLoader === ld.id}"
              @click="newLoader = ld.id"
            >
              <b>{{ t('loader.' + ld.id) }}</b>
            </button>
          </div>

          <template v-if="newLoader !== 'vanilla'">
            <label class="fld-label">{{ t('inst.loaderVersion') }}</label>
            <div v-if="loaderVerLoading" class="loader-ver-status">{{ t('inst.loaderLoading') }}</div>
            <div v-else-if="loaderVerErr" class="loader-ver-status err">{{ t('inst.loaderErr') }}</div>
            <ul v-else-if="loaderVerList.length" class="ver-pick-list short">
              <li
                v-for="lv in loaderVerList.slice(0, 30)"
                :key="lv.version"
                :class="{sel: lv.version === loaderVer}"
                @click="loaderVer = lv.version"
              >
                {{ lv.version }}
                <em v-if="lv.label" class="dd-tag" :class="{rel: lv.label === 'recommended', snap: lv.label === 'latest'}">
                  {{ t('inst.' + (lv.label === 'recommended' ? 'rec' : lv.label === 'latest' ? 'latest' : 'beta')) }}
                </em>
              </li>
            </ul>
            <div v-else class="loader-ver-status">{{ t('home.noData') }}</div>
          </template>

          <label class="fld-label">{{ t('inst.version') }}</label>
          <input class="txt-in" v-model="verQuery" :placeholder="t('inst.search')">
          <ul class="ver-pick-list">
            <li
              v-for="v in modalVersions"
              :key="v.id"
              :class="{sel: v.id === newVersion}"
              @click="newVersion = v.id"
            >
              {{ v.id }}
              <em class="dd-tag" :class="{rel: v.type === 'release', snap: v.type === 'snapshot'}">
                {{ v.installed ? '✓ ' : '' }}{{ typeNames[v.type] || v.type }}
              </em>
            </li>
            <li v-if="!modalVersions.length" style="cursor:default;color:var(--muted)">{{ t('home.noData') }}</li>
          </ul>
        </div>

        <div class="modal-foot">
          <button class="btn-sec" @click="currentStep = 'menu'">{{ t('inst.back') }}</button>
          <div style="flex: 1;"></div>
          <button class="btn-sec" @click="closeCreate">{{ t('inst.cancel') }}</button>
          <button class="btn-create" :disabled="!newVersion || creating" @click="doCreate">
            {{ creating ? '…' : t('inst.createBtn') }}
          </button>
        </div>
      </template>

    </div>
  </div>
</template>
