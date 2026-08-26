<script setup>
import {ref, computed, watch, onMounted, nextTick} from 'vue'
import {store, activeAccount, getAccountAvatar, reloadAccounts, switchActiveAccount, deleteAccount, toast} from '../store'
import {t} from '../i18n'
import {
  AddOfflineAccount,
  StartMicrosoftAuth,
  PollMicrosoftAuth,
  CancelMicrosoftAuth,
  RefreshAccount,
  PickSkinFile,
  PickCapeFile,
  SetAccountSkin,
  SetAccountCape,
  ClearAccountCape,
  GetPresetCapes,
  OpenURL
} from '../../wailsjs/go/main/App'
import skinFallback from '../assets/skin.png'
import SkinViewer3D from '../components/SkinViewer3D.vue'

const mode = ref('list') // 'list' | 'add'
const addTab = ref('offline') // 'offline' | 'microsoft'
const selectedAccId = ref(null)

const currentActiveAcc = computed(() => activeAccount())

const targetAcc = computed(() => {
  if (selectedAccId.value) {
    const found = store.accounts.find(a => a.id === selectedAccId.value)
    if (found) return found
  }
  return currentActiveAcc.value || store.accounts[0] || null
})

const targetSkinUrl = computed(() => {
  if (!targetAcc.value) return skinFallback
  if (targetAcc.value.skinUrl) return targetAcc.value.skinUrl
  if (targetAcc.value.type === 'microsoft' && targetAcc.value.uuid) {
    return `https://mc-heads.net/skin/${targetAcc.value.uuid}`
  }
  return `https://mc-heads.net/skin/${targetAcc.value.username}`
})

const targetCapeUrl = computed(() => {
  if (!targetAcc.value) return ''
  if (targetAcc.value.capeUrl) return targetAcc.value.capeUrl
  return ''
})

// Auto-refresh Microsoft account profile capes on mount or selection
const refreshingCapes = ref(false)
async function refreshActiveCapes() {
  if (!targetAcc.value || targetAcc.value.type !== 'microsoft' || refreshingCapes.value) return
  refreshingCapes.value = true
  try {
    const updated = await RefreshAccount(targetAcc.value.id)
    if (updated) {
      await reloadAccounts()
    }
  } catch (e) {
    console.warn('Auto-refresh account error:', e)
  } finally {
    refreshingCapes.value = false
  }
}

watch(targetAcc, (newAcc, oldAcc) => {
  if (newAcc && (!oldAcc || newAcc.id !== oldAcc.id) && newAcc.type === 'microsoft') {
    refreshActiveCapes()
  }
})

onMounted(async () => {
  if (targetAcc.value && targetAcc.value.type === 'microsoft') {
    refreshActiveCapes()
  }
  loadPresetCapes()
})

// Preset Capes
const presetCapesList = ref([])
async function loadPresetCapes() {
  try {
    const list = await GetPresetCapes()
    presetCapesList.value = list || []
  } catch (_) {}
}

// Canvas helper to render 2D front of cape texture crisp
function renderCapeCanvas(el, src) {
  if (!el || !src) return
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => {
    const ctx = el.getContext('2d')
    ctx.imageSmoothingEnabled = false
    ctx.clearRect(0, 0, el.width, el.height)
    // Front face of standard 64x32 Minecraft cape is at (1, 1), w: 10, h: 16
    ctx.drawImage(img, 1, 1, 10, 16, 0, 0, el.width, el.height)
  }
  img.src = src
}

const vCapeCanvas = {
  mounted(el, binding) {
    renderCapeCanvas(el, binding.value)
  },
  updated(el, binding) {
    if (binding.value !== binding.oldValue) {
      renderCapeCanvas(el, binding.value)
    }
  }
}

// Skin modal state
const skinModalOpen = ref(false)
const skinTargetAcc = ref(null)
const skinModelVariant = ref('classic')
const skinInputFile = ref(null)
const skinInputUrl = ref('')
const skinSaving = ref(false)

function openSkinModal(acc) {
  if (!acc) return
  skinTargetAcc.value = acc
  skinModelVariant.value = acc.skinModel || 'classic'
  skinInputFile.value = null
  skinInputUrl.value = ''
  skinModalOpen.value = true
}

