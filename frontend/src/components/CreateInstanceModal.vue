<script setup>
import {computed, ref, watch} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {CreateInstance, GetLoaderVersions, ImportInstanceDialog} from '../../wailsjs/go/main/App'

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

function closeCreate() {
  if (!creating.value && !importingInst.value) {
    store.createInstanceModalOpen = false
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
    newName.value = ''
    newVersion.value = store.settings.selectedVersion || store.latestRelease || (store.versions[0] ? store.versions[0].id : '')
    newLoader.value = 'vanilla'
    verQuery.value = ''
    loaderVer.value = ''
    loaderVerList.value = []
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

async function onImportInstance() {
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
    <div class="modal-box create-modal">
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2 21 7v10l-9 5-9-5V7l9-5z M12 22V12 M21 7l-9 5 M3 7l9 5"/></svg>
          </div>
          <div>
            <h3 class="modal-title">{{ t('inst.createTitle') }}</h3>
            <div class="modal-subtitle">{{ t('inst.createSub') }}</div>
          </div>
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
        <button class="btn-sec" @click="onImportInstance" :title="t('inst.import')" :disabled="importingInst || creating">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mr-mini-icon"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          <span>{{ importingInst ? 'Импорт…' : t('inst.import') }}</span>
        </button>
        <div style="flex: 1;"></div>
        <button class="btn-sec" @click="closeCreate">{{ t('inst.cancel') }}</button>
        <button class="btn-create" :disabled="!newVersion || creating" @click="doCreate">
          {{ creating ? '…' : t('inst.createBtn') }}
        </button>
      </div>
    </div>
  </div>
</template>
