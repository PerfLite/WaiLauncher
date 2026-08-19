<script setup>
import {ref, watch, computed, onMounted, onUnmounted} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {GetArticle, TranslateArticle, OpenURL} from '../../wailsjs/go/main/App'

const loading = ref(false)
const translating = ref(false)
const isTranslated = ref(false)
const article = ref(null)
const error = ref('')

watch(() => store.activeArticleUrl, async (url) => {
  if (!url) {
    article.value = null
    error.value = ''
    isTranslated.value = false
    return
  }
  loading.value = true
  error.value = ''
  isTranslated.value = false
  try {
    const data = await GetArticle(url)
    article.value = data
  } catch (e) {
    error.value = String(e || t('news.errArticle'))
  } finally {
    loading.value = false
  }
})

function close() {
  store.activeArticleUrl = ''
}

function onKeydown(e) {
  if (e.key === 'Escape' && store.activeArticleUrl) {
    close()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
})

async function toggleTranslate() {
  if (!article.value) return
  if (isTranslated.value) {
    isTranslated.value = false
    return
  }

  if (article.value.translatedHtml && article.value.translatedTitle) {
    isTranslated.value = true
    return
  }

  translating.value = true
  try {
    const data = await TranslateArticle(article.value.link, 'ru')
    if (data) {
      article.value = data
      isTranslated.value = true
    }
  } catch (e) {
    toast('Ошибка перевода: ' + e, true)
  } finally {
    translating.value = false
  }
}

function openInBrowser() {
  if (article.value && article.value.link) {
    OpenURL(article.value.link).catch(() => {})
  }
}

const displayTitle = computed(() => {
  if (!article.value) return ''
  return (isTranslated.value && article.value.translatedTitle) ? article.value.translatedTitle : article.value.title
})

const displayContent = computed(() => {
  if (!article.value) return ''
  return (isTranslated.value && article.value.translatedHtml) ? article.value.translatedHtml : article.value.contentHtml
})
</script>

<template>
  <div class="modal-root article-modal-root" v-if="store.activeArticleUrl">
    <div class="modal-backdrop article-backdrop" @click="close"></div>
    <div class="article-reader-container">
      
      <!-- Top Action Bar -->
      <div class="article-navbar">
        <button class="article-nav-btn back-btn" @click="close" :title="t('news.back')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5M12 19l-7-7 7-7"/></svg>
          <span>{{ t('news.back') }}</span>
        </button>

        <div class="article-nav-center"></div>

        <div class="article-nav-actions">
          <button
            class="article-nav-btn translate-btn"
            :class="{active: isTranslated}"
            :disabled="loading || translating"
            @click="toggleTranslate"
            :title="isTranslated ? t('news.showOriginal') : t('news.translate')"
          >
            <span v-if="translating" class="spinner-inline"></span>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
            <span>{{ translating ? t('news.translating') : (isTranslated ? t('news.showOriginal') : t('news.translate')) }}</span>
          </button>

          <button class="article-nav-btn browser-btn" @click="openInBrowser" :title="t('news.openInBrowser')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>
            <span>{{ t('news.openInBrowser') }}</span>
          </button>

          <button class="article-nav-close" @click="close">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
      </div>

      <!-- Main Reader Body -->
      <div class="article-reader-scroll">
        <div v-if="loading" class="article-loading-state">
          <div class="article-loading-spinner"></div>
          <p>{{ t('news.loadingArticle') }}</p>
        </div>

        <div v-else-if="error" class="article-error-state">
          <div class="err-icon">⚠️</div>
          <h3>{{ t('news.errArticle') }}</h3>
          <p>{{ error }}</p>
          <button class="btn-sec" style="margin-top: 16px;" @click="openInBrowser">{{ t('news.openInBrowser') }}</button>
        </div>

        <article v-else-if="article" class="article-main-view">
          <!-- Hero Header -->
          <div class="article-hero-header" v-if="article.HeroImage" :style="{backgroundImage: `url(${article.HeroImage})`}">
            <div class="article-hero-overlay">
              <div class="article-meta-row">
                <span class="news-tag" :class="article.kind">{{ article.tag }}</span>
                <span class="article-meta-dot">•</span>
                <span class="article-meta-date">{{ article.displayDate }}</span>
                <span class="article-meta-dot" v-if="article.author">•</span>
                <span class="article-meta-author" v-if="article.author">{{ article.author }}</span>
              </div>
              <h1 class="article-main-title">{{ displayTitle }}</h1>
            </div>
          </div>

          <div v-else class="article-header-noimg">
            <div class="article-meta-row">
              <span class="news-tag" :class="article.kind">{{ article.tag }}</span>
              <span class="article-meta-dot">•</span>
              <span class="article-meta-date">{{ article.displayDate }}</span>
              <span class="article-meta-dot" v-if="article.author">•</span>
              <span class="article-meta-author" v-if="article.author">{{ article.author }}</span>
            </div>
            <h1 class="article-main-title">{{ displayTitle }}</h1>
          </div>

          <!-- Body Content -->
          <div class="article-body-content" v-html="displayContent"></div>
        </article>
      </div>

    </div>
  </div>
</template>
