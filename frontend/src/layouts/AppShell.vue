<template>
  <div class="shell">
    <aside class="nav">
      <div class="brand">
        <div class="mark">A</div>
        <div>
          <div class="brand-name display">Aishow</div>
          <div class="brand-sub">本地影像工坊</div>
        </div>
      </div>

      <nav>
        <router-link v-for="item in items" :key="item.to" :to="item.to" class="nav-item">
          <span class="nav-ico">{{ item.icon }}</span>
          <span>{{ item.label }}</span>
          <span v-if="item.badge" class="badge">{{ item.badge }}</span>
        </router-link>
      </nav>

      <div class="nav-foot">
        <div class="account">
          <div class="who">{{ auth.user?.username || '未登录' }}</div>
          <button class="btn btn-ghost logout" type="button" @click="openPass">改密</button>
          <button class="btn btn-ghost logout" type="button" @click="signOut">退出</button>
        </div>
        <div class="live">
          <span class="dot" :class="store.connected ? 'on' : 'off'"></span>
          {{ store.connected ? '实时通道已连接' : '实时通道断开' }}
        </div>
        <div class="mode-chip">{{ (store.system?.inference_mode || '—').toUpperCase() }}</div>
      </div>
    </aside>

    <main class="main scrollbar">
      <header class="top">
        <div>
          <div class="kicker">{{ kicker }}</div>
          <h1>{{ title }}</h1>
        </div>
        <div class="top-meta">
          <div class="clock display">{{ clock }}</div>
          <div class="meters">
            <div class="meter">
              <span>CPU</span>
              <b>{{ cpuText }}</b>
              <i><em :style="{ width: cpuPct + '%' }"></em></i>
            </div>
            <div class="meter gpu">
              <span>GPU</span>
              <b>{{ gpuText }}</b>
              <i><em :style="{ width: gpuPct + '%' }"></em></i>
            </div>
          </div>
          <div class="pill">队列 {{ store.system?.queue_depth ?? 0 }} · 在跑 {{ store.system?.running ?? 0 }}</div>
        </div>
      </header>
      <router-view />
    </main>

    <div v-if="store.notice" class="toast">{{ store.notice }}</div>

    <Modal :open="showPass" title="修改密码" kicker="Account" :busy="passBusy" @close="showPass = false">
      <form id="pass-form" class="pass-form" @submit.prevent="savePass">
        <div class="field">
          <label>原密码</label>
          <input v-model="oldPass" class="input" type="password" autocomplete="current-password" />
        </div>
        <div class="field">
          <label>新密码</label>
          <input v-model="newPass" class="input" type="password" autocomplete="new-password" />
        </div>
        <p v-if="passError" class="pass-error">{{ passError }}</p>
      </form>
      <template #footer>
        <button class="btn btn-ghost" type="button" :disabled="passBusy" @click="showPass = false">取消</button>
        <button class="btn btn-primary" type="submit" form="pass-form" :disabled="passBusy">{{ passBusy ? '保存中…' : '保存密码' }}</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '../stores/app'
import { useAuthStore } from '../stores/auth'
import { api } from '../api/http'
import Modal from '../components/Modal.vue'

const store = useAppStore()
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const clock = ref('')
let timer = 0
let poll = 0
let hwPoll = 0
let es: EventSource | null = null

const map: Record<string, { title: string; kicker: string }> = {
  dashboard: { title: '指挥台', kicker: 'Command' },
  studio: { title: '生成工坊', kicker: '离线也能排队' },
  queue: { title: '任务队列', kicker: 'Queue' },
  gallery: { title: '作品库', kicker: 'Gallery' },
  nodes: { title: '推理节点', kicker: 'Nodes' },
  users: { title: '用户管理', kicker: 'Users' },
}

const cpuPct = computed(() => Math.round(store.hardware?.cpu_percent || 0))
const gpuPct = computed(() => store.hardware?.gpu_percent || 0)
const cpuText = computed(() => `${cpuPct.value}%`)
const gpuText = computed(() => {
  const hw = store.hardware
  if (!hw?.gpu_name) return '—'
  const used = (hw.gpu_mem_used_mb / 1024).toFixed(1)
  const total = (hw.gpu_mem_total_mb / 1024).toFixed(1)
  const temp = hw.gpu_temp ? ` · ${hw.gpu_temp}°C` : ''
  return `${hw.gpu_percent}% ${used}/${total}G${temp}`
})
const title = computed(() => map[String(route.name)]?.title || 'Aishow')
const kicker = computed(() => map[String(route.name)]?.kicker || 'Aishow')
const items = computed(() => {
  const all = [
    { to: '/', label: '指挥台', icon: '⌘', badge: '' },
    { to: '/studio', label: '生成工坊', icon: '✦', badge: '' },
    { to: '/queue', label: '任务队列', icon: '☰', badge: String((store.system?.queue_depth || 0) + (store.system?.running || 0) || '') },
    { to: '/gallery', label: '作品库', icon: '▣', badge: '' },
    { to: '/nodes', label: '推理节点', icon: '◎', badge: '' },
  ]
  if (auth.isAdmin) all.push({ to: '/users', label: '用户管理', icon: '◍', badge: '' })
  return all
})
const showPass = ref(false)
const oldPass = ref('')
const newPass = ref('')
const passBusy = ref(false)
const passError = ref('')