async function onPickSkinFile() {
  try {
    const res = await PickSkinFile()
    if (res) {
      skinInputFile.value = res
    }
  } catch (e) {
    toast(t('accounts.err') + e, true)
  }
}

async function applySkin() {
  if (!skinTargetAcc.value || skinSaving.value) return
  const val = skinInputFile.value ? (skinInputFile.value.dataUrl || skinInputFile.value.filePath) : skinInputUrl.value.trim()
  if (!val && !skinInputFile.value) {
    toast('Пожалуйста, выберите файл скина или укажите ссылку', true)
    return
  }
  skinSaving.value = true
  try {
    await SetAccountSkin(skinTargetAcc.value.id, val, skinModelVariant.value)
    await reloadAccounts()
    toast(t('skin.skinSuccess'))
    skinModalOpen.value = false
  } catch (e) {
    toast(t('accounts.err') + e, true)
  } finally {
    skinSaving.value = false
  }
}

// Cape modal state
const capeModalOpen = ref(false)
const capeTargetAcc = ref(null)
const capeActiveTab = ref('gallery') // 'gallery' | 'official' | 'custom'
const selectedPresetCape = ref(null)
const selectedOfficialCape = ref(null)
const customCapeFile = ref(null)
const customCapeUrl = ref('')
const capeSaving = ref(false)

async function openCapeModal(acc) {
  if (!acc) return
  capeTargetAcc.value = acc
  selectedPresetCape.value = null
  selectedOfficialCape.value = null
  customCapeFile.value = null
  customCapeUrl.value = ''

  if (!presetCapesList.value.length) {
    await loadPresetCapes()
  }

  if (acc.type === 'microsoft') {
    if (!acc.capes || !acc.capes.length) {
      refreshActiveCapes()
    }
    if (acc.capes && acc.capes.length > 0) {
      capeActiveTab.value = 'official'
      selectedOfficialCape.value = acc.capes.find(c => c.state === 'ACTIVE') || acc.capes[0]
    } else {
      capeActiveTab.value = 'gallery'
    }
  } else {
    capeActiveTab.value = 'gallery'
  }

  if (presetCapesList.value.length) {
    selectedPresetCape.value = presetCapesList.value.find(c => c.url === acc.capeUrl || c.dataUrl === acc.capeUrl) || presetCapesList.value[0]
  }

  capeModalOpen.value = true
}

async function onPickCapeFile() {
  try {
    const res = await PickCapeFile()
    if (res) {
      customCapeFile.value = res
    }
  } catch (e) {
    toast(t('accounts.err') + e, true)
  }
}

async function applyCape() {
  if (!capeTargetAcc.value || capeSaving.value) return
  capeSaving.value = true
  try {
    if (capeActiveTab.value === 'official' && selectedOfficialCape.value) {
      await SetAccountCape(capeTargetAcc.value.id, selectedOfficialCape.value.url, selectedOfficialCape.value.id)
    } else if (capeActiveTab.value === 'gallery' && selectedPresetCape.value) {
      const urlToSet = selectedPresetCape.value.dataUrl || selectedPresetCape.value.url
      await SetAccountCape(capeTargetAcc.value.id, urlToSet, '')
    } else if (capeActiveTab.value === 'custom') {
      const val = customCapeFile.value ? (customCapeFile.value.dataUrl || customCapeFile.value.filePath) : customCapeUrl.value.trim()
      if (!val) {
        toast('Пожалуйста, выберите файл плаща или укажите ссылку', true)
        capeSaving.value = false
        return
      }
      await SetAccountCape(capeTargetAcc.value.id, val, '')
    }
    await reloadAccounts()
    toast(t('skin.capeSuccess'))
    capeModalOpen.value = false
  } catch (e) {
    toast(t('accounts.err') + e, true)
  } finally {
    capeSaving.value = false
  }
}

async function removeCape(acc) {
  if (!acc) return
  try {
    await ClearAccountCape(acc.id)
    await reloadAccounts()
    toast(t('skin.capeRemoved'))
    capeModalOpen.value = false
  } catch (e) {
    toast(t('accounts.err') + e, true)
  }
}

