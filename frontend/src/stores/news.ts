import { defineStore } from 'pinia'
import { ref } from 'vue'
import { GetNews } from 'bindings/WaiLauncher/app'

export const useNewsStore = defineStore('news', () => {
  const news = ref<any[]>([])
  const newsLoaded = ref(false)
  const activeArticleUrl = ref('')

  async function reloadNews() {
    try {
      const n = await GetNews()
      news.value = n || []
      newsLoaded.value = true
    } catch (_) {
      newsLoaded.value = true
    }
  }

  return {
    news,
    newsLoaded,
    activeArticleUrl,
    reloadNews,
  }
})