function openPass() {
  oldPass.value = ''
  newPass.value = ''
  passError.value = ''
  showPass.value = true
}

async function savePass() {
  passError.value = ''
  if (newPass.value.length < 6) {
    passError.value = '新密码至少 6 位'
    return
  }
  passBusy.value = true
  try {
    await api.changePassword(oldPass.value, newPass.value)
    oldPass.value = ''
    newPass.value = ''
    showPass.value = false
    store.flash('密码已更新')
  } catch (err) {
    passError.value = err instanceof Error ? err.message : '修改失败'
  } finally {
    passBusy.value = false
  }
}

async function signOut() {
  es?.close()
  es = null
  await auth.logout()
  await router.replace({ name: 'login' })
}

onMounted(async () => {
  const tick = () => {
    clock.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  }
  tick()
  timer = window.setInterval(tick, 1000)
  poll = window.setInterval(() => { store.refresh().catch(() => {}) }, 4000)
  hwPoll = window.setInterval(() => { store.refreshMetrics().catch(() => {}) }, 2000)
  try {
    await store.refresh()
  } catch {
    store.flash('后端未连接，请先启动 Go 控制面')
  }
  es = store.connectEvents()
})
onUnmounted(() => {
  window.clearInterval(timer)
  window.clearInterval(poll)
  window.clearInterval(hwPoll)
  es?.close()
})
</script>

<style scoped>
.shell {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: var(--nav-w) 1fr;
  min-height: 100%;
}
.nav {
  position: sticky;
  top: 0;
  height: 100vh;
  padding: 22px 16px;
  border-right: 1px solid var(--line);
  background: rgba(6,7,11,0.72);
  backdrop-filter: blur(18px);
  display: flex;
  flex-direction: column;
  gap: 28px;
}
.brand { display: flex; gap: 12px; align-items: center; padding: 4px 8px; }
.mark {
  width: 40px; height: 40px; border-radius: 12px;
  display: grid; place-items: center;
  background: linear-gradient(180deg, #7ff5e0, #1f8f80);
  color: #05211c; font-weight: 800; font-family: Sora, sans-serif;
}
.brand-name { font-size: 18px; }
.brand-sub { font-size: 12px; color: var(--muted); }
nav { display: flex; flex-direction: column; gap: 6px; }
.nav-item {
  display: flex; align-items: center; gap: 10px;
  padding: 11px 12px; border-radius: 12px; color: var(--muted);
}
.nav-item.router-link-exact-active {
  background: var(--mint-dim); color: var(--text);
}
.nav-ico { width: 18px; color: var(--mint); }
.badge {
  margin-left: auto;
  font-size: 11px;
  min-width: 18px;
  text-align: center;
  color: var(--gold);
}
.nav-foot { margin-top: auto; display: grid; gap: 10px; }
.account { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.who { font-size: 13px; color: var(--text); overflow: hidden; text-overflow: ellipsis; flex: 1; min-width: 0; }
.logout { padding: 6px 10px; font-size: 12px; }
.pass-form { display: grid; gap: 14px; }
.pass-error { color: var(--rose); font-size: 13px; }
.live { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--muted); }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--rose); }
.dot.on { background: var(--mint); box-shadow: 0 0 12px var(--mint); }
.mode-chip {
  font-family: Sora, sans-serif;
  font-size: 11px;
  letter-spacing: 0.16em;
  color: var(--gold);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 8px 10px;
}
.main { padding: 28px 36px 48px; min-width: 0; }
.top { display: flex; justify-content: space-between; align-items: end; margin-bottom: 28px; gap: 16px; }
.top h1 { font-size: 36px; margin-top: 6px; }
.top-meta { text-align: right; display: grid; gap: 8px; justify-items: end; }
.clock { font-size: 28px; color: var(--text); }
.meters { display: flex; gap: 12px; }
.meter {
  min-width: 168px; text-align: left;
  border: 1px solid var(--line); border-radius: 12px;
  padding: 8px 10px; background: rgba(0,0,0,0.28);
}
.meter span { font-size: 10px; letter-spacing: 0.14em; color: var(--muted); }
.meter b { display: block; font-size: 13px; margin: 4px 0 6px; font-family: Sora, sans-serif; }
.meter i { display: block; height: 4px; border-radius: 99px; background: rgba(255,255,255,0.08); overflow: hidden; }
.meter em { display: block; height: 100%; background: var(--mint); }
.meter.gpu em { background: linear-gradient(90deg, var(--mint), var(--gold)); }
.toast {
  position: fixed; right: 24px; bottom: 24px; z-index: 90;
  background: #11151f; border: 1px solid var(--line); padding: 12px 16px; border-radius: 12px;
}
@media (max-width: 980px) {
  .shell { grid-template-columns: 1fr; }
  .nav { height: auto; position: relative; flex-direction: row; align-items: center; }
  nav { flex-direction: row; overflow: auto; }
  .nav-foot { display: none; }
}
</style>