// Offline form state
const offlineName = ref('')
const offlineError = ref('')
const offlineAvatarUrl = computed(() => {
  const name = offlineName.value.trim()
  if (!name) return skinFallback
  return `https://mc-heads.net/avatar/${name}/64`
})

// Microsoft device code state
const msState = ref('idle') // 'idle' | 'authorizing' | 'success' | 'error'
const msError = ref('')
const msCodeData = ref(null)
const codeCopied = ref(false)
const msCreatedAccount = ref(null)

function resetForms() {
  offlineName.value = ''
  offlineError.value = ''
  msState.value = 'idle'
  msError.value = ''
  msCodeData.value = null
  codeCopied.value = false
  msCreatedAccount.value = null
}

function openAdd(tab = 'offline') {
  addTab.value = tab
  mode.value = 'add'
  resetForms()
}

function cancelAdd() {
  if (msState.value === 'authorizing') {
    cancelMsAuth()
  }
  mode.value = 'list'
  resetForms()
}

// ---- Offline Flow ----
async function submitOffline() {
  const name = offlineName.value.trim()
  if (!name || name.length > 16 || !/^[a-zA-Z0-9_]+$/.test(name)) {
    offlineError.value = t('accounts.offline.err')
    return
  }
  offlineError.value = ''

  try {
    const acc = await AddOfflineAccount(name)
    await reloadAccounts()
    if (acc) selectedAccId.value = acc.id
    toast(t('accounts.added') + name)
    mode.value = 'list'
    resetForms()
  } catch (e) {
    offlineError.value = String(e)
  }
}

// ---- Microsoft Flow ----
async function startMsAuth() {
  msState.value = 'authorizing'
  msError.value = ''
  codeCopied.value = false
  msCodeData.value = null

  try {
    const resp = await StartMicrosoftAuth()
    if (!resp || !resp.deviceCode) {
      throw new Error('Failed to get device authorization code')
    }
    msCodeData.value = resp

    try {
      if (navigator.clipboard) {
        await navigator.clipboard.writeText(resp.userCode)
        codeCopied.value = true
      }
    } catch (_) {}

    pollMsAuth(resp.deviceCode, resp.interval || 5)
  } catch (e) {
    msState.value = 'error'
    msError.value = String(e)
  }
}

async function pollMsAuth(deviceCode, interval) {
  try {
    const acc = await PollMicrosoftAuth(deviceCode, interval)
    if (acc) {
      msCreatedAccount.value = acc
      msState.value = 'success'
      await reloadAccounts()
      selectedAccId.value = acc.id
      toast(t('accounts.ms.success'))
      setTimeout(() => {
        if (msState.value === 'success') {
          mode.value = 'list'
          resetForms()
        }
      }, 2000)
    }
  } catch (e) {
    if (msState.value === 'authorizing') {
      msState.value = 'error'
      msError.value = String(e)
    }
  }
}

async function copyUserCode() {
  if (msCodeData.value?.userCode) {
    try {
      await navigator.clipboard.writeText(msCodeData.value.userCode)
      codeCopied.value = true
      toast(t('accounts.ms.copied'))
      setTimeout(() => { codeCopied.value = false }, 2500)
    } catch (_) {}
  }
}

function openMsLink() {
  const uri = msCodeData.value?.verificationUri || 'https://microsoft.com/link'
  OpenURL(uri).catch(() => {})
}

function cancelMsAuth() {
  CancelMicrosoftAuth().catch(() => {})
  msState.value = 'idle'
  msError.value = ''
  msCodeData.value = null
}

// ---- Accounts Actions ----
const accToDelete = ref(null)

async function selectAcc(acc) {
  selectedAccId.value = acc.id
  try {
    await switchActiveAccount(acc.id)
    toast(t('accounts.selected') + acc.username)
  } catch (e) {
    toast(t('accounts.err') + e, true)
  }
}

function promptDelete(acc) {
  accToDelete.value = acc
}

async function confirmDeleteAcc() {
  if (!accToDelete.value) return
  const target = accToDelete.value
  accToDelete.value = null
  try {
    await deleteAccount(target.id)
    if (selectedAccId.value === target.id) {
      selectedAccId.value = store.activeAccountId
    }
    toast(t('accounts.deleted'))
  } catch (e) {
    toast(t('accounts.err') + e, true)
  }
}

