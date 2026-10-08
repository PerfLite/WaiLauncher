import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Instance } from 'bindings/WaiLauncher/models'
import { CheckInstanceModpackUpdate } from 'bindings/WaiLauncher/app'

export const useInstanceStore = defineStore('instances', () => {
  const instances = ref<Instance[]>([])
  const selectedInstanceId = ref('')
  const modpackUpdates = ref<Record<string, any>>({})

  const selectedInstance = computed<Instance | null>(() => {
    if (!instances.value || instances.value.length === 0) return null
    if (selectedInstanceId.value) {
      const found = instances.value.find(i => i.id === selectedInstanceId.value)
      if (found) return found
    }
    return instances.value[0] || null
  })

  async function checkAllModpackUpdates() {
    if (!instances.value || !instances.value.length) return
    for (const inst of instances.value) {
      if (inst.modpackSource && inst.modpackId) {
        try {
          const res = await CheckInstanceModpackUpdate(inst.id)
          if (res) {
            modpackUpdates.value[inst.id] = res
          }
        } catch (_) {}
      }
    }
  }

  function setInstances(list: Instance[]) {
    instances.value = list
    checkAllModpackUpdates()
  }

  return {
    instances,
    selectedInstanceId,
    modpackUpdates,
    selectedInstance,
    checkAllModpackUpdates,
    setInstances,
  }
})
