<script setup>
import {ref, watch, computed} from 'vue'
import {t} from '../i18n'
import {toast} from '../store'
import {GetModAvailableVersions, ChangeModVersion} from '../../wailsjs/go/main/App'
import {renderMarkdown, handleMarkdownClick} from '../utils/markdown'

const props = defineProps({
  show: {type: Boolean, default: false},
  instanceId: {type: String, required: true},
  item: {type: Object, default: null}
})

const emit = defineEmits(['close', 'version-changed'])

const loading = ref(false)
const error = ref('')
const changeInfo = ref(null)
const downloading = ref(false)
const searchQuery = ref('')
const showIncompatible = ref(false)
const selectedVer = ref(null)

watch(() => props.show, async (isOpen) => {
  if (isOpen && props.item) {
    searchQuery.value = ''
    showIncompatible.value = false
    selectedVer.value = null
    loadVersions()
  } else {
    changeInfo.value = null
    error.value = ''
    downloading.value = false
    selectedVer.value = null
  }
})

async function loadVersions() {
  if (!props.item || !props.instanceId) return
  loading.value = true
  error.value = ''
  changeInfo.value = null
  selectedVer.value = null
  try {
    const res = await GetModAvailableVersions(props.instanceId, props.item.filename)
    changeInfo.value = res
    if (res && res.versions && res.versions.length > 0) {
      // Pick current version if available, otherwise first compatible, otherwise first
      const cur = res.versions.find(v => v.isCurrent)
      const firstComp = res.versions.find(v => v.isCompatible)
      selectedVer.value = cur || firstComp || res.versions[0]
      // If no compatible versions found at all, auto-show incompatible
      if (!res.versions.some(v => v.isCompatible)) {
        showIncompatible.value = true
      }
    }
  } catch (e) {
    error.value = String(e || 'Не удалось загрузить список версий мода')
  } finally {
    loading.value = false
  }
}

const displayedVersions = computed(() => {
  if (!changeInfo.value || !changeInfo.value.versions) return []
  let list = changeInfo.value.versions
  if (!showIncompatible.value) {
    list = list.filter(v => v.isCompatible)
  }
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(v =>
      (v.name && v.name.toLowerCase().includes(q)) ||
      (v.versionNumber && v.versionNumber.toLowerCase().includes(q)) ||
      (v.filename && v.filename.toLowerCase().includes(q)) ||
      (v.gameVersions && v.gameVersions.some(gv => gv.toLowerCase().includes(q)))
    )
  }
  return list
})

async function doChangeVersion() {
  if (!props.instanceId || !changeInfo.value || !selectedVer.value || downloading.value) return
  if (selectedVer.value.isCurrent) return

  downloading.value = true
  const ver = selectedVer.value
  try {
    const updated = await ChangeModVersion(
      props.instanceId,
      changeInfo.value.filename,
      ver.downloadUrl,
      ver.filename,
      changeInfo.value.source,
      changeInfo.value.projectId || changeInfo.value.projectID || ''
    )
    toast(t('mods.versionChanged') || `Версия мода изменена на ${ver.versionNumber || ver.name}`)
    emit('version-changed', {
      oldFilename: changeInfo.value.filename,
      item: updated
    })
    emit('close')
  } catch (e) {
    toast((t('inst.err') || 'Ошибка смены версии: ') + e, true)
  } finally {
    downloading.value = false
  }
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  try {
    const d = new Date(dateStr)
    return d.toLocaleDateString(undefined, {year: 'numeric', month: 'short', day: 'numeric'})
  } catch {
    return dateStr
  }
}

function getVersionInitial(type) {
  const t = (type || 'release').toLowerCase()
  if (t === 'beta') return 'B'
  if (t === 'alpha') return 'A'
  return 'P' // Production / Release
}
</script>