async function refreshAcc(acc) {
  try {
    await RefreshAccount(acc.id)
    await reloadAccounts()
    toast(t('accounts.refreshed'))
  } catch (e) {
    toast(t('accounts.err') + e, true)
  }
}

function onAvatarError(e) {
  e.target.src = skinFallback
}
</script>

<template>
  <section class="page page-accounts">
    <!-- Top Header -->
    <div class="accounts-page-head">
      <div class="accounts-head-info">
        <h2 class="accounts-page-title">{{ t('accounts.title') }}</h2>
        <p class="accounts-page-sub">{{ t('accounts.subtitle') }}</p>
      </div>

      <div class="accounts-head-actions">
        <button v-if="mode === 'list'" class="btn-primary" @click="openAdd('offline')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M12 5v14M5 12h14"/></svg>
          <span>{{ t('accounts.addBtn') }}</span>
        </button>
        <button v-else class="btn-sec" @click="cancelAdd">
          <span>{{ t('inst.cancel') }}</span>
        </button>
      </div>
    </div>

    <!-- MAIN CONTENT VIEW -->
    <div class="accounts-page-content">
      <!-- VIEW 1: ACCOUNTS LIST & STUDIO -->
      <div v-if="mode === 'list'" class="accounts-studio-layout">
        
        <!-- LEFT COLUMN: Accounts Cards List -->
        <div class="accounts-left-col">
          <div class="accounts-sec-header">
            <span class="accounts-sec-title">{{ t('accounts.listLabel') }}</span>
            <span class="accounts-count-badge">{{ store.accounts.length }}</span>
          </div>

          <div v-if="store.accounts.length === 0" class="acc-empty-card">
            <p>{{ t('accounts.empty') }}</p>
            <button class="btn-primary-sm" @click="openAdd('offline')">{{ t('accounts.addBtn') }}</button>
          </div>

          <div v-else class="accounts-cards-scroll">
            <div
              v-for="acc in store.accounts"
              :key="acc.id"
              class="acc-card-item"
              :class="{
                active: store.activeAccountId === acc.id,
                selected: targetAcc?.id === acc.id
              }"
              @click="selectedAccId = acc.id"
            >
              <div class="acc-card-avatar">
                <img :src="getAccountAvatar(acc)" @error="onAvatarError" alt="Avatar">
                <span class="acc-type-pip" :class="acc.type"></span>
              </div>

              <div class="acc-card-details">
                <div class="acc-card-name-row">
                  <span class="acc-card-name">{{ acc.username }}</span>
                  <span v-if="store.activeAccountId === acc.id" class="acc-active-pill">{{ t('accounts.active') }}</span>
                </div>
                <div class="acc-card-tags-row">
                  <span class="acc-tag" :class="acc.type">
                    {{ acc.type === 'microsoft' ? 'MICROSOFT' : 'OFFLINE' }}
                  </span>
                  <span v-if="acc.ownsGame" class="acc-license-tag">✓ Лицензия</span>
                </div>
              </div>

              <div class="acc-card-actions" @click.stop>
                <button
                  v-if="store.activeAccountId !== acc.id"
                  class="acc-btn-action choose"
                  :title="t('accounts.selectHint')"
                  @click="selectAcc(acc)"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                </button>

                <button
                  v-if="acc.type === 'microsoft'"
                  class="acc-btn-action refresh"
                  :title="t('accounts.refreshHint')"
                  @click="refreshAcc(acc)"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-1.19"/></svg>
                </button>

                <button
                  class="acc-btn-action delete"
                  :title="t('accounts.deleteHint')"
                  @click="promptDelete(acc)"
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- RIGHT COLUMN: 3D Character Studio & Customization -->
        <div class="accounts-right-col" v-if="targetAcc">
          <div class="character-studio-card">
            
            <div class="studio-header">
              <div class="studio-acc-info">
                <h3 class="studio-acc-name">{{ targetAcc.username }}</h3>
                <span class="studio-acc-type" :class="targetAcc.type">
                  {{ targetAcc.type === 'microsoft' ? 'Официальный Microsoft аккаунт' : 'Оффлайн аккаунт' }}
                </span>
              </div>

              <div class="studio-active-state">
                <button
                  v-if="store.activeAccountId !== targetAcc.id"
                  class="btn-primary-sm"
                  @click="selectAcc(targetAcc)"
                >
                  Сделать активным
                </button>
                <span v-else class="studio-badge-active">★ Текущий выбранный</span>
              </div>
            </div>

            <div class="studio-body-layout">
              <!-- 3D Preview Box -->
              <div class="studio-3d-box">
                <SkinViewer3D
                  :skin-url="targetSkinUrl"
                  :cape-url="targetCapeUrl"
                  :model="targetAcc.skinModel || 'classic'"
                  :width="280"
                  :height="360"
                  :show-controls="true"
                  :auto-rotate="false"
                />
              </div>

              <!-- Customization Panels -->
              <div class="studio-controls-col">
                
                <!-- Skin Section -->
                <div class="studio-section-card">
                  <div class="studio-sec-title-row">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.38 3.46 16 2a4 4 0 0 1-8 0L3.62 3.46a2 2 0 0 0-1.34 2.23l.58 3.47a1 1 0 0 0 .99.84H6v10c0 1.1.9 2 2 2h8a2 2 0 0 0 2-2V10h2.15a1 1 0 0 0 .99-.84l.58-3.47a2 2 0 0 0-1.34-2.23z"/></svg>
                    <h4>Внешний вид (Скин)</h4>
                  </div>
                  
                  <div class="studio-meta-grid">
                    <div class="studio-meta-item">
                      <span class="sm-label">Модель рук:</span>
                      <span class="sm-val">{{ (targetAcc.skinModel || 'classic') === 'slim' ? 'Тонкие (Slim / Alex, 3px)' : 'Классические (Classic / Steve, 4px)' }}</span>
                    </div>
                  </div>

                  <div class="studio-actions-row">
                    <button class="btn-sec full-w" @click="openSkinModal(targetAcc)">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
                      <span>Сменить скин</span>
                    </button>
                  </div>
                </div>

                <!-- Cape Section -->
                <div class="studio-section-card">
                  <div class="studio-sec-title-row">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 3h12l4 6-10 12L2 9l4-6z"/></svg>
                    <h4>Плащ персонажа</h4>
                  </div>

                  <div class="studio-meta-grid">
                    <div class="studio-meta-item">
                      <span class="sm-label">Текущий плащ:</span>
                      <span class="sm-val" v-if="targetAcc.capeUrl">Надет</span>
                      <span class="sm-val muted" v-else>Не надет</span>
                    </div>
                  </div>

                  <div class="studio-actions-row">
                    <button class="btn-sec full-w" @click="openCapeModal(targetAcc)">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
                      <span>Выбрать плащ</span>
                    </button>
                    <button v-if="targetAcc.capeUrl" class="btn-sec-danger" @click="removeCape(targetAcc)" title="Снять плащ">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/></svg>
                    </button>
                  </div>
                </div>

              </div>
            </div>

          </div>
        </div>

      </div>

      <!-- VIEW 2: ADD ACCOUNT FORM -->
      <div v-else class="accounts-add-view">
        <div class="acc-add-card">
          
          <div class="acc-type-tabs">
            <button
              class="acc-type-tab"
              :class="{active: addTab === 'offline'}"
              @click="addTab = 'offline'"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="7" r="4"/><path d="M5.5 21a8.38 8.38 0 0 1 13 0"/></svg>
              <span>{{ t('accounts.offline.tab') }}</span>
            </button>
            <button
              class="acc-type-tab"
              :class="{active: addTab === 'microsoft'}"
              @click="addTab = 'microsoft'"
            >
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M11.4 24H0V12.6h11.4V24zM24 24H12.6V12.6H24V24zM11.4 11.4H0V0h11.4v11.4zm12.6 0H12.6V0H24v11.4z"/></svg>
              <span>{{ t('accounts.ms.tab') }}</span>
            </button>
          </div>

          <!-- SUB-TAB: Offline Form -->
          <div v-if="addTab === 'offline'" class="acc-tab-pane">
            <div class="offline-preview-row">
              <div class="offline-avatar">
                <img :src="offlineAvatarUrl" @error="onAvatarError" alt="Avatar">
              </div>
              <div class="offline-intro">
                <h4>{{ t('accounts.offline.title') }}</h4>
                <p>{{ t('accounts.offline.desc') }}</p>
              </div>
            </div>

            <div class="fld-group">
              <label class="fld-label">{{ t('accounts.offline.name') }}</label>
              <input
                type="text"
                class="txt-in full-w"
                v-model="offlineName"
                :placeholder="t('accounts.offline.ph')"
                maxlength="16"
                @keyup.enter="submitOffline"
                autofocus
              >
              <span class="fld-hint">{{ t('accounts.offline.hint') }}</span>
            </div>

            <p v-if="offlineError" class="acc-error-msg">{{ offlineError }}</p>

            <div class="acc-form-foot">
              <button class="btn-sec" @click="cancelAdd">{{ t('inst.cancel') }}</button>
              <button class="btn-primary" @click="submitOffline" :disabled="!offlineName.trim()">
                {{ t('accounts.offline.create') }}
              </button>
            </div>
          </div>

          <!-- SUB-TAB: Microsoft OAuth Device Code -->
          <div v-else class="acc-tab-pane">
            <div v-if="msState === 'idle'" class="ms-intro-pane">
              <div class="ms-icon-big">
                <svg viewBox="0 0 24 24" fill="currentColor"><path d="M11.4 24H0V12.6h11.4V24zM24 24H12.6V12.6H24V24zM11.4 11.4H0V0h11.4v11.4zm12.6 0H12.6V0H24v11.4z"/></svg>
              </div>
              <h4>{{ t('accounts.ms.title') }}</h4>
              <p>{{ t('accounts.ms.desc') }}</p>

              <div class="acc-form-foot center">
                <button class="btn-sec" @click="cancelAdd">{{ t('inst.cancel') }}</button>
                <button class="btn-primary btn-ms-start" @click="startMsAuth">
                  <span>{{ t('accounts.ms.start') }}</span>
                </button>
              </div>
            </div>

            <div v-else-if="msState === 'authorizing'" class="ms-auth-pane">
              <div class="ms-code-card">
                <span class="ms-code-label">{{ t('accounts.ms.codeLabel') }}</span>
                <div class="ms-code-row" @click="copyUserCode" :title="t('accounts.ms.copy')">
                  <span class="ms-code-value">{{ msCodeData?.userCode || '••••••••' }}</span>
                  <button class="ms-btn-copy" :class="{copied: codeCopied}">
                    <svg v-if="!codeCopied" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                  </button>
                </div>
              </div>

              <div class="ms-actions-bar">
                <button class="btn-primary" @click="openMsLink">
                  <span>{{ t('accounts.ms.openLink') }}</span>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6M15 3h6v6M10 14L21 3"/></svg>
                </button>
                <button class="btn-sec" @click="cancelMsAuth">{{ t('inst.cancel') }}</button>
              </div>

              <div class="ms-polling-status">
                <div class="skin-spinner mini"></div>
                <span>{{ t('accounts.ms.waiting') }}</span>
              </div>
            </div>

            <div v-else-if="msState === 'success'" class="ms-success-pane">
              <div class="ms-success-icon">✓</div>
              <h4>{{ t('accounts.ms.success') }}</h4>
              <p v-if="msCreatedAccount">Добро пожаловать, <strong>{{ msCreatedAccount.username }}</strong>!</p>
            </div>

            <div v-else-if="msState === 'error'" class="ms-error-pane">
              <div class="ms-err-icon">✕</div>
              <h4>{{ t('accounts.ms.errTitle') }}</h4>
              <p class="acc-error-msg">{{ msError }}</p>
              <div class="acc-form-foot center">
                <button class="btn-sec" @click="cancelAdd">{{ t('inst.cancel') }}</button>
                <button class="btn-primary" @click="startMsAuth">{{ t('accounts.ms.retry') }}</button>
              </div>
            </div>
          </div>

        </div>
      </div>
    </div>

    <!-- SKIN CUSTOMIZATION MODAL -->
    <div v-if="skinModalOpen" class="modal-root">
      <div class="modal-backdrop" @click="skinModalOpen = false"></div>
      <div class="modal-box skin-modal-box">
        <div class="modal-header">
          <div class="modal-title-group">
            <h3 class="modal-title">{{ t('skin.modalTitle') }}</h3>
            <span class="modal-subtitle">{{ skinTargetAcc?.username }}</span>
          </div>
          <button class="modal-close" @click="skinModalOpen = false">✕</button>
        </div>

        <div class="modal-body skin-modal-body">
          <div class="fld-group">
            <label class="fld-label">{{ t('skin.modelVariant') }}</label>
            <div class="skin-model-pills">
              <button
                class="skin-model-pill"
                :class="{ active: skinModelVariant === 'classic' }"
                @click="skinModelVariant = 'classic'"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="12" height="16" rx="2"/></svg>
                <span>{{ t('skin.modelClassic') }}</span>
              </button>
              <button
                class="skin-model-pill"
                :class="{ active: skinModelVariant === 'slim' }"
                @click="skinModelVariant = 'slim'"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="7" y="4" width="10" height="16" rx="2"/></svg>
                <span>{{ t('skin.modelSlim') }}</span>
              </button>
            </div>
          </div>

          <div class="fld-group">
            <label class="fld-label">Файл скина (.png)</label>
            <div class="skin-pick-file-box">
              <button class="btn-sec full-w pick-btn" @click="onPickSkinFile">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
                <span>{{ skinInputFile ? skinInputFile.fileName : t('skin.pickFile') }}</span>
              </button>
              <div v-if="skinInputFile" class="skin-file-selected-badge">
                <span>✓ Выбран файл: {{ skinInputFile.fileName }}</span>
                <button class="skin-clear-file-btn" @click="skinInputFile = null">✕</button>
              </div>
            </div>
          </div>

          <div class="fld-group" v-if="!skinInputFile">
            <label class="fld-label">{{ t('skin.orUrl') }}</label>
            <input class="txt-in" v-model="skinInputUrl" placeholder="https://example.com/skin.png">
          </div>
        </div>

        <div class="modal-foot">
          <button class="btn-sec" @click="skinModalOpen = false">{{ t('inst.cancel') }}</button>
          <button class="btn-primary" :disabled="skinSaving" @click="applySkin">
            <span v-if="skinSaving">Сохранение…</span>
            <span v-else>{{ t('skin.apply') }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- CAPE CUSTOMIZATION MODAL -->
    <div v-if="capeModalOpen" class="modal-root">
      <div class="modal-backdrop" @click="capeModalOpen = false"></div>
      <div class="modal-box cape-modal-box">
        <div class="modal-header">
          <div class="modal-title-group">
            <h3 class="modal-title">{{ t('skin.capeModalTitle') }}</h3>
            <span class="modal-subtitle">{{ capeTargetAcc?.username }}</span>
          </div>
          <button class="modal-close" @click="capeModalOpen = false">✕</button>
        </div>

        <div class="cape-nav-tabs">
          <button
            class="cape-nav-tab"
            :class="{ active: capeActiveTab === 'gallery' }"
            @click="capeActiveTab = 'gallery'"
          >
            {{ t('skin.presetCapes') }}
          </button>
          <button
            v-if="capeTargetAcc?.type === 'microsoft'"
            class="cape-nav-tab"
            :class="{ active: capeActiveTab === 'official' }"
            @click="capeActiveTab = 'official'"
          >
            {{ t('skin.officialCapes') }} ({{ capeTargetAcc.capes ? capeTargetAcc.capes.length : 0 }})
          </button>
          <button
            class="cape-nav-tab"
            :class="{ active: capeActiveTab === 'custom' }"
            @click="capeActiveTab = 'custom'"
          >
            {{ t('skin.customCape') }}
          </button>
        </div>

        <div class="modal-body cape-modal-body">
          <!-- TAB: Gallery -->
          <div v-if="capeActiveTab === 'gallery'" class="cape-gallery-grid">
            <div
              v-for="cape in presetCapesList"
              :key="cape.id"
              class="cape-card"
              :class="{ selected: selectedPresetCape?.id === cape.id }"
              @click="selectedPresetCape = cape"
            >
              <div class="cape-img-wrap">
                <canvas
                  class="cape-canvas-preview"
                  width="40"
                  height="64"
                  v-cape-canvas="cape.dataUrl || cape.url"
                ></canvas>
              </div>
              <span class="cape-card-name">{{ cape.name }}</span>
              <span class="cape-card-tag">{{ cape.category }}</span>
            </div>
          </div>

          <!-- TAB: Official (Microsoft) -->
          <div v-else-if="capeActiveTab === 'official'" class="cape-official-list">
            <div v-if="!capeTargetAcc?.capes || !capeTargetAcc.capes.length" class="cape-empty-box">
              <p>{{ t('skin.noOfficialCapes') }}</p>
              <button class="btn-sec-sm" @click="refreshActiveCapes">
                <span>Обновить данные с Mojang</span>
              </button>
            </div>
            <div v-else class="cape-gallery-grid">
              <div
                v-for="cape in capeTargetAcc.capes"
                :key="cape.id"
                class="cape-card"
                :class="{ selected: selectedOfficialCape?.id === cape.id }"
                @click="selectedOfficialCape = cape"
              >
                <div class="cape-img-wrap">
                  <canvas
                    class="cape-canvas-preview"
                    width="40"
                    height="64"
                    v-cape-canvas="cape.dataUrl || cape.url"
                  ></canvas>
                </div>
                <span class="cape-card-name">{{ cape.alias || cape.id }}</span>
                <span class="cape-card-tag" :class="{ active: cape.state === 'ACTIVE' }">
                  {{ cape.state === 'ACTIVE' ? 'Активен' : 'Доступен' }}
                </span>
              </div>
            </div>
          </div>

          <!-- TAB: Custom -->
          <div v-else class="cape-custom-pane">
            <div class="fld-group">
              <label class="fld-label">Файл плаща (.png)</label>
              <div class="skin-pick-file-box">
                <button class="btn-sec full-w pick-btn" @click="onPickCapeFile">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
                  <span>{{ customCapeFile ? customCapeFile.fileName : t('skin.pickFile') }}</span>
                </button>
                <div v-if="customCapeFile" class="skin-file-selected-badge">
                  <span>✓ Выбран файл: {{ customCapeFile.fileName }}</span>
                  <button class="skin-clear-file-btn" @click="customCapeFile = null">✕</button>
                </div>
              </div>
            </div>

            <div class="fld-group" v-if="!customCapeFile">
              <label class="fld-label">{{ t('skin.orUrl') }}</label>
              <input class="txt-in" v-model="customCapeUrl" placeholder="https://example.com/cape.png">
            </div>
          </div>
        </div>

        <div class="modal-foot cape-modal-foot">
          <button v-if="capeTargetAcc?.capeUrl" class="btn-sec clear-btn" @click="removeCape(capeTargetAcc)">
            {{ t('skin.removeCape') }}
          </button>
          <div class="right-actions">
            <button class="btn-sec" @click="capeModalOpen = false">{{ t('inst.cancel') }}</button>
            <button class="btn-primary" :disabled="capeSaving" @click="applyCape">
              <span v-if="capeSaving">Применение…</span>
              <span v-else>{{ t('skin.applyCape') }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- DELETE ACCOUNT CONFIRMATION MODAL -->
    <div v-if="accToDelete" class="modal-root">
      <div class="modal-backdrop" @click="accToDelete = null"></div>
      <div class="modal-box confirm-del-box">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('accounts.delConfirm') }}</h3>
          <button class="modal-close" @click="accToDelete = null">✕</button>
        </div>
        <div class="modal-body">
          <p>Вы действительно хотите удалить аккаунт <strong>{{ accToDelete.username }}</strong>?</p>
        </div>
        <div class="modal-foot">
          <button class="btn-sec" @click="accToDelete = null">{{ t('inst.cancel') }}</button>
          <button class="btn-danger" @click="confirmDeleteAcc">{{ t('accounts.deleteBtn') }}</button>
        </div>
      </div>
    </div>

  </section>
</template>
