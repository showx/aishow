<template>
  <div class="login">
    <div class="card panel">
      <div class="brand">
        <div class="mark">A</div>
        <div>
          <div class="kicker">Aishow</div>
          <h1>{{ heading }}</h1>
        </div>
      </div>
      <p class="lead">{{ lead }}</p>

      <form class="form" @submit.prevent="submit">
        <div class="field">
          <label>用户名</label>
          <input v-model="username" class="input" autocomplete="username" autofocus />
        </div>
        <div class="field">
          <label>密码</label>
          <input v-model="password" class="input" type="password" autocomplete="current-password" />
        </div>
        <div v-if="mode === 'register'" class="field">
          <label>确认密码</label>
          <input v-model="confirm" class="input" type="password" autocomplete="new-password" />
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <button class="btn btn-primary" type="submit" :disabled="busy">
          {{ busy ? '请稍候…' : mode === 'register' ? '创建账号' : '进入工坊' }}
        </button>
      </form>

      <button v-if="canToggle" class="switch" type="button" @click="toggle">
        {{ mode === 'login' ? '没有账号？去注册' : '已有账号？去登录' }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const confirm = ref('')
const error = ref('')
const busy = ref(false)
const mode = ref<'login' | 'register'>('login')

const canToggle = computed(() => auth.status?.allow_register && auth.status?.has_users)
const heading = computed(() => (mode.value === 'register' ? '创建账号' : '登录'))
const lead = computed(() => {
  if (!auth.status?.has_users) return '还没有账号。先创建一个，之后任务和成片都会记在你名下。'
  return '登录后继续使用本地影像工坊。'
})

onMounted(() => {
  if (!auth.status?.has_users) mode.value = 'register'
})

function toggle() {
  error.value = ''
  mode.value = mode.value === 'login' ? 'register' : 'login'
}

async function submit() {
  error.value = ''
  const name = username.value.trim()
  if (!name) {
    error.value = '请填写用户名'
    return
  }
  if (password.value.length < 6) {
    error.value = '密码至少 6 位'
    return
  }
  if (mode.value === 'register' && password.value !== confirm.value) {
    error.value = '两次输入的密码不一致'
    return
  }
  busy.value = true
  try {
    if (mode.value === 'register') await auth.register(name, password.value)
    else await auth.login(name, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect || '/')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.login {
  min-height: 100%;
  display: grid;
  place-items: center;
  padding: 32px 16px;
  position: relative;
  z-index: 1;
}
.card {
  width: min(420px, 100%);
  padding: 32px 28px 24px;
}
.brand {
  display: flex;
  gap: 14px;
  align-items: center;
  margin-bottom: 12px;
}
.mark {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  background: linear-gradient(180deg, #7ff5e0, #1f8f80);
  color: #05211c;
  font-weight: 800;
  font-family: Sora, sans-serif;
}
h1 { font-size: 28px; margin-top: 4px; }
.lead { color: var(--muted); margin: 8px 0 22px; line-height: 1.6; }
.form { display: grid; gap: 14px; }
.error { color: var(--rose); font-size: 13px; }
.switch {
  margin-top: 16px;
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}
.switch:hover { color: var(--text); }
</style>
