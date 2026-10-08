import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Play, CancelPlay, StopGame } from 'bindings/WaiLauncher/app'

export interface LaunchState {
  state: string
  stage: string
  percent: number
  message: string
}

export const useLaunchStore = defineStore('launch', () => {
  const launch = ref<LaunchState>({
    state: 'idle',
    stage: '',
    percent: 0,
    message: '',
  })
  const gameLog = ref<{ running: boolean; lines: string[] }>({
    running: false,
    lines: [],
  })

  async function startPlay(instanceId: string) {
    return await Play(instanceId)
  }

  async function cancel() {
    return await CancelPlay()
  }

  async function stop() {
    return await StopGame()
  }

  function setLaunchState(st: LaunchState) {
    launch.value = st
  }

  return {
    launch,
    gameLog,
    startPlay,
    cancel,
    stop,
    setLaunchState,
  }
})
