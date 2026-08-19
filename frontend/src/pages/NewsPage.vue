<script setup>
import {ref, computed, onMounted, onUnmounted} from 'vue'
import {store, toast} from '../store'
import {t} from '../i18n'
import {GetArticle, TranslateArticle, OpenURL} from '../../wailsjs/go/main/App'

const activeUrl = ref('')
const loading = ref(false)
const translating = ref(false)
const isTranslated = ref(false)
const article = ref(null)
const error = ref('')

async function openArticle(a) {
  if (!a || !a.link) return
  activeUrl.value = a.link
  loading.value = true
  error.value = ''
  isTranslated.value = false
  article.value = null

  // Scroll to top of page
  const mainScroll = document.getElementById('mainScroll')
  if (mainScroll) mainScroll.scrollTop = 0

  try {
    const data = await GetArticle(a.link)
    article.value = data
  } catch (e) {
    error.value = String(e || t('news.errArticle'))
  } finally {
    loading.value = false
  }
}

function closeArticle() {
  activeUrl.value = ''
  article.value = null
  error.value = ''
  isTranslated.value = false
  translating.value = false
}

function onKeydown(e) {
  if (e.key === 'Escape' && activeUrl.value && store.page === 'news') {
    closeArticle()
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
    if (data && (data.translatedHtml || data.translatedTitle)) {
      article.value = data
      isTranslated.value = true
    } else {
      toast('Не удалось перевести статью', true)
    }
  } catch (e) {
    toast('Ошибка перевода: ' + e, true)
  } finally {
    translating.value = false
  }
}

function openInBrowser() {
  const url = (article.value && article.value.link) ? article.value.link : activeUrl.value
  if (url) {
    OpenURL(url).catch(() => {})
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
  <section class="page news-page-section">
    <!-- View 1: Full-Page Article Reader View -->
    <div class="article-fullpage-wrap" v-if="activeUrl">
      <!-- Top Action Bar -->
      <div class="article-navbar fullpage">
        <button class="article-nav-btn back-btn" @click="closeArticle" :title="t('news.back')">
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
        </div>
      </div>

      <!-- Main Article Reader View -->
      <div class="article-fullpage-content">
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
          <div class="article-hero-header fullpage" v-if="article.heroImage" :style="{backgroundImage: `url(${article.heroImage})`}">
            <div class="article-hero-overlay fullpage">
              <div class="article-meta-row">
                <span class="news-tag" :class="article.kind">{{ article.tag }}</span>
                <span class="article-meta-dot">•</span>
                <span class="article-meta-date">{{ article.displayDate }}</span>
                <span class="article-meta-dot" v-if="article.author">•</span>
                <span class="article-meta-author" v-if="article.author">{{ article.author }}</span>
              </div>
              <h1 class="article-main-title fullpage">{{ displayTitle }}</h1>
            </div>
          </div>

          <div v-else class="article-header-noimg fullpage">
            <div class="article-meta-row">
              <span class="news-tag" :class="article.kind">{{ article.tag }}</span>
              <span class="article-meta-dot">•</span>
              <span class="article-meta-date">{{ article.displayDate }}</span>
              <span class="article-meta-dot" v-if="article.author">•</span>
              <span class="article-meta-author" v-if="article.author">{{ article.author }}</span>
            </div>
            <h1 class="article-main-title fullpage">{{ displayTitle }}</h1>
          </div>

          <!-- Body Content -->
          <div class="article-body-content fullpage" v-html="displayContent"></div>
        </article>
      </div>
    </div>

    <!-- View 2: News Grid View -->
    <div v-else class="news-grid-view">
      <div class="section-head" style="margin-top:0"><h2>{{ t('news.title') }}</h2></div>
      <div class="news-grid" v-if="store.news.length">
        <article v-for="a in store.news" :key="a.link" class="news-card" @click="openArticle(a)">
          <div class="news-img" :style="{backgroundImage: `url(${a.image})`}">
            <span class="news-tag" :class="a.kind">{{ a.tag }}</span>
          </div>
          <div class="news-body">
            <div class="news-date">{{ a.displayDate }}</div>
            <div class="news-title">{{ a.title }}</div>
            <div class="news-text">{{ a.text }}</div>
          </div>
        </article>
      </div>
      <div v-else class="news-empty">
        {{ store.newsLoaded ? t('news.error') : t('news.loading') }}
      </div>
    </div>
  </section>
</template>
