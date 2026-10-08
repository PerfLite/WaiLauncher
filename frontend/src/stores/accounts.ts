import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { GetAccounts, SelectAccount, RemoveAccount } from 'bindings/WaiLauncher/app'
import type { Account } from 'bindings/WaiLauncher/internal/launcher/auth/models'
import skinImg from '@/assets/skin.png'

export const useAccountStore = defineStore('accounts', () => {
  const accounts = ref<Account[]>([])
  const activeAccountId = ref('')

  const activeAccount = computed<Account | any>(() => {
    if (!accounts.value || accounts.value.length === 0) {
      return {
        id: 'default',
        username: 'Player',
        type: 'offline',
        avatarUrl: 'https://mc-heads.net/avatar/Player/64',
      }
    }
    const found = accounts.value.find(a => a.id === activeAccountId.value)
    return found || accounts.value[0]
  })

  function getAvatar(acc?: Account | any): string {
    if (!acc) return skinImg
    if (acc.avatarUrl) return acc.avatarUrl
    if (acc.uuid) return `https://mc-heads.net/avatar/${acc.uuid}/64`
    if (acc.username) return `https://mc-heads.net/avatar/${acc.username}/64`
    return skinImg
  }

  async function reloadAccounts() {
    try {
      const data = await GetAccounts()
      if (data) {
        accounts.value = (data.accounts || []) as Account[]
        activeAccountId.value = data.activeId || (accounts.value[0] ? accounts.value[0].id : '')
      }
    } catch (e) {
      console.error('Failed to load accounts:', e)
    }
  }

  async function switchActiveAccount(id: string) {
    const acc = await SelectAccount(id)
    if (acc) {
      activeAccountId.value = acc.id
      await reloadAccounts()
      return acc
    }
  }

  async function deleteAccount(id: string) {
    await RemoveAccount(id)
    await reloadAccounts()
  }

  return {
    accounts,
    activeAccountId,
    activeAccount,
    getAvatar,
    reloadAccounts,
    switchActiveAccount,
    deleteAccount,
  }
})
