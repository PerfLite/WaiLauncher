import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface ToastItem {
  id: number
  text: string
  gold: boolean
  out: boolean
}

let toastId = 0

export const useUiStore = defineStore('ui', () => {
  const page = ref('home')
  const maximized = ref(false)
  const ready = ref(false)
  const aboutModalOpen = ref(false)
  const createInstanceModalOpen = ref(false)
  const accountsModalOpen = ref(false)
  const toasts = ref<ToastItem[]>([])

  function addToast(text: string, gold = false) {
    const id = ++toastId
    toasts.value.push({ id, text, gold, out: false })
    setTimeout(() => {
      const t = toasts.value.find(t => t.id === id)
      if (t) t.out = true
    }, 2800)
    setTimeout(() => {
      const i = toasts.value.findIndex(t => t.id === id)
      if (i !== -1) toasts.value.splice(i, 1)
    }, 3300)
  }

  function setPage(newPage: string) {
    page.value = newPage
    const el = document.getElementById('mainScroll')
    if (el) el.scrollTop = 0
  }

  return {
    page,
    maximized,
    ready,
    aboutModalOpen,
    createInstanceModalOpen,
    accountsModalOpen,
    toasts,
    addToast,
    setPage,
  }
})
