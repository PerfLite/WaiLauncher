<script setup>
import {ref, computed, watch} from 'vue'
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
import SkinViewer3D from './SkinViewer3D.vue'

const mode = ref('list') // 'list' | 'add'
const addTab = ref('offline') // 'offline' | 'microsoft'
const hoveredAccId = ref(null)

const targetAcc = computed(() => {
  if (hoveredAccId.value) {
    const found = store.accounts.find(a => a.id === hoveredAccId.value)
    if (found) return found
  }
  return activeAccount.value || store.accounts[0] || null
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
  return `https://mc-heads.net/cape/${targetAcc.value.username}`
})

// Skin modal state
const skinModalOpen = ref(false)
const skinTargetAcc = ref(null)
const skinModelVariant = ref('classic')
const skinInputFile = ref(null)
const skinInputUrl = ref('')
const skinSaving = ref(false)

// Cape modal state
const capeModalOpen = ref(false)
const capeTargetAcc = ref(null)
const capeActiveTab = ref('gallery') // 'gallery' | 'official' | 'custom'
const presetCapesList = ref([])
const selectedPresetCape = ref(null)
const selectedOfficialCape = ref(null)
const customCapeFile = ref(null)
const customCapeUrl = ref('')
const capeSaving = ref(false)

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

async function openCapeModal(acc) {
  if (!acc) return
  capeTargetAcc.value = acc
  selectedPresetCape.value = null
  selectedOfficialCape.value = null
  customCapeFile.value = null
  customCapeUrl.value = ''

  if (!presetCapesList.value.length) {
    try {
      const list = await GetPresetCapes()
      presetCapesList.value = list || []
    } catch (_) {}
  }

  if (acc.type === 'microsoft' && acc.capes && acc.capes.length > 0) {
    capeActiveTab.value = 'official'
    selectedOfficialCape.value = acc.capes.find(c => c.state === 'ACTIVE') || acc.capes[0]
  } else {
    capeActiveTab.value = 'gallery'
    if (presetCapesList.value.length) {
      selectedPresetCape.value = presetCapesList.value.find(c => c.url === acc.capeUrl) || presetCapesList.value[0]
    }
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
      await SetAccountCape(capeTargetAcc.value.id, selectedPresetCape.value.url, '')
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

function closeModal() {
  if (msState.value === 'authorizing') {
    cancelMsAuth()
  }
  store.accountsModalOpen = false
  mode.value = 'list'
  resetForms()
}

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

    // Automatically copy code to clipboard
    try {
      if (navigator.clipboard) {
        await navigator.clipboard.writeText(resp.userCode)
        codeCopied.value = true
      }
    } catch (_) {}

    // Start background polling
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
      toast(t('accounts.ms.success'))
      setTimeout(() => {
        if (store.accountsModalOpen && msState.value === 'success') {
          mode.value = 'list'
          resetForms()
        }
      }, 2200)
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
  <div v-if="store.accountsModalOpen" class="modal-root">
    <div class="modal-backdrop" @click="closeModal"></div>

    <div class="modal-box acc-modal-box">
      <!-- Modal Header -->
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/>
              <circle cx="12" cy="7" r="4"/>
            </svg>
          </div>
          <div>
            <h2 class="modal-title">{{ t('accounts.title') }}</h2>
            <div class="modal-subtitle">
              {{ mode === 'list' ? t('accounts.subtitle') : t('accounts.addSubtitle') }}
            </div>
          </div>
        </div>

        <button class="modal-close" @click="closeModal" :title="t('tb.close')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
        </button>
      </div>

      <!-- Modal Body -->
      <div class="modal-body">

        <!-- VIEW: ACCOUNTS LIST -->
        <div v-if="mode === 'list'" class="acc-list-view">
          <div v-if="store.accounts.length === 0" class="acc-empty">
            <p>{{ t('accounts.empty') }}</p>
            <button class="btn-primary" @click="openAdd('offline')">{{ t('accounts.addBtn') }}</button>
          </div>

          <div v-else class="acc-list-layout">
            <div class="acc-list-col">
              <div class="acc-list-top">
                <span class="acc-count-label">{{ t('accounts.listLabel') }}</span>
                <button class="btn-primary-sm" @click="openAdd('offline')">
                  {{ t('accounts.addBtn') }}
                </button>
              </div>

              <div class="acc-cards-scroll">
                <div
                  v-for="acc in store.accounts"
                  :key="acc.id"
                  class="acc-card"
                  :class="{active: acc.id === store.activeAccountId, hovered: hoveredAccId === acc.id}"
                  @click="selectAcc(acc)"
                  @mouseenter="hoveredAccId = acc.id"
                  @mouseleave="hoveredAccId = null"
                >
                  <div class="acc-avatar-box">
                    <img :src="getAccountAvatar(acc)" @error="onAvatarError" alt="Head" class="acc-head-img">
                    <span class="acc-status-dot" :class="acc.type"></span>
                  </div>

                  <div class="acc-info">
                    <div class="acc-name-row">
                      <span class="acc-name">{{ acc.username }}</span>
                      <span v-if="acc.id === store.activeAccountId" class="acc-active-badge">
                        {{ t('accounts.active') }}
                      </span>
                    </div>
                    <div class="acc-type-row">
                      <span class="acc-type-pill" :class="acc.type">
                        {{ acc.type === 'microsoft' ? t('accounts.type.microsoft') : t('accounts.type.offline') }}
                      </span>
                      <span v-if="acc.type === 'microsoft' && acc.ownsGame" class="acc-owns-tag">
                        ✓ {{ t('accounts.ms.ownsGame') }}
                      </span>
                    </div>
                  </div>

                  <div class="acc-actions" @click.stop>
                    <button
                      v-if="acc.type === 'microsoft'"
                      class="acc-icon-btn"
                      @click="refreshAcc(acc)"
                      :title="t('accounts.refresh')"
                    >
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.2"/></svg>
                    </button>
                    <button
                      class="acc-icon-btn delete"
                      @click="promptDelete(acc)"
                      :title="t('accounts.delete')"
                    >
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- 3D Skin Viewer Column -->
            <div class="acc-skin-preview-col" v-if="targetAcc">
              <div class="acc-preview-header">
                <span class="acc-preview-title">{{ t('accounts.skinPreview') || '3D Скин персонажа' }}</span>
                <span class="acc-preview-user">{{ targetAcc?.username }}</span>
              </div>
              <SkinViewer3D
                :skin-url="targetSkinUrl"
                :cape-url="targetCapeUrl"
                :model="targetAcc?.skinModel || 'default'"
                :width="230"
                :height="260"
                :animation="'walk'"
                :show-controls="true"
              />
              <div class="acc-skin-actions-row">
                <button class="acc-skin-act-btn" @click="openSkinModal(targetAcc)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.38 3.46 16 2a4 4 0 0 1-8 0L3.62 3.46a2 2 0 0 0-1.34 2.23l.58 3.47a1 1 0 0 0 .99.84H6v10c0 1.1.9 2 2 2h8a2 2 0 0 0 2-2V10h2.15a1 1 0 0 0 .99-.84l.58-3.47a2 2 0 0 0-1.34-2.23z"/></svg>
                  <span>{{ t('skin.changeSkin') }}</span>
                </button>
                <button class="acc-skin-act-btn" @click="openCapeModal(targetAcc)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m4 4 8 16 8-16"/></svg>
                  <span>{{ t('skin.changeCape') }}</span>
                </button>
                <button v-if="targetCapeUrl" class="acc-skin-act-btn clear-cape" @click="removeCape(targetAcc)" :title="t('skin.removeCape')">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- VIEW: ADD ACCOUNT -->
        <div v-else class="acc-add-view">
          <div class="acc-add-top">
            <button class="acc-back-btn" @click="mode = 'list'">
              {{ t('accounts.backToList') }}
            </button>
          </div>

          <!-- Account Type Tabs -->
          <div class="acc-tabs">
            <button
              class="acc-tab"
              :class="{active: addTab === 'offline'}"
              @click="addTab = 'offline'"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
              <span>{{ t('accounts.tab.offline') }}</span>
            </button>
            <button
              class="acc-tab"
              :class="{active: addTab === 'microsoft'}"
              @click="addTab = 'microsoft'"
            >
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M0 0h11v11H0zM13 0h11v11H13zM0 13h11v11H0zM13 13h11v11H13z"/></svg>
              <span>{{ t('accounts.tab.microsoft') }}</span>
            </button>
          </div>

          <!-- TAB CONTENT: OFFLINE -->
          <div v-if="addTab === 'offline'" class="tab-pane">
            <div class="tab-intro">
              <h3>{{ t('accounts.offline.title') }}</h3>
              <p>{{ t('accounts.offline.desc') }}</p>
            </div>

            <form @submit.prevent="submitOffline" class="offline-form">
              <div class="offline-preview-row">
                <div class="offline-avatar-preview">
                  <img :src="offlineAvatarUrl" @error="onAvatarError" alt="Preview">
                </div>
                <div class="offline-input-col">
                  <label class="form-label">{{ t('accounts.offline.name') }}</label>
                  <input
                    type="text"
                    class="txt-in large"
                    v-model="offlineName"
                    :placeholder="t('accounts.offline.placeholder')"
                    maxlength="16"
                    autofocus
                  >
                  <span v-if="offlineError" class="form-error">{{ offlineError }}</span>
                </div>
              </div>

              <div class="form-actions">
                <button type="button" class="btn-sec" @click="mode = 'list'">{{ t('accounts.ms.cancel') }}</button>
                <button type="submit" class="btn-primary" :disabled="!offlineName.trim()">
                  {{ t('accounts.offline.submit') }}
                </button>
              </div>
            </form>
          </div>

          <!-- TAB CONTENT: MICROSOFT -->
          <div v-else class="tab-pane">
            <div class="tab-intro">
              <h3>{{ t('accounts.ms.title') }}</h3>
              <p>{{ t('accounts.ms.desc') }}</p>
            </div>

            <!-- Idle / Start state -->
            <div v-if="msState === 'idle'" class="ms-idle-box">
              <div class="ms-feature-list">
                <div class="ms-feat">
                  <span class="feat-icon">🛡️</span>
                  <div>
                    <strong>Безопасный OAuth2 Device Flow</strong>
                    <p>Пароль вводится исключительно на официальном сайте Microsoft</p>
                  </div>
                </div>
                <div class="ms-feat">
                  <span class="feat-icon">🌐</span>
                  <div>
                    <strong>Лицензионные сервера</strong>
                    <p>Полный доступ к Hypixel, 2b2t и любым официальным серверам</p>
                  </div>
                </div>
                <div class="ms-feat">
                  <span class="feat-icon">🎨</span>
                  <div>
                    <strong>Скины и плащи</strong>
                    <p>Автоматическая синхронизация официального скина и плащей</p>
                  </div>
                </div>
              </div>

              <button class="btn-primary ms-login-btn" @click="startMsAuth">
                <svg viewBox="0 0 24 24" fill="currentColor" style="width:18px;height:18px"><path d="M0 0h11v11H0zM13 0h11v11H13zM0 13h11v11H0zM13 13h11v11H13z"/></svg>
                <span>{{ t('accounts.ms.start') }}</span>
              </button>
            </div>

            <!-- Authorizing / Device Code Display -->
            <div v-else-if="msState === 'authorizing'" class="ms-auth-box">
              <div class="ms-step">
                <div class="ms-step-title">{{ t('accounts.ms.step1') }}</div>
                <div class="ms-code-box">
                  <span class="ms-code-text">{{ msCodeData?.userCode || '...' }}</span>
                  <button class="copy-code-btn" :class="{copied: codeCopied}" @click="copyUserCode">
                    <svg v-if="!codeCopied" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5"/></svg>
                    <span>{{ codeCopied ? t('accounts.ms.copied') : t('accounts.ms.copyCode') }}</span>
                  </button>
                </div>
              </div>

              <div class="ms-step">
                <div class="ms-step-title">{{ t('accounts.ms.step2') }}</div>
                <button class="btn-primary ms-open-btn" @click="openMsLink">
                  <span>{{ t('accounts.ms.openLink') }}</span>
                </button>
              </div>

              <div class="ms-waiting-bar">
                <div class="pulse-spinner"></div>
                <span>{{ t('accounts.ms.waiting') }}</span>
              </div>

              <div class="form-actions center">
                <button class="btn-sec" @click="cancelMsAuth">{{ t('accounts.ms.cancel') }}</button>
              </div>
            </div>

            <!-- Success State -->
            <div v-else-if="msState === 'success'" class="ms-success-box">
              <div class="success-icon">✓</div>
              <h3>{{ t('accounts.ms.success') }}</h3>
              <div class="ms-account-preview" v-if="msCreatedAccount">
                <img :src="getAccountAvatar(msCreatedAccount)" alt="Avatar">
                <span class="ms-acc-name">{{ msCreatedAccount.username }}</span>
              </div>
            </div>

            <!-- Error State -->
            <div v-else-if="msState === 'error'" class="ms-error-box">
              <div class="error-icon">✕</div>
              <h3>{{ t('accounts.err') }}</h3>
              <p class="error-msg">{{ msError }}</p>
              <div class="form-actions center">
                <button class="btn-primary" @click="startMsAuth">{{ t('accounts.ms.start') }}</button>
                <button class="btn-sec" @click="resetForms">{{ t('accounts.ms.cancel') }}</button>
              </div>
            </div>

          </div>
        </div>

      </div>
    </div>

    <!-- Custom In-App Delete Confirmation Modal -->
    <div v-if="accToDelete" class="confirm-modal-backdrop" @click="accToDelete = null">
      <div class="confirm-modal-box" @click.stop>
        <div class="confirm-icon-wrap">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><path d="M10 11v6M14 11v6"/></svg>
        </div>
        <h3 class="confirm-title">{{ t('accounts.deleteTitle') }}</h3>
        <p class="confirm-text">{{ t('accounts.deleteConfirmText', {name: accToDelete.username}) }}</p>
        <div class="confirm-actions">
          <button class="btn-sec" @click="accToDelete = null">{{ t('accounts.deleteCancel') }}</button>
          <button class="btn-danger" @click="confirmDeleteAcc">{{ t('accounts.deleteConfirm') }}</button>
        </div>
      </div>
    </div>

    <!-- Skin Customization Modal -->
    <div v-if="skinModalOpen" class="modal-root">
      <div class="modal-backdrop" @click="skinModalOpen = false"></div>
      <div class="modal-box skin-manage-modal">
        <div class="modal-header">
          <div class="modal-title-group">
            <div class="modal-icon skin-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.38 3.46 16 2a4 4 0 0 1-8 0L3.62 3.46a2 2 0 0 0-1.34 2.23l.58 3.47a1 1 0 0 0 .99.84H6v10c0 1.1.9 2 2 2h8a2 2 0 0 0 2-2V10h2.15a1 1 0 0 0 .99-.84l.58-3.47a2 2 0 0 0-1.34-2.23z"/></svg>
            </div>
            <div>
              <h2 class="modal-title">{{ t('skin.skinTitle') }}</h2>
              <div class="modal-subtitle">{{ skinTargetAcc?.username }} ({{ skinTargetAcc?.type === 'microsoft' ? t('accounts.type.microsoft') : t('accounts.type.offline') }})</div>
            </div>
          </div>
          <button class="modal-close" @click="skinModalOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="modal-body skin-modal-body">
          <div class="fld-group">
            <label class="fld-label">{{ t('skin.model') }}</label>
            <div class="skin-model-switch">
              <button
                class="skin-model-btn"
                :class="{ active: skinModelVariant === 'classic' }"
                @click="skinModelVariant = 'classic'"
              >
                <b>Classic</b>
                <span>{{ t('skin.modelClassic') }}</span>
              </button>
              <button
                class="skin-model-btn"
                :class="{ active: skinModelVariant === 'slim' }"
                @click="skinModelVariant = 'slim'"
              >
                <b>Slim</b>
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
            <input class="txt-in" v-model="skinInputUrl" :placeholder="t('skin.urlPh')">
          </div>

          <div v-if="skinTargetAcc?.type === 'microsoft'" class="skin-ms-note">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
            <span>{{ t('skin.msUploadNote') }}</span>
          </div>
        </div>

        <div class="modal-foot">
          <button class="btn-sec" @click="skinModalOpen = false">{{ t('inst.cancel') }}</button>
          <button class="btn-primary" :disabled="skinSaving || (!skinInputFile && !skinInputUrl.trim())" @click="applySkin">
            {{ skinSaving ? 'Сохранение…' : t('skin.applySkin') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Cape Customization Modal -->
    <div v-if="capeModalOpen" class="modal-root">
      <div class="modal-backdrop" @click="capeModalOpen = false"></div>
      <div class="modal-box cape-manage-modal">
        <div class="modal-header">
          <div class="modal-title-group">
            <div class="modal-icon cape-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m4 4 8 16 8-16"/></svg>
            </div>
            <div>
              <h2 class="modal-title">{{ t('skin.capeTitle') }}</h2>
              <div class="modal-subtitle">{{ capeTargetAcc?.username }}</div>
            </div>
          </div>
          <button class="modal-close" @click="capeModalOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <!-- Cape Tabs -->
        <div class="cape-tabs-bar">
          <button
            class="cape-tab-btn"
            :class="{ active: capeActiveTab === 'gallery' }"
            @click="capeActiveTab = 'gallery'"
          >
            {{ t('skin.tabGallery') }}
          </button>
          <button
            v-if="capeTargetAcc?.type === 'microsoft'"
            class="cape-tab-btn"
            :class="{ active: capeActiveTab === 'official' }"
            @click="capeActiveTab = 'official'"
          >
            {{ t('skin.tabOfficial') }} ({{ capeTargetAcc?.capes?.length || 0 }})
          </button>
          <button
            class="cape-tab-btn"
            :class="{ active: capeActiveTab === 'custom' }"
            @click="capeActiveTab = 'custom'"
          >
            {{ t('skin.tabCustom') }}
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
                <img :src="cape.url" alt="Cape" class="cape-preview-img">
              </div>
              <span class="cape-card-name">{{ cape.name }}</span>
              <span class="cape-card-tag">{{ cape.category }}</span>
            </div>
          </div>

          <!-- TAB: Official (Microsoft) -->
          <div v-else-if="capeActiveTab === 'official'" class="cape-official-list">
            <div v-if="!capeTargetAcc?.capes || !capeTargetAcc.capes.length" class="cape-empty-box">
              <p>{{ t('skin.noOfficialCapes') }}</p>
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
                  <img :src="cape.url" alt="Official Cape" class="cape-preview-img">
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
          <div style="flex: 1;"></div>
          <button class="btn-sec" @click="capeModalOpen = false">{{ t('inst.cancel') }}</button>
          <button class="btn-primary" :disabled="capeSaving" @click="applyCape">
            {{ capeSaving ? 'Сохранение…' : t('skin.applyCape') }}
          </button>
        </div>
      </div>
    </div>

  </div>
</template>
