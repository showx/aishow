<template>
  <div class="page">
    <div class="toolbar">
      <div class="seg">
        <button :class="{ active: filter === '' }" @click="filter = ''">全部</button>
        <button v-for="m in modes" :key="m" :class="{ active: filter === m }" @click="filter = m">{{ modeLabel[m] }}</button>
      </div>
    </div>

    <div v-if="items.length === 0" class="panel empty">片库还是空的。去工坊生成第一条。</div>
    <div class="grid">
      <article v-for="job in items" :key="job.id" class="panel card" @click="open(job)">
        <div class="frame">
          <img v-if="job.has_image" :src="`/api/v1/jobs/${job.id}/image`" alt="" />
          <video v-else-if="job.has_video" :src="`/api/v1/jobs/${job.id}/video`" muted></video>
          <div v-else class="poster">
            <span>模拟成品</span>
          </div>
          <div class="shade">
            <span class="tag">{{ engineName(job.engine) }} · {{ modeLabel[job.mode] }}</span>
            <span>seed {{ job.seed }} · {{ jobMeta(job) }}</span>
          </div>
        </div>
        <div class="body">
          <h3>{{ job.title }}</h3>
          <p>{{ job.prompt }}</p>
          <div class="card-ops" @click.stop>
            <button class="btn btn-danger" type="button" @click="askDelete(job)">删除</button>
          </div>
        </div>
      </article>
    </div>

    <div v-if="active" class="lightbox" @click.self="close">
      <div class="panel player">
        <div class="media-col">
          <img v-if="active.has_image" class="preview" :src="`/api/v1/jobs/${active.id}/image`" alt="" />
          <video v-else-if="active.has_video" :src="`/api/v1/jobs/${active.id}/video`" controls autoplay></video>
          <div v-else class="poster big">该任务在模拟模式下完成，没有真实成品。</div>
        </div>
        <div class="info scrollbar">
          <div class="info-head">
            <div>
              <div class="kicker">历史参数</div>
              <h2>{{ active.title }}</h2>
            </div>
            <button class="btn btn-ghost" type="button" @click="close">关闭</button>
          </div>

          <section class="block">
            <div class="block-head">
              <span>原始提示词</span>
              <button class="link" type="button" @click="copy(active.prompt)">复制</button>
            </div>
            <p class="prompt">{{ active.prompt }}</p>
          </section>

          <section v-if="showEnhanced" class="block">
            <div class="block-head">
              <span>增强后提示词</span>
              <button class="link" type="button" @click="copy(active.enhanced_prompt)">复制</button>
            </div>
            <p class="prompt">{{ active.enhanced_prompt }}</p>
          </section>

          <section class="block">
            <div class="block-head">
              <span>生成参数</span>
              <button class="link" type="button" @click="copy(paramText(active))">复制全部</button>
            </div>
            <dl class="kvs">
              <template v-for="row in paramRows(active)" :key="row.k">
                <dt>{{ row.k }}</dt>
                <dd>{{ row.v }}</dd>
              </template>
            </dl>
          </section>

          <section v-if="(active.assets || []).length" class="block">
            <div class="block-head"><span>输入素材</span></div>
            <div class="refs">
              <figure v-for="asset in active.assets" :key="asset.id" class="ref">
                <img v-if="asset.type === 'image'" :src="`/api/v1/uploads/${asset.upload_id}/raw`" :alt="asset.filename" />
                <video v-else-if="asset.type === 'video'" :src="`/api/v1/uploads/${asset.upload_id}/raw`" muted></video>
                <div v-else class="ref-file">{{ asset.filename }}</div>
                <figcaption>{{ assetLabel(asset) }}</figcaption>
              </figure>
            </div>
          </section>

          <div class="ops">
            <button class="btn" type="button" @click="reuse(active)">填回工坊</button>
            <a v-if="active.has_image" class="btn btn-primary" :href="`/api/v1/jobs/${active.id}/image`" download>下载图片</a>
            <a v-else-if="active.has_video" class="btn btn-primary" :href="`/api/v1/jobs/${active.id}/video`" download>下载成片</a>
            <button class="btn btn-danger" type="button" @click="askDelete(active)">删除</button>
          </div>
        </div>
      </div>
    </div>

    <JobDeleteModal :job="pending" :busy="deleting" :error="deleteError" @close="closeDelete" @confirm="confirmDelete" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import JobDeleteModal from '../components/JobDeleteModal.vue'
import { useAppStore } from '../stores/app'
import {
  api,
  canvasSize,
  engineName,
  isImageJob,
  modeLabel,
  resolutionLabel,
  type Job,
  type JobAsset,
  type JobMode,
} from '../api/http'

