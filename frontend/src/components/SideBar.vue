<script setup>
import {computed, ref, watch} from 'vue'
import {store, activeAccount, getAccountAvatar} from '../store'
import {t} from '../i18n'
import {SetActiveInstance} from '../../wailsjs/go/main/App'
import skinFallback from '../assets/skin.png'

const instancesExpanded = ref(true)

watch(() => store.page, (page) => {
  if (page === 'instances') {
    instancesExpanded.value = true
  }
})

const currentAcc = computed(() => activeAccount())
const avatarUrl = computed(() => getAccountAvatar(currentAcc.value))

function fmtDur(sec) {
  if (!sec || sec <= 0) return '0 ч'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h > 0) return m > 0 ? `${h} ч ${m} м` : `${h} ч`
  return `${m} м`
}

const totalPlayTime = computed(() => {
  let total = 0
  let today = 0
  const day = new Date()
  const key = `${day.getFullYear()}-${String(day.getMonth() + 1).padStart(2, '0')}-${String(day.getDate()).padStart(2, '0')}`
  for (const ins of store.instances) {
    total += ins.playTime || 0
    if (ins.lastPlayDay === key) today += ins.playTimeToday || 0
  }
  return {total, today}
})

function onAvatarError(e) {
  e.target.src = skinFallback
}

function onNavClick(item) {
  if (item.id === 'instances') {
    if (store.page === 'instances') {
      instancesExpanded.value = !instancesExpanded.value
    } else {
      store.page = 'instances'
      instancesExpanded.value = true
    }
  } else {
    store.page = item.id
  }
}

async function selectInstance(inst) {
  store.selectedInstanceId = inst.id
  store.page = 'instances'
  if (inst.id !== store.settings.activeInstance) {
    try {
      await SetActiveInstance(inst.id)
      store.settings.activeInstance = inst.id
      store.settings.selectedVersion = inst.versionId
    } catch (e) {}
  }
}

function openCreateInstance() {
  store.createInstanceModalOpen = true
}
</script>

<template>
  <aside class="sidebar">
    <div class="nav-label">{{ t('nav.menu') }}</div>

    <!-- Home Button -->
    <button
      class="nav-btn"
      :class="{active: store.page === 'home'}"
      @click="onNavClick({id: 'home'})"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M3 10.5 12 3l9 7.5 M5 9.5V21h14V9.5"/>
      </svg>
      <span>{{ t('nav.home') }}</span>
    </button>

    <!-- Instances Accordion Navigation -->
    <div class="sidebar-inst-group">
      <button
        class="nav-btn nav-btn-expandable"
        :class="{active: store.page === 'instances', expanded: instancesExpanded}"
        @click="onNavClick({id: 'instances'})"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
        </svg>
        <span class="nav-btn-label">{{ t('nav.instances') }}</span>
        <svg
          class="nav-expand-arrow"
          :class="{rotated: instancesExpanded}"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
        >
          <path d="m6 9 6 6 6-6"/>
        </svg>
      </button>

      <!-- Instances Sub-List -->
      <div class="sidebar-inst-sublist" v-show="instancesExpanded">
        <div
          v-for="inst in store.instances"
          :key="inst.id"
          class="sidebar-inst-item"
          :class="{
            selected: store.page === 'instances' && (store.selectedInstanceId === inst.id || (!store.selectedInstanceId && store.settings.activeInstance === inst.id)),
            active: store.settings.activeInstance === inst.id
          }"
          @click="selectInstance(inst)"
          :title="inst.name"
        >
          <div class="sb-inst-icon" :class="['loader-' + (inst.loader || 'vanilla')]">
            <img v-if="inst.icon" :src="inst.icon" alt="" class="sb-inst-icon-img">
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/>
            </svg>
          </div>
          <div class="sb-inst-text">
            <span class="sb-inst-name">{{ inst.name }}</span>
            <span class="sb-inst-sub">{{ inst.versionId }} • {{ t('loader.' + inst.loader) }}</span>
          </div>
          <span v-if="store.settings.activeInstance === inst.id" class="sb-inst-dot" :title="t('inst.active')"></span>
        </div>

        <button class="sidebar-inst-create-btn" @click="openCreateInstance">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M12 5v14M5 12h14"/></svg>
          <span>{{ t('inst.createBtn') }}</span>
        </button>
      </div>
    </div>

    <!-- Modpacks Catalog -->
    <button
      class="nav-btn"
      :class="{active: store.page === 'mods'}"
      @click="onNavClick({id: 'mods'})"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M14.7 6.3a4 4 0 0 0-5.4 5.4L3 18v3h3l6.3-6.3a4 4 0 0 0 5.4-5.4L14 13l-3-3 3.7-3.7z"/>
      </svg>
      <span>{{ t('nav.mods') }}</span>
    </button>

    <!-- News -->
    <button
      class="nav-btn"
      :class="{active: store.page === 'news'}"
      @click="onNavClick({id: 'news'})"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M4 4h13v16H6a2 2 0 0 1-2-2V4z M17 8h3v10a2 2 0 0 1-2 2 M8 8h5 M8 12h5 M8 16h5"/>
      </svg>
      <span>{{ t('nav.news') }}</span>
    </button>

    <!-- Settings -->
    <button
      class="nav-btn"
      :class="{active: store.page === 'settings'}"
      @click="onNavClick({id: 'settings'})"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
      <span>{{ t('nav.settings') }}</span>
    </button>

    <!-- Play time stats -->
    <div class="sidebar-stats" v-if="totalPlayTime.total > 0">
      <div class="sidebar-stat-row" :title="t('stats.todayHint') || 'Время игры сегодня (все сборки)'">
        <span class="sidebar-stat-label">{{ t('stats.today') || 'Сегодня' }}</span>
        <span class="sidebar-stat-val">{{ fmtDur(totalPlayTime.today) }}</span>
      </div>
      <div class="sidebar-stat-row" :title="t('stats.totalHint') || 'Общее время игры (все сборки)'">
        <span class="sidebar-stat-label">{{ t('stats.total') || 'Всего' }}</span>
        <span class="sidebar-stat-val">{{ fmtDur(totalPlayTime.total) }}</span>
      </div>
    </div>

    <!-- Profile card at bottom -->
    <div
      class="profile-card"
      :class="{active: store.page === 'accounts'}"
      @click="store.page = 'accounts'"
      :title="t('profile.open')"
    >
      <div class="p-avatar-wrap">
        <img :src="avatarUrl" @error="onAvatarError" alt="Skin">
        <span class="p-type-dot" :class="currentAcc?.type"></span>
      </div>
      <div class="p-meta">
        <div class="profile-name">{{ currentAcc?.username || store.settings.username }}</div>
        <div class="profile-badge-row">
          <span class="profile-type-badge" :class="currentAcc?.type">
            {{ currentAcc?.type === 'microsoft' ? 'MICROSOFT' : 'OFFLINE' }}
          </span>
        </div>
      </div>
      <div class="p-switch-icon" :title="t('accounts.title')">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="m9 18 6-6-6-6"/></svg>
      </div>
    </div>
  </aside>
</template>
