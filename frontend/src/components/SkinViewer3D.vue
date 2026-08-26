<template>
  <div class="skin-viewer-container" :style="{ width: width + 'px', height: height + 'px' }">
    <canvas ref="canvasRef" class="skin-canvas"></canvas>
    
    <div class="skin-viewer-controls" v-if="showControls">
      <button 
        class="skin-ctrl-btn" 
        :class="{ active: currentAnim === 'walk' }" 
        @click="setAnimation('walk')" 
        title="Ходьба"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="5" r="2"/>
          <path d="m9 20 3-6 3 6M6 12l6-3 6 3"/>
        </svg>
      </button>

      <button 
        class="skin-ctrl-btn" 
        :class="{ active: currentAnim === 'idle' }" 
        @click="setAnimation('idle')" 
        title="Покой"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="5" r="2"/>
          <path d="M12 7v10M9 12h6M9 21h6"/>
        </svg>
      </button>

      <button 
        class="skin-ctrl-btn" 
        :class="{ active: currentAnim === 'run' }" 
        @click="setAnimation('run')" 
        title="Бег"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M13 4v4l4 2-2 4 3 6M7 14l3-3 2 1M5 19l4-3"/>
        </svg>
      </button>

      <button 
        class="skin-ctrl-btn" 
        :class="{ active: isRotating }" 
        @click="toggleAutoRotate" 
        title="Авто-вращение"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-1.19"/>
        </svg>
      </button>

      <button 
        class="skin-ctrl-btn" 
        @click="resetCamera" 
        title="Сброс камеры"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>
          <path d="M3 3v5h5"/>
        </svg>
      </button>
    </div>

    <div v-if="loading" class="skin-loading-overlay">
      <div class="skin-spinner"></div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { SkinViewer, WalkingAnimation, RunningAnimation, FlyingAnimation, IdleAnimation } from 'skinview3d'

const props = defineProps({
  skinUrl: {
    type: String,
    default: ''
  },
  capeUrl: {
    type: String,
    default: ''
  },
  model: {
    type: String,
    default: 'auto' // 'auto' | 'default' | 'slim'
  },
  width: {
    type: Number,
    default: 260
  },
  height: {
    type: Number,
    default: 340
  },
  animation: {
    type: String,
    default: 'walk' // 'walk' | 'run' | 'idle' | 'fly' | 'none'
  },
  showControls: {
    type: Boolean,
    default: true
  },
  autoRotate: {
    type: Boolean,
    default: false
  }
})

const canvasRef = ref(null)
const loading = ref(false)
const isRotating = ref(props.autoRotate)
const currentAnim = ref(props.animation)

let viewer = null

function applyAnimation(type) {
  if (!viewer) return
  if (type === 'walk') {
    viewer.animation = new WalkingAnimation()
    viewer.animation.speed = 0.75
  } else if (type === 'run') {
    viewer.animation = new RunningAnimation()
    viewer.animation.speed = 0.9
  } else if (type === 'idle') {
    viewer.animation = new IdleAnimation()
    viewer.animation.speed = 0.6
  } else if (type === 'fly') {
    viewer.animation = new FlyingAnimation()
  } else {
    viewer.animation = null
  }
}

function setAnimation(type) {
  currentAnim.value = type
  applyAnimation(type)
}

function toggleAutoRotate() {
  if (!viewer) return
  isRotating.value = !isRotating.value
  viewer.autoRotate = isRotating.value
  viewer.autoRotateSpeed = 1.5
}

function resetCamera() {
  if (!viewer) return
  if (viewer.controls) {
    viewer.controls.target.set(0, 0, 0)
    viewer.controls.update()
  }
  if (viewer.playerWrapper) {
    viewer.playerWrapper.rotation.set(0, 0, 0)
  }
  viewer.resetCameraPose()
  viewer.zoom = 0.95
}

async function loadSkinAndCape() {
  if (!viewer) return
  loading.value = true
  try {
    if (props.skinUrl) {
      await viewer.loadSkin(props.skinUrl, {
        model: props.model === 'slim' ? 'slim' : (props.model === 'default' ? 'default' : 'auto-detect')
      })
    }
    if (props.capeUrl) {
      await viewer.loadCape(props.capeUrl)
    } else {
      viewer.resetCape()
    }
  } catch (err) {
    console.warn('SkinViewer load error:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!canvasRef.value) return

  viewer = new SkinViewer({
    canvas: canvasRef.value,
    width: props.width,
    height: props.height,
    zoom: 0.95,
    fov: 50
  })

  viewer.autoRotate = isRotating.value
  viewer.autoRotateSpeed = 1.5

  applyAnimation(currentAnim.value)
  loadSkinAndCape()
})

watch(() => [props.skinUrl, props.capeUrl, props.model], () => {
  loadSkinAndCape()
})

watch(() => [props.width, props.height], ([newW, newH]) => {
  if (viewer) {
    viewer.setSize(newW, newH)
  }
})

onBeforeUnmount(() => {
  if (viewer) {
    viewer.dispose()
    viewer = null
  }
})
</script>

<style scoped>
.skin-viewer-container {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border-radius: 16px;
  background: radial-gradient(circle at 50% 30%, rgba(27, 217, 106, 0.08) 0%, rgba(13, 19, 15, 0.95) 75%);
  border: 1px solid rgba(255, 255, 255, 0.08);
  overflow: hidden;
  box-shadow: inset 0 0 30px rgba(0, 0, 0, 0.5), 0 8px 24px rgba(0, 0, 0, 0.3);
}

.skin-canvas {
  width: 100% !important;
  height: 100% !important;
  outline: none;
  cursor: grab;
}

.skin-canvas:active {
  cursor: grabbing;
}

.skin-viewer-controls {
  position: absolute;
  bottom: 12px;
  display: flex;
  gap: 6px;
  background: rgba(15, 22, 18, 0.85);
  backdrop-filter: blur(8px);
  padding: 4px 8px;
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  z-index: 10;
}

.skin-ctrl-btn {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: 1px solid transparent;
  background: transparent;
  color: var(--muted, #8e9cae);
  display: grid;
  place-items: center;
  cursor: pointer;
  transition: all 0.15s ease;
  padding: 0;
}

.skin-ctrl-btn svg {
  width: 14px;
  height: 14px;
}

.skin-ctrl-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.08);
}

.skin-ctrl-btn.active {
  color: #1bd96a;
  background: rgba(27, 217, 106, 0.15);
  border-color: rgba(27, 217, 106, 0.4);
}

.skin-loading-overlay {
  position: absolute;
  inset: 0;
  background: rgba(13, 19, 15, 0.6);
  display: grid;
  place-items: center;
  z-index: 5;
}

.skin-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(27, 217, 106, 0.2);
  border-top-color: #1bd96a;
  border-radius: 50%;
  animation: skin-spin 0.8s linear infinite;
}

@keyframes skin-spin {
  to { transform: rotate(360deg); }
}
</style>
