<template>
  <div class="page">
    <div class="legend">
      <span class="pill status-queued">排队 {{ store.queued.length }}</span>
      <span class="pill status-running">推理 {{ store.running.length }}</span>
      <span class="pill status-succeeded">完成 {{ store.gallery.length }}</span>
    </div>

    <div class="board">
      <section class="col panel">
        <h2>排队</h2>
        <article v-for="job in store.queued" :key="job.id" class="card">
          <div class="card-top">
            <strong>{{ job.title }}</strong>
            <span class="mono pos">#{{ job.queue_position || '—' }}</span>
          </div>
          <div class="meta">{{ engineName(job.engine) }} · {{ modeLabel[job.mode] }} · {{ jobLine(job) }} · P{{ job.priority }}</div>
          <p>{{ job.prompt }}</p>
          <div class="ops">
            <button class="btn" @click="act(() => api.bumpJob(job.id))">插队</button>
            <button class="btn" @click="act(() => api.cancelJob(job.id))">取消</button>
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
          <div class="meta">已运行 {{ elapsed(job.started_at) }} · seed {{ job.seed }} · {{ job.steps }} steps</div>
          <div class="ops">
            <button class="btn btn-danger" @click="act(() => api.cancelJob(job.id))">中止</button>
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
            <button class="btn" @click="act(() => api.deleteJob(job.id).then(() => store.refresh()))">删除</button>
          </div>
        </article>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useAppStore } from '../stores/app'
import { api, engineName, isImageJob, modeLabel, resolutionLabel, statusLabel, type Job } from '../api/http'

const store = useAppStore()
const now = ref(Date.now())
const expanded = ref(new Set<string>())
let tick = 0
const done = computed(() => store.jobs.filter(j => ['succeeded', 'failed', 'cancelled'].includes(j.status)).slice(0, 12))

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

function elapsed(started?: string | null) {
  if (!started) return '—'
  const sec = Math.max(0, Math.floor((now.value - new Date(started).getTime()) / 1000))
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return m > 0 ? `${m} 分 ${s} 秒` : `${s} 秒`
}

function jobStamp(job: Job) {
  const raw = job.finished_at || job.updated_at || job.created_at
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return '—'
  const hm = d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
  const today = new Date(now.value)
  today.setHours(0, 0, 0, 0)
  const that = new Date(d)
  that.setHours(0, 0, 0, 0)
  const dayDiff = Math.round((today.getTime() - that.getTime()) / 86400000)
  let abs = `${d.getMonth() + 1}/${d.getDate()} ${hm}`
  if (dayDiff === 0) abs = `今天 ${hm}`
  else if (dayDiff === 1) abs = `昨天 ${hm}`
  const sec = Math.max(0, Math.floor((now.value - d.getTime()) / 1000))
  let rel = '刚刚'
  if (sec >= 86400) rel = `${Math.floor(sec / 86400)} 天前`
  else if (sec >= 3600) rel = `${Math.floor(sec / 3600)} 小时前`
  else if (sec >= 60) rel = `${Math.floor(sec / 60)} 分钟前`
  return `${abs} · ${rel}`
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
