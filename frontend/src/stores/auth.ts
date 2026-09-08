import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, type AuthStatus, type User } from '../api/http'
import { useAppStore } from './app'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const status = ref<AuthStatus | null>(null)
  const ready = ref(false)

  const signedIn = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  function clear() {
    user.value = null
    useAppStore().reset()
  }

  async function hydrate() {
    try {
      status.value = await api.authStatus()
    } catch {
      status.value = { has_users: true, allow_register: true }
    }
    try {
      user.value = await api.me()
    } catch {
      user.value = null
    } finally {
      ready.value = true
    }
  }

  async function login(username: string, password: string) {
    user.value = await api.login(username, password)
    status.value = { has_users: true, allow_register: status.value?.allow_register ?? true }
    return user.value
  }

  async function register(username: string, password: string) {
    user.value = await api.register(username, password)
    status.value = { has_users: true, allow_register: status.value?.allow_register ?? true }
    return user.value
  }

  async function logout() {
    try {
      await api.logout()
    } catch {
      // ignore network errors; local session is cleared anyway
    }
    clear()
  }

  return { user, status, ready, signedIn, isAdmin, clear, hydrate, login, register, logout }
})
