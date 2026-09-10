<template>
  <div class="page">
    <div class="legend">
      <span class="pill status-queued">排队 {{ store.queued.length }}</span>
      <span class="pill status-running">推理 {{ store.running.length }}</span>
      <span class="pill status-succeeded">完成 {{ store.gallery.length }}</span>
      <span v-if="autoSwitch" class="pill switch-on">自动切换模型</span>
    </div>
    <p class="switch-hint">
      <template v-if="autoSwitch">
        离线不是禁用：工坊里标「可排队」的引擎现在就能投。默认同时只加载 1 个模型，多出来的会被后台关掉。
        <template v-if="activeEngine">当前已加载：{{ engineName(activeEngine) }}。</template>
      </template>
      <template v-else>自动切换已关，只有已经在线的引擎会跑；离线任务会失败。</template>
    </p>

    <div class="board">
      <section class="col panel">
        <h2>排队</h2>
        <article v-for="job in store.queued" :key="job.id" class="card">
          <div class="card-top">
            <strong>{{ job.title }}</strong>
            <span class="mono pos">#{{ job.queue_position || '—' }}</span>
          </div>
          <div class="meta">{{ engineName(job.engine) }} · {{ modeLabel[job.mode] }} · {{ jobLine(job) }} · P{{ job.priority }}</div>
          <div class="meta">创建 {{ clock(job.created_at) }} · 等待 {{ elapsed(job.created_at) }}</div>
          <p>{{ job.prompt }}</p>
          <div class="ops">
            <button class="btn" @click="act(() => api.bumpJob(job.id))">插队</button>
            <button class="btn" @click="act(() => api.cancelJob(job.id))">取消</button>
            <button class="btn btn-danger" @click="askDelete(job)">删除</button>
          </div>
        </article>
        <div v-if="store.queued.length === 0" class="empty">队列空闲</div>
      </section>

      <section class="col panel">
        <h2>推理中</h2>
        <article v-for="job in store.running" :key="job.id" class="card live">
          <div class="card-top">
            <strong>{{ job.title }}</strong>
            <span class="display pct">{{ job.progress }}%</span>
          </div>
          <div class="stage">{{ job.stage }}</div>
          <div class="bar"><i :style="{ width: job.progress + '%' }"></i></div>
          <div class="meta">开始 {{ clock(job.started_at) }} · 已运行 {{ elapsed(job.started_at) }} · seed {{ job.seed }} · {{ job.steps }} steps</div>
          <div class="ops">
            <button class="btn btn-danger" @click="act(() => api.cancelJob(job.id))">中止</button>
            <button class="btn" @click="askDelete(job)">删除</button>
          </div>
        </article>
        <div v-if="store.running.length === 0" class="empty">工位空闲</div>
      </section>

      <section class="col panel">
        <h2>近期结果</h2>
        <article v-for="job in done" :key="job.id" class="card">
          <div class="card-top">
            <strong>{{ job.title }}</strong>
            <span class="pill" :class="'status-' + job.status">{{ statusLabel[job.status] }}</span>
          </div>
          <div class="meta">{{ jobStamp(job) }}<template v-if="job.stage"> · {{ job.stage }}</template></div>
          <p
            v-if="job.error_message"
            class="err"
            :class="{ expanded: expanded.has(job.id) }"
            :title="expanded.has(job.id) ? '点击收起' : '点击展开完整错误'"
            @click="toggleErr(job.id)"
          >{{ job.error_message }}</p>
          <div class="ops">
            <button v-if="job.status !== 'succeeded'" class="btn" @click="act(() => api.retryJob(job.id))">重试</button>
            <button class="btn btn-danger" @click="askDelete(job)">删除</button>
          </div>
        </article>
      </section>
    </div>

    <JobDeleteModal :job="pending" :busy="deleting" :error="deleteError" @close="closeDelete" @confirm="confirmDelete" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import JobDeleteModal from '../components/JobDeleteModal.vue'
import { useAppStore } from '../stores/app'
import { api, engineName, isImageJob, modeLabel, resolutionLabel, statusLabel, type Job } from '../api/http'

const store = useAppStore()
const autoSwitch = computed(() => {
  if (store.settings?.auto_switch_engine !== undefined) return store.settings.auto_switch_engine
  if (store.system?.auto_switch_engine !== undefined) return store.system.auto_switch_engine
  return true
})
const activeEngine = computed(() => store.system?.active_engine || '')
const now = ref(Date.now())
const expanded = ref(new Set<string>())
const pending = ref<Job | null>(null)
const deleting = ref(false)
const deleteError = ref('')
let tick = 0
const done = computed(() => store.jobs.filter(j => ['succeeded', 'failed', 'cancelled'].includes(j.status)).slice(0, 12))