<template>
  <div class="modal-root" v-if="show">
    <div class="modal-backdrop" @click="!downloading && emit('close')"></div>
    <div class="modal-box mr-change-version-modal">
      <!-- Modal Header -->
      <div class="mr-cv-header">
        <div class="mr-cv-header-left">
          <div class="mr-cv-avatar">
            <img v-if="item?.iconUrl" :src="item.iconUrl" alt="">
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
            </svg>
          </div>
          <div class="mr-cv-title-box">
            <h3 class="mr-cv-title">Смена версии</h3>
            <span class="mr-cv-mod-name">{{ changeInfo?.modName || item?.name }}</span>
          </div>
        </div>
        <button class="modal-close" :disabled="downloading" @click="emit('close')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
        </button>
      </div>

      <!-- Modal Body (Two-Column Layout) -->
      <div class="mr-cv-body">
        <!-- Loading State -->
        <div v-if="loading" class="mr-cv-loading">
          <span class="bar-mini"><i></i></span>
          <p>{{ t('mods.loadingVersions') || 'Поиск и загрузка доступных версий…' }}</p>
        </div>

        <!-- Error State -->
        <div v-else-if="error" class="mr-cv-error">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <p>{{ error }}</p>
          <button class="btn-sec btn-retry" @click="loadVersions">Попробовать снова</button>
        </div>

        <!-- Content Layout -->
        <template v-else>
          <!-- Left Column: Versions List -->
          <div class="mr-cv-left-col">
            <div class="mr-cv-search-wrap">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-cv-search-icon">
                <circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>
              </svg>
              <input
                type="text"
                v-model="searchQuery"
                placeholder="Поиск версии..."
                class="mr-cv-search-input"
              >
              <button v-if="searchQuery" class="mr-cv-search-clear" @click="searchQuery = ''">✕</button>
            </div>

            <div class="mr-cv-list-scroll">
              <div v-if="displayedVersions.length === 0" class="mr-cv-list-empty">
                Версии не найдены
              </div>
              <div
                v-for="ver in displayedVersions"
                :key="ver.id"
                class="mr-cv-list-item"
                :class="{
                  selected: selectedVer?.id === ver.id,
                  current: ver.isCurrent,
                  incompatible: !ver.isCompatible
                }"
                @click="selectedVer = ver"
              >
                <!-- Initial Badge: P / B / A -->
                <div class="mr-cv-badge" :class="(ver.versionType || 'release').toLowerCase()">
                  {{ getVersionInitial(ver.versionType) }}
                </div>

                <!-- Version Title / Number -->
                <span class="mr-cv-item-title">{{ ver.versionNumber || ver.name }}</span>

                <!-- Current Tag -->
                <span v-if="ver.isCurrent" class="mr-cv-current-tag">Текущая</span>
              </div>
            </div>

            <!-- Toggle Incompatible Versions -->
            <button
              class="mr-cv-toggle-incompatible"
              @click="showIncompatible = !showIncompatible"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/>
                <circle cx="12" cy="12" r="3"/>
              </svg>
              <span>{{ showIncompatible ? 'Скрыть несовместимые' : 'Показать несовместимые' }}</span>
            </button>
          </div>

          <!-- Right Column: Changelog & Details -->
          <div class="mr-cv-right-col">
            <template v-if="selectedVer">
              <!-- Version Top Header -->
              <div class="mr-cv-ver-header">
                <div class="mr-cv-ver-header-top">
                  <div class="mr-cv-ver-title-group">
                    <h2 class="mr-cv-ver-number">{{ selectedVer.versionNumber || selectedVer.name }}</h2>
                    <span class="mr-cv-type-badge" :class="(selectedVer.versionType || 'release').toLowerCase()">
                      {{ selectedVer.versionType || 'Release' }}
                    </span>
                  </div>
                  <span class="mr-cv-ver-date" v-if="selectedVer.datePublished">
                    {{ formatDate(selectedVer.datePublished) }}
                  </span>
                </div>

                <div class="mr-cv-ver-header-sub">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-cv-sub-icon">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                    <polyline points="14 2 14 8 20 8"/>
                    <line x1="16" y1="13" x2="8" y2="13"/>
                    <line x1="16" y1="17" x2="8" y2="17"/>
                  </svg>
                  <span>Список изменений</span>
                  <span class="mr-cv-dot">•</span>
                  <span v-if="selectedVer.loaders?.length" class="mr-cv-env-tag">
                    {{ selectedVer.loaders.join(', ') }}
                  </span>
                  <span v-if="selectedVer.gameVersions?.length" class="mr-cv-env-tag">
                    {{ selectedVer.gameVersions.join(', ') }}
                  </span>
                </div>
              </div>

              <!-- Changelog Content -->
              <div class="mr-cv-changelog-wrap">
                <div
                  v-if="selectedVer.changelog"
                  class="mr-cv-changelog-body md-body"
                  v-html="renderMarkdown(selectedVer.changelog)"
                  @click="handleMarkdownClick"
                ></div>
                <div v-else class="mr-cv-no-changelog">
                  <p>Автор мода не оставил подробного описания изменений для этой версии.</p>
                  <p class="mr-cv-file-hint">Файл: <code>{{ selectedVer.filename }}</code></p>
                </div>
              </div>
            </template>
            <div v-else class="mr-cv-no-selection">
              <p>Выберите версию из списка слева</p>
            </div>
          </div>
        </template>
      </div>

      <!-- Modal Footer -->
      <div class="mr-cv-footer">
        <!-- Warning alert matching Modrinth App -->
        <div class="mr-cv-warn-banner">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-cv-warn-icon">
            <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/>
            <line x1="12" y1="9" x2="12" y2="13"/>
            <line x1="12" y1="17" x2="12.01" y2="17"/>
          </svg>
          <span class="mr-cv-warn-text">
            Обновление может повредить вашу сборку. Сперва ознакомьтесь со списком изменений версии и создайте резервную копию.
          </span>
        </div>

        <!-- Action Buttons -->
        <div class="mr-cv-actions">
          <button class="btn-sec mr-cv-btn-cancel" :disabled="downloading" @click="emit('close')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M18 6 6 18M6 6l12 12"/></svg>
            <span>Отмена</span>
          </button>
          <button
            class="btn-primary mr-cv-btn-change"
            :disabled="!selectedVer || selectedVer.isCurrent || downloading"
            @click="doChangeVersion"
          >
            <template v-if="downloading">
              <span class="btn-spinner"></span>
              <span>Загрузка…</span>
            </template>
            <template v-else-if="selectedVer?.isCurrent">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M20 6 9 17l-5-5"/></svg>
              <span>Текущая версия</span>
            </template>
            <template v-else>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3"/></svg>
              <span>Сменить на {{ selectedVer?.versionNumber || selectedVer?.name || 'выбранную' }}</span>
            </template>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