const store = useAppStore()
const router = useRouter()
const filter = ref('')
const active = ref<Job | null>(null)
const pending = ref<Job | null>(null)
const deleting = ref(false)
const deleteError = ref('')
const modes: JobMode[] = ['t2va', 'i2va', 'l2va', 'fl2va', 'ref2va', 't2i', 'i2i']
const items = computed(() => store.gallery.filter(j => !filter.value || j.mode === filter.value))
const showEnhanced = computed(() => {
  const job = active.value
  return !!job?.enhanced_prompt && job.enhanced_prompt !== job.prompt
})

function jobMeta(job: Job) {
  const size = `${resolutionLabel(job.short_edge)} · ${job.aspect_ratio}`
  if (isImageJob(job)) return size
  return `${job.duration}s · ${size}`
}

function qualityLabel(job: Job) {
  if (job.quality === 'turbo') return 'Turbo 4 步'
  if (job.quality === 'base') return 'Base 50 步'
  return job.quality || '—'
}

function when(value?: string | null) {
  if (!value) return '—'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString('zh-CN')
}

function took(job: Job) {
  if (!job.started_at || !job.finished_at) return '—'
  const start = new Date(job.started_at).getTime()
  const end = new Date(job.finished_at).getTime()
  if (Number.isNaN(start) || Number.isNaN(end)) return '—'
  const sec = Math.max(0, Math.floor((end - start) / 1000))
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return m > 0 ? `${m} 分 ${s} 秒` : `${s} 秒`
}

function sizeBytes(n: number) {
  if (!n) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

function paramRows(job: Job) {
  const { width, height } = canvasSize(job.aspect_ratio, job.short_edge)
  const image = isImageJob(job)
  const fast = job.engine === 'fasth3' || job.engine === 'h3-max'
  const rows: { k: string; v: string }[] = [
    { k: '引擎', v: engineName(job.engine) },
    { k: '模式', v: modeLabel[job.mode] },
  ]
  if (!image) rows.push({ k: '时长', v: `${job.duration}s` })
  rows.push(
    { k: '分辨率', v: `${resolutionLabel(job.short_edge)} · ${width}×${height}` },
    { k: '画幅', v: job.aspect_ratio || '—' },
    { k: 'Seed', v: String(job.seed) },
    { k: 'Steps', v: String(job.steps) },
    { k: '档位', v: qualityLabel(job) },
  )
  if (image) rows.push({ k: 'Guidance', v: String(job.flow_shift) })
  else if (!fast) {
    rows.push(
      { k: 'Flow Shift', v: String(job.flow_shift) },
      { k: 'Audio Flow', v: String(job.audio_flow_shift) },
    )
  }
  if (!image) rows.push({ k: '提示增强', v: job.enhance_prompt ? '开启' : '关闭' })
  rows.push(
    { k: '优先级', v: String(job.priority) },
    { k: '成品体积', v: sizeBytes(job.output_size) },
    { k: '创建时间', v: when(job.created_at) },
    { k: '开始时间', v: when(job.started_at) },
    { k: '完成时间', v: when(job.finished_at) },
    { k: '生成耗时', v: took(job) },
    { k: '任务 ID', v: job.id },
  )
  return rows
}

function paramText(job: Job) {
  const lines = paramRows(job).map(row => `${row.k}: ${row.v}`)
  lines.unshift(`标题: ${job.title}`)
  lines.push(`提示词: ${job.prompt}`)
  if (job.enhanced_prompt && job.enhanced_prompt !== job.prompt) {
    lines.push(`增强提示: ${job.enhanced_prompt}`)
  }
  return lines.join('\n')
}

function assetLabel(asset: JobAsset) {
  if (asset.role === 'keyframe' && asset.frame_index === 0) return '首帧'
  if (asset.role === 'keyframe' && asset.frame_index === -1) return '尾帧'
  if (asset.role === 'keyframe') return '关键帧'
  if (asset.type === 'video') return '参考视频'
  if (asset.type === 'audio') return '参考音频'
  return '参考图'
}

async function open(job: Job) {
  active.value = job
  try {
    const full = await api.job(job.id)
    if (active.value?.id === job.id) active.value = full
  } catch {}
}

function close() {
  active.value = null
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    store.flash('已复制')
  } catch {
    store.flash('复制失败')
  }
}

