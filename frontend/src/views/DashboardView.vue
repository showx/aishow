<template>
  <div class="page">
    <section class="stats">
      <article v-for="card in cards" :key="card.label" class="panel stat">
        <div class="kicker">{{ card.kicker }}</div>
        <div class="num display">{{ card.value }}</div>
        <div class="hint">{{ card.label }}</div>
      </article>
    </section>

    <section class="panel block hw">
      <div class="row-head">
        <h2>机器占用</h2>
        <span class="pill">{{ store.hardware?.gpu_name || '未检测到 GPU' }}</span>
      </div>
      <div class="hw-grid">
        <div class="hw-card">
          <div class="kicker">CPU · {{ store.hardware?.cpu_cores || 0 }} 核</div>
          <div class="num display">{{ Math.round(store.hardware?.cpu_percent || 0) }}%</div>
          <div class="bar"><i :style="{ width: (store.hardware?.cpu_percent || 0) + '%' }"></i></div>
          <div class="hint">内存 {{ (store.hardware?.ram_used_gb || 0).toFixed(1) }} / {{ (store.hardware?.ram_total_gb || 0).toFixed(1) }} GB</div>
        </div>
        <div class="hw-card">
          <div class="kicker">GPU 算力</div>
          <div class="num display">{{ store.hardware?.gpu_percent ?? 0 }}%</div>
          <div class="bar gold"><i :style="{ width: (store.hardware?.gpu_percent || 0) + '%' }"></i></div>
          <div class="hint">{{ store.hardware?.gpu_temp ? store.hardware.gpu_temp + '°C' : '温度未知' }}</div>
        </div>
        <div class="hw-card">
          <div class="kicker">显存</div>
          <div class="num display">{{ gpuMem }}</div>
          <div class="bar"><i :style="{ width: gpuMemPct + '%' }"></i></div>
          <div class="hint">{{ ((store.hardware?.gpu_mem_used_mb || 0) / 1024).toFixed(1) }} / {{ ((store.hardware?.gpu_mem_total_mb || 0) / 1024).toFixed(1) }} GB</div>
        </div>
      </div>
    </section>

    <section class="split">
      <article class="panel block">
        <div class="row-head">
          <h2>推理链路</h2>
          <span class="pill">H3 + LLaDA-Image</span>
        </div>
        <div class="nodes">
          <div v-for="ep in store.system?.endpoints || []" :key="ep.name" class="node">
            <div class="node-top">
              <strong>{{ ep.name }}</strong>
              <span class="pill" :class="ep.healthy ? 'status-succeeded' : 'status-queued'">
                <span class="dot"></span>{{ ep.healthy ? '在线' : '可排队' }}
              </span>
            </div>
            <div class="mono url">{{ ep.url }}</div>
            <div class="muted">{{ ep.detail }} · {{ ep.latency_ms }}ms</div>
          </div>
        </div>
        <p class="note">
          「在线」表示权重已加载。后台默认同时只加载 1 个模型，多出来的边车会被关掉；「可排队」不是禁用，工坊仍可投递。FL2VA 文生 / 首尾帧；INT8 参考生成；FastH3 文生 4-step；LLaDA-Image 文生图。
        </p>
      </article>

      <article class="panel block">
        <div class="row-head">
          <h2>产线快照</h2>
          <router-link to="/queue" class="btn btn-ghost">进入队列</router-link>
        </div>
        <div v-if="live.length === 0" class="empty">当前没有在跑或排队的任务。</div>
        <div v-else class="live-list">
          <div v-for="job in live" :key="job.id" class="live">
            <div class="live-main">
              <div class="title">{{ job.title }}</div>
              <div class="muted">{{ modeLabel[job.mode] }} · {{ job.stage }}<template v-if="job.started_at"> · 开始 {{ new Date(job.started_at).toLocaleTimeString('zh-CN', { hour12: false }) }}</template></div>
            </div>
            <div class="bar"><i :style="{ width: job.progress + '%' }"></i></div>
            <div class="pct display">{{ job.progress }}%</div>
          </div>
        </div>
      </article>
    </section>

    <section class="panel block">
      <div class="row-head">
        <h2>最近成片</h2>
        <router-link to="/gallery" class="btn btn-ghost">打开片库</router-link>
      </div>
      <div v-if="recent.length === 0" class="empty">还没有完成的任务。去工坊投一条。</div>
      <div class="recent">
        <router-link v-for="job in recent" :key="job.id" class="clip" to="/gallery">
          <div class="poster">
            <img v-if="job.has_image" :src="`/api/v1/jobs/${job.id}/image`" alt="" />
            <span v-else class="play">▶</span>
            <span class="tag">{{ modeLabel[job.mode] }}</span>
          </div>
          <div class="clip-name">{{ job.title }}</div>
          <div class="muted">{{ recentStamp(job) }}</div>
        </router-link>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/app'