function askDelete(job: Job) {
  pending.value = job
  deleteError.value = ''
}

function closeDelete() {
  if (deleting.value) return
  pending.value = null
  deleteError.value = ''
}

async function confirmDelete() {
  if (!pending.value) return
  deleting.value = true
  deleteError.value = ''
  try {
    await store.deleteJob(pending.value.id)
    store.flash('已删除')
    pending.value = null
    await store.refresh()
  } catch (e: any) {
    deleteError.value = e.message || '删除失败'
  } finally {
    deleting.value = false
  }
}

function toggleErr(id: string) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

function jobLine(job: Job) {
  const size = resolutionLabel(job.short_edge)
  if (isImageJob(job)) return size
  return `${job.duration}s · ${size}`
}

function parseTime(value?: string | null) {
  if (!value) return null
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? null : d
}

function clock(value?: string | null) {
  const d = parseTime(value)
  if (!d) return '—'
  const hm = d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
  const today = new Date(now.value)
  today.setHours(0, 0, 0, 0)
  const that = new Date(d)
  that.setHours(0, 0, 0, 0)
  const dayDiff = Math.round((today.getTime() - that.getTime()) / 86400000)
  if (dayDiff === 0) return `今天 ${hm}`
  if (dayDiff === 1) return `昨天 ${hm}`
  return `${d.getMonth() + 1}/${d.getDate()} ${hm}`
}

function elapsed(started?: string | null) {
  const d = parseTime(started)
  if (!d) return '—'
  const sec = Math.max(0, Math.floor((now.value - d.getTime()) / 1000))
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return m > 0 ? `${m} 分 ${s} 秒` : `${s} 秒`
}

function took(job: Job) {
  const start = parseTime(job.started_at)
  const end = parseTime(job.finished_at)
  if (!start || !end) return ''
  const sec = Math.max(0, Math.floor((end.getTime() - start.getTime()) / 1000))
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return m > 0 ? `${m} 分 ${s} 秒` : `${s} 秒`
}

function jobStamp(job: Job) {
  const cost = took(job)
  if (job.started_at) {
    const parts = [`开始 ${clock(job.started_at)}`]
    if (job.finished_at) parts.push(`完成 ${clock(job.finished_at)}`)
    if (cost) parts.push(`耗时 ${cost}`)
    return parts.join(' · ')
  }
  const raw = job.finished_at || job.updated_at || job.created_at
  const d = parseTime(raw)
  if (!d) return '—'
  return clock(raw)
}

onMounted(() => {
  tick = window.setInterval(() => { now.value = Date.now() }, 1000)
})
onUnmounted(() => window.clearInterval(tick))

async function act(fn: () => Promise<unknown>) {
  try {
    await fn()
    await store.refresh()
  } catch (e: any) {
    store.flash(e.message)
  }
}
</script>

<style scoped>
.page { display: grid; gap: 16px; min-width: 0; }
.legend { display: flex; gap: 8px; flex-wrap: wrap; }
.switch-on { background: rgba(94,234,212,0.14); color: var(--mint); }
.switch-hint { color: var(--muted); font-size: 13px; line-height: 1.55; margin: 0; }
.board { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.col { padding: 18px; min-height: 420px; min-width: 0; overflow: hidden; }
.col h2 { font-size: 16px; margin-bottom: 14px; }
.card { padding: 14px 0; border-bottom: 1px solid var(--line); min-width: 0; }
.card-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; min-width: 0; }
.card-top strong {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.card-top .pill,
.card-top .pct,
.card-top .pos { flex: 0 0 auto; white-space: nowrap; }
.meta, .empty, .stage {
  color: var(--muted);
  font-size: 12px;
  margin: 6px 0;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.pos { color: var(--gold); }
.pct { color: var(--mint); }
p {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
  min-width: 0;
  max-width: 100%;
  overflow-wrap: anywhere;
  word-break: break-word;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.ops { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.bar { height: 6px; background: rgba(255,255,255,0.08); border-radius: 99px; overflow: hidden; margin: 10px 0; }
.bar i { display: block; height: 100%; background: linear-gradient(90deg, var(--mint), var(--gold)); }
.err {
  display: block;
  color: var(--rose);
  font-family: "JetBrains Mono", ui-monospace, monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  max-height: calc(1.55em * 4);
  overflow: hidden;
  cursor: pointer;
}
.err.expanded {
  max-height: 240px;
  overflow: auto;
}
.live { background: rgba(94,234,212,0.04); margin: 0 -8px; padding: 14px 8px; border-radius: 12px; }
@media (max-width: 1100px) { .board { grid-template-columns: minmax(0, 1fr); } }
</style>