function reuse(job: Job) {
  router.push({ name: 'studio', query: { reuse: job.id } })
}

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
  const id = pending.value.id
  deleting.value = true
  deleteError.value = ''
  try {
    await store.deleteJob(id)
    if (active.value?.id === id) active.value = null
    store.flash('已删除')
    pending.value = null
  } catch (e: any) {
    deleteError.value = e.message || '删除失败'
  } finally {
    deleting.value = false
  }
}

watch(() => store.jobs, list => {
  if (active.value && !list.some(j => j.id === active.value?.id)) active.value = null
})

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && !pending.value) close()
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<style scoped>
.page { display: grid; gap: 18px; min-width: 0; }
.grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.card { overflow: hidden; cursor: pointer; min-width: 0; }
.frame { position: relative; aspect-ratio: 16/10; background: #0b0d14; }
.frame video, .frame img, .poster { width: 100%; height: 100%; object-fit: cover; }
.preview { width: 100%; border-radius: 12px; background: #000; display: block; }
.poster {
  display: grid; place-items: center;
  background:
    radial-gradient(circle at 20% 20%, rgba(94,234,212,0.25), transparent 45%),
    radial-gradient(circle at 80% 80%, rgba(232,184,109,0.2), transparent 40%),
    #10131b;
  color: var(--muted);
}
.shade {
  position: absolute; inset: auto 0 0 0; padding: 10px 12px;
  display: flex; justify-content: space-between; gap: 8px;
  background: linear-gradient(transparent, rgba(0,0,0,0.7));
  font-size: 12px;
}
.shade span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag { color: var(--mint); }
.body { padding: 14px 16px 16px; min-width: 0; }
.body h3 { font-size: 16px; margin-bottom: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.body p {
  color: var(--muted); font-size: 13px; line-height: 1.6;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
  overflow-wrap: anywhere;
}
.card-ops { display: flex; justify-content: flex-end; margin-top: 12px; }
.empty { padding: 40px; color: var(--muted); text-align: center; }
.lightbox {
  position: fixed; inset: 0; z-index: 30;
  background: rgba(0,0,0,0.62);
  display: grid; place-items: center; padding: 24px;
}
.player {
  width: min(1180px, 100%);
  max-height: calc(100vh - 48px);
  padding: 16px;
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(280px, 0.9fr);
  gap: 16px;
  align-items: start;
  overflow: hidden;
}
.media-col { min-width: 0; }
.player video { width: 100%; border-radius: 12px; background: #000; display: block; }
.info {
  min-width: 0;
  max-height: calc(100vh - 80px);
  overflow: auto;
  display: grid;
  gap: 14px;
  align-content: start;
}
.info-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.info-head h2 { font-size: 20px; overflow-wrap: anywhere; }
.block { min-width: 0; }
.block-head {
  display: flex; justify-content: space-between; align-items: center; gap: 8px;
  color: var(--muted); font-size: 12px; letter-spacing: 0.08em; text-transform: uppercase;
  margin-bottom: 8px;
}
.link {
  border: 0; background: transparent; color: var(--mint); cursor: pointer; font-size: 12px; padding: 0;
}
.prompt {
  color: var(--text); line-height: 1.7; white-space: pre-wrap; overflow-wrap: anywhere;
  max-height: 160px; overflow: auto; font-size: 13px;
  background: rgba(0,0,0,0.22); border: 1px solid var(--line); border-radius: 12px; padding: 10px 12px;
}
.kvs {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  gap: 8px 12px;
  margin: 0;
  font-size: 13px;
}
.kvs dt { color: var(--muted); }
.kvs dd { margin: 0; overflow-wrap: anywhere; font-family: "JetBrains Mono", ui-monospace, monospace; font-size: 12px; }
.refs { display: grid; grid-template-columns: repeat(auto-fill, minmax(92px, 1fr)); gap: 8px; }
.ref { margin: 0; min-width: 0; }
.ref img, .ref video {
  width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 10px; background: #000; display: block;
}
.ref-file {
  aspect-ratio: 1; border-radius: 10px; display: grid; place-items: center; text-align: center;
  padding: 8px; font-size: 11px; color: var(--muted); background: rgba(0,0,0,0.28); overflow-wrap: anywhere;
}
.ref figcaption { margin-top: 6px; font-size: 11px; color: var(--muted); }
.ops { display: flex; flex-wrap: wrap; gap: 8px; }
.poster.big { min-height: 280px; border-radius: 12px; }
@media (max-width: 1100px) {
  .grid { grid-template-columns: minmax(0, 1fr); }
  .player { grid-template-columns: minmax(0, 1fr); max-height: calc(100vh - 32px); overflow: auto; }
  .info { max-height: none; }
}
</style>