import { modeLabel, type Job } from '../api/http'

const store = useAppStore()
const cards = computed(() => [
  { kicker: 'Queue', value: store.system?.queue_depth ?? 0, label: '等待中的任务' },
  { kicker: 'Live', value: store.system?.running ?? 0, label: '正在推理' },
  { kicker: 'Today', value: store.system?.succeeded_today ?? 0, label: '今日成片' },
  { kicker: 'Fault', value: store.system?.failed_today ?? 0, label: '今日失败' },
])
const live = computed(() => [...store.running, ...store.queued].slice(0, 6))
const recent = computed(() => store.gallery.slice(0, 6))
const gpuMem = computed(() => {
  const total = store.hardware?.gpu_mem_total_mb || 0
  if (!total) return '—'
  return `${Math.round((store.hardware?.gpu_mem_used_mb || 0) / total * 100)}%`
})
const gpuMemPct = computed(() => {
  const total = store.hardware?.gpu_mem_total_mb || 0
  if (!total) return 0
  return Math.round((store.hardware?.gpu_mem_used_mb || 0) / total * 100)
})

function recentStamp(job: Job) {
  if (job.started_at && job.finished_at) {
    const start = new Date(job.started_at).getTime()
    const end = new Date(job.finished_at).getTime()
    if (!Number.isNaN(start) && !Number.isNaN(end)) {
      const sec = Math.max(0, Math.floor((end - start) / 1000))
      const m = Math.floor(sec / 60)
      const s = sec % 60
      const cost = m > 0 ? `${m} 分 ${s} 秒` : `${s} 秒`
      return `开始 ${new Date(job.started_at).toLocaleTimeString('zh-CN', { hour12: false })} · 耗时 ${cost}`
    }
  }
  const raw = job.finished_at || job.created_at
  return raw ? new Date(raw).toLocaleString('zh-CN') : '—'
}
</script>

<style scoped>
.page { display: grid; gap: 22px; }
.stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; }
.stat { padding: 18px 20px; }
.num { font-size: 40px; margin: 10px 0 4px; }
.hint { color: var(--muted); font-size: 13px; }
.split { display: grid; grid-template-columns: 1.1fr 0.9fr; gap: 16px; }
.hw-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 14px; }
.hw-card { padding: 4px 2px; }
.bar.gold i { background: linear-gradient(90deg, var(--mint), var(--gold)); }
.block { padding: 20px; }
.row-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.nodes { display: grid; gap: 12px; }
.node { padding: 14px; border: 1px solid var(--line); border-radius: 14px; background: rgba(0,0,0,0.18); }
.node-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.url { font-size: 12px; color: var(--blue); word-break: break-all; }
.muted { color: var(--muted); font-size: 12px; }
.note { margin-top: 14px; color: var(--muted); line-height: 1.7; font-size: 13px; }
.empty { color: var(--muted); padding: 20px 0; }
.live-list { display: grid; gap: 12px; }
.live { display: grid; grid-template-columns: 1fr 120px 48px; gap: 10px; align-items: center; }
.bar { height: 6px; background: rgba(255,255,255,0.08); border-radius: 99px; overflow: hidden; }
.bar i { display: block; height: 100%; background: linear-gradient(90deg, var(--mint), var(--gold)); }
.pct { font-size: 13px; text-align: right; }
.recent { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; }
.poster {
  aspect-ratio: 16/10;
  border-radius: 14px;
  background:
    linear-gradient(180deg, transparent, rgba(0,0,0,0.55)),
    radial-gradient(circle at 30% 20%, rgba(94,234,212,0.35), transparent 40%),
    #12151d;
  position: relative;
  margin-bottom: 8px;
  overflow: hidden;
}
.poster img { width: 100%; height: 100%; object-fit: cover; }
.play { position: absolute; left: 12px; bottom: 10px; color: #fff; }
.tag { position: absolute; right: 10px; top: 10px; font-size: 11px; color: var(--mint); }
.clip-name { font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
@media (max-width: 1100px) {
  .stats, .split, .recent, .hw-grid { grid-template-columns: 1fr 1fr; }
}
</style>
