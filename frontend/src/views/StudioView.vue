<template>
  <div class="studio">
    <section class="panel composer">
      <div class="seg">
        <button
          v-for="e in visibleEngines"
          :key="e.id"
          :class="{ active: form.engine === e.id, offline: !e.online }"
          @click="pickEngine(e.id)"
        >{{ e.label }}<small>{{ e.online ? '在线' : '离线' }}</small></button>
      </div>
      <p class="hint">{{ engineHint }}</p>

      <div class="seg">
        <button v-for="m in visibleModes" :key="m.id" :class="{ active: form.mode === m.id }" @click="setMode(m.id)">{{ m.label }}</button>
      </div>

      <div class="field">
        <label>{{ isLLada ? '画面提示' : '镜头提示' }}</label>
        <textarea v-model="form.prompt" class="textarea" :placeholder="placeholder"></textarea>
        <div class="examples">
          <button v-for="ex in examples" :key="ex" class="pill click" @click="form.prompt = ex">{{ ex.slice(0, 18) }}…</button>
        </div>
      </div>

      <div class="media" v-if="needMedia">
        <DropZone v-if="needFirst" v-model="firstFrame" title="首帧" accept="image/*" kind="image" />
        <DropZone v-if="needLast" v-model="lastFrame" title="尾帧" accept="image/*" kind="image" />
        <DropZone v-if="form.mode === 'ref2va' || form.mode === 'i2i'" v-model="refImage" title="参考图" accept="image/*" kind="image" />
        <DropZone v-if="form.mode === 'ref2va'" v-model="refVideo" title="参考视频" accept="video/*" kind="video" />
        <DropZone v-if="form.mode === 'ref2va'" v-model="refAudio" title="参考音频" accept="audio/*" kind="audio" />
      </div>

      <div class="grid-2">
        <div class="field" v-if="!isLLada">
          <label>时长 {{ form.duration }}s</label>
          <input v-model.number="form.duration" type="range" :min="minDuration" max="15" step="1" />
          <div class="seg ticks">
            <button v-for="d in visibleDurations" :key="d" :class="{ active: form.duration === d }" @click="form.duration = d">{{ d }}s</button>
          </div>
        </div>
        <div class="field" v-else>
          <label>档位</label>
          <div class="seg">
            <button :class="{ active: form.quality === 'turbo' }" @click="setLLadaQuality('turbo')">Turbo 4 步</button>
            <button :class="{ active: form.quality === 'base' }" @click="setLLadaQuality('base')">Base 50 步</button>
          </div>
          <div class="hint">Turbo 对应蒸馏权重；Base 对应 50 步高品质权重。边车加载的 checkpoint 应与档位一致。</div>
        </div>
        <div class="field">
          <label>分辨率 {{ sizeHint }}</label>
          <div class="seg">
            <button
              v-for="r in visibleResolutions"
              :key="r.short"
              :class="{ active: form.short_edge === r.short }"
              @click="form.short_edge = r.short"
            >{{ r.label }}</button>
          </div>
          <div class="hint">{{ resolutionHint }}</div>
        </div>
      </div>

      <div class="field">
        <label>画幅</label>
        <div class="seg">
          <button v-for="r in visibleRatios" :key="r" :class="{ active: form.aspect_ratio === r }" @click="form.aspect_ratio = r">{{ r }}</button>
        </div>
      </div>

      <details class="adv">
        <summary>高级采样</summary>
        <div v-if="isLLada" class="grid-3">
          <div class="field"><label>Seed</label><input v-model.number="form.seed" class="input" type="number"></div>
          <div class="field"><label>Steps</label><input v-model.number="form.steps" class="input" type="number"></div>
          <div class="field"><label>Guidance</label><input v-model.number="form.flow_shift" class="input" type="number" step="0.1"></div>
          <div class="field"><label>优先级</label><input v-model.number="form.priority" class="input" type="number"></div>
        </div>
        <div v-else-if="!isFastH3" class="grid-3">
          <div class="field"><label>Seed</label><input v-model.number="form.seed" class="input" type="number"></div>
          <div class="field"><label>Steps</label><input v-model.number="form.steps" class="input" type="number"></div>
          <div class="field"><label>Quality</label>
            <select v-model="form.quality" class="select">
              <option value="lossless">lossless</option>
              <option value="high">high</option>
            </select>
          </div>
          <div class="field"><label>Flow Shift</label><input v-model.number="form.flow_shift" class="input" type="number" step="0.1"></div>
          <div class="field"><label>Audio Flow</label><input v-model.number="form.audio_flow_shift" class="input" type="number" step="0.1"></div>
          <div class="field"><label>优先级</label><input v-model.number="form.priority" class="input" type="number"></div>
        </div>
        <div v-else class="grid-3">
          <div class="field"><label>Seed</label><input v-model.number="form.seed" class="input" type="number"></div>
          <div class="field"><label>Steps</label><input class="input" type="number" :value="5" disabled></div>
          <div class="field"><label>优先级</label><input v-model.number="form.priority" class="input" type="number"></div>
        </div>
        <label v-if="!isLLada" class="check">
          <input v-model="form.enhance_prompt" type="checkbox" />
          使用官方 H3-Context-IR 增强提示（需要 MiniMax Token）
        </label>
      </details>

      <div class="actions">
        <div class="muted">{{ actionHint }}</div>
        <button class="btn btn-primary" :disabled="busy" @click="submit">{{ busy ? '投递中…' : '投入队列' }}</button>
      </div>
    </section>

    <aside class="panel side">
      <div class="row-head"><h2>队列预览</h2></div>
      <div v-if="preview.length === 0" class="muted">工坊提交后会出现在这里。</div>
      <div v-for="job in preview" :key="job.id" class="item">
        <div class="item-top">
          <strong>{{ job.title }}</strong>
          <span class="pill" :class="'status-' + job.status">{{ statusLabel[job.status] }}</span>
        </div>
        <div class="muted">{{ engineName(job.engine) }} · {{ job.stage }} · {{ jobMeta(job) }}</div>
        <div class="bar"><i :style="{ width: job.progress + '%' }"></i></div>
      </div>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DropZone from '../components/DropZone.vue'
import { api, canvasSize, engineName, isImageJob, resolutionLabel, resolutionPresets, statusLabel, type Job, type JobEngine, type JobMode } from '../api/http'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const route = useRoute()
const router = useRouter()
const busy = ref(false)
const firstFrame = ref<File | null>(null)
const lastFrame = ref<File | null>(null)
const refImage = ref<File | null>(null)
const refVideo = ref<File | null>(null)
const refAudio = ref<File | null>(null)

const engines = [
  { id: 'h3' as JobEngine, label: 'H3-Base 本地' },
  { id: 'h3-ref2va-int8' as JobEngine, label: 'H3 Ref2VA INT8' },
  { id: 'fasth3' as JobEngine, label: 'FastH3 本地' },
  { id: 'llada-image' as JobEngine, label: 'LLaDA-Image' },
]
const enginePref: JobEngine[] = ['fasth3', 'h3', 'h3-ref2va-int8', 'llada-image']
const userPickedEngine = ref(false)
const autoPickedEngine = ref(false)
const modes = [
  { id: 't2va' as JobMode, label: '文生影像' },
  { id: 'i2va' as JobMode, label: '首帧续写' },
  { id: 'l2va' as JobMode, label: '尾帧倒叙' },
  { id: 'fl2va' as JobMode, label: '首尾桥接' },
  { id: 'ref2va' as JobMode, label: '参考生成' },
]
const imageModes = [
  { id: 't2i' as JobMode, label: '文生图' },
  { id: 'i2i' as JobMode, label: '指令编辑' },
]
const ratios = ['16:9', '9:16', '1:1', '4:3', '21:9', 'auto']
const imageRatios = ['1:1', '16:9', '9:16', '4:3', '3:4']
const durations = [2, 5, 8, 10, 15]
const fastDurations = [5, 8, 10, 15]
const fastResolutions = [
  { short: 480, label: '480p' },
  { short: 768, label: '768p' },
] as const
const imageResolutions = [
  { short: 768, label: '768' },
  { short: 1024, label: '1024' },
] as const
const videoExamples = [
  '夜色卧室：主人熟睡时，三只猫列队闯入吹奏微型铜管，随即若无其事离开。',
  '女舰长背对镜头立于星舰舰桥观景窗前，舰队跃迁的蓝白强光吞没整座舰桥。',
  '日式食堂特写拉面升腾热气，焦点缓缓推向后方热闹的家庭聚餐。',
]
const imageExamples = [
  '电影感照片：一只红狐站在新雪里，冬日柔光，毛发细节，浅景深。',
  '云海之上的安静天文台，日出金光，广角摄影，中英文字体海报感。',
  '把它改成水彩风格，保留主体与构图，纸面纹理清晰。',
]
const form = reactive({
  engine: 'h3' as JobEngine,
  mode: 't2va' as JobMode,
  prompt: '',
  duration: 5,
  aspect_ratio: '16:9',
  short_edge: 480,
  seed: 1101,
  steps: 50,
  flow_shift: 12,
  audio_flow_shift: 3,
  quality: 'lossless',
  enhance_prompt: false,
  priority: 0,
})

const engineOnline = computed(() => {
  const map: Record<JobEngine, boolean> = {
    h3: false,
    fasth3: false,
    'h3-max': false,
    'h3-ref2va-int8': false,
    'llada-image': false,
  }
  for (const ep of store.system?.endpoints || []) {
    if (!ep.healthy) continue
    const name = (ep.name || '').toLowerCase()
    if (name.includes('fasth3')) map.fasth3 = true
    else if (name.includes('llada')) map['llada-image'] = true
    else if (name.includes('ref2va') || name.includes('int8')) map['h3-ref2va-int8'] = true
    else if (name.includes('fl2va') || name.includes('h3-base')) map.h3 = true
  }
  return map
})
const visibleEngines = computed(() =>
  engines
    .map(e => ({ ...e, online: !!engineOnline.value[e.id] }))
    .sort((a, b) => {
      if (a.online !== b.online) return a.online ? -1 : 1
      return enginePref.indexOf(a.id) - enginePref.indexOf(b.id)
    }),
)

function preferredOnlineEngine(): JobEngine {
  const online = enginePref.filter(id => engineOnline.value[id])
  return online[0] || 'h3'
}

function pickEngine(id: JobEngine) {
  userPickedEngine.value = true
  setEngine(id)
}

function applyOnlineDefault() {
  if (autoPickedEngine.value || userPickedEngine.value || route.query.reuse) return
  if (!enginePref.some(id => engineOnline.value[id])) return
  autoPickedEngine.value = true
  const next = preferredOnlineEngine()
  if (next !== form.engine) setEngine(next)
}

const isFastH3 = computed(() => form.engine === 'fasth3' || form.engine === 'h3-max')
const isRef2VAInt8 = computed(() => form.engine === 'h3-ref2va-int8')
const isLLada = computed(() => form.engine === 'llada-image')
const visibleModes = computed(() => {
  if (isLLada.value) return imageModes
  if (isFastH3.value) return modes.filter(m => m.id === 't2va')
  if (isRef2VAInt8.value) return modes.filter(m => m.id === 'ref2va')
  return modes.filter(m => m.id !== 'ref2va')
})
const visibleDurations = computed(() => (isFastH3.value || isRef2VAInt8.value) ? [2, 5, 8, 10, 15] : durations)
const visibleResolutions = computed(() => {
  if (isLLada.value) return imageResolutions
  if (isFastH3.value || isRef2VAInt8.value) return fastResolutions
  return resolutionPresets
})
const visibleRatios = computed(() => {
  if (isLLada.value) return imageRatios
  if (isFastH3.value || isRef2VAInt8.value) return ratios.filter(r => r !== 'auto')
  return ratios
})
const minDuration = computed(() => 2)
const engineHint = computed(() => {
  const online = engineOnline.value[form.engine]
  const offline = online ? '' : '当前节点离线，请改选带「在线」的引擎。'
  if (isLLada.value) return ['LLaDA-Image 是开源 6B 文生图 / 指令编辑模型。先启动本机边车，再把推理模式设成 auto。', offline].filter(Boolean).join(' ')
  if (isFastH3.value) return ['本地 FastH3 GGUF Q4（ComfyUI 4-step + VSA）。只支持文生。先跑 start_fasth3_gguf.bat，推理模式设成 auto。', offline].filter(Boolean).join(' ')
  if (isRef2VAInt8.value) return ['Comfy-Org pruned INT8 参考生成。先跑 start_h3_ref2va_int8.bat。24GB 不要和 NF4 边车同时开。', offline].filter(Boolean).join(' ')
  return ['本地 H3-Base NF4：文生 / 首尾帧走 start_h3_nf4.bat（30010）。参考生成请改选「H3 Ref2VA INT8」。', offline].filter(Boolean).join(' ')
})
const resolutionHint = computed(() => {
  if (isLLada.value) return '文生图边长需能被 16 整除；指令编辑需能被 32 整除。默认 1024。'
  if (isFastH3.value) return '训练分辨率是 768×1344 / 5 秒。24GB 建议先用 480p / 5 秒试一条。'
  if (isRef2VAInt8.value) return 'INT8 24GB 建议 480p / 5 秒 / 20 步。768p 更吃显存。'
  return 'NF4 24GB 推荐 480p；720p / 1080p 更慢，也可能撑满显存。'
})
const actionHint = computed(() => {
  if (isLLada.value) return '将任务投入队列，由工位调用本机 LLaDA-Image /v1/images。'
  if (isFastH3.value) return '将任务投入队列，由工位调用 FastH3 GGUF 边车 /v1/videos（模型别名 fasth3）。'
  if (isRef2VAInt8.value) return '将任务投入队列，由工位调用 Ref2VA INT8 边车 /v1/videos（30011 → ComfyUI）。'
  return '将生成任务投入本地队列，由工位调用 NF4 / SGLang FL2VA（30010）。'
})
const needFirst = computed(() => form.mode === 'i2va' || form.mode === 'fl2va')
const needLast = computed(() => form.mode === 'l2va' || form.mode === 'fl2va')
const needMedia = computed(() => form.mode !== 't2va' && form.mode !== 't2i')
const examples = computed(() => isLLada.value ? imageExamples : videoExamples)
const placeholder = computed(() => isLLada.value
  ? '写画面：主体、风格、光线、构图；编辑模式写你要改什么。'
  : '用镜头语言写：主体、运动、光、声音与时间点。')
const preview = computed(() => store.jobs.slice(0, 8))
const sizeHint = computed(() => {
  const { width, height } = canvasSize(form.aspect_ratio, form.short_edge)
  return `${width}×${height}`
})

function setMode(id: JobMode) {
  form.mode = id
  if (form.engine === 'h3-ref2va-int8' && form.steps === 50) form.steps = 20
}

function setEngine(id: JobEngine) {
  form.engine = id
  if (id === 'llada-image') {
    if (form.mode !== 't2i' && form.mode !== 'i2i') form.mode = 't2i'
    form.aspect_ratio = '1:1'
    form.short_edge = 1024
    form.enhance_prompt = false
    setLLadaQuality(form.quality === 'base' ? 'base' : 'turbo')
    return
  }
  if (form.mode === 't2i' || form.mode === 'i2i') form.mode = 't2va'
  if (form.quality === 'turbo' || form.quality === 'base') form.quality = 'lossless'
  if (form.flow_shift === 1 || form.flow_shift === 5) form.flow_shift = 12
  if (form.steps === 4) form.steps = 50
  if (id === 'h3-ref2va-int8') {
    form.mode = 'ref2va'
    form.steps = 20
    form.short_edge = form.short_edge >= 640 ? 768 : 480
    if (form.aspect_ratio === 'auto') form.aspect_ratio = '16:9'
    return
  }
  if (id === 'h3') {
    if (form.mode === 'ref2va') form.mode = 't2va'
    if (form.steps === 20) form.steps = 50
  }
  if (id === 'fasth3' || id === 'h3-max') {
    form.mode = 't2va'
    form.steps = 4
    form.short_edge = form.short_edge >= 640 ? 768 : 480
    if (form.aspect_ratio === 'auto') form.aspect_ratio = '16:9'
  }
}

function setLLadaQuality(q: 'turbo' | 'base') {
  form.quality = q
  form.steps = q === 'base' ? 50 : 4
  form.flow_shift = q === 'base' ? 5 : 1
}

function jobMeta(job: Job) {
  const size = `${resolutionLabel(job.short_edge)} · ${job.aspect_ratio}`
  if (isImageJob(job)) return size
  return `${job.duration}s · ${size}`
}

async function uploadIf(file: File | null) {
  if (!file) return null
  return api.upload(file)
}

function asEngine(engine?: string, mode?: string): JobEngine {
  if (engine === 'fasth3' || engine === 'h3-max') return 'fasth3'
  if (engine === 'h3-ref2va-int8') return 'h3-ref2va-int8'
  if (engine === 'llada-image') return 'llada-image'
  if (mode === 'ref2va') return 'h3-ref2va-int8'
  return 'h3'
}

async function fileFromUpload(uploadId: string, filename: string) {
  try {
    const res = await fetch(`/api/v1/uploads/${uploadId}/raw`, { credentials: 'include' })
    if (!res.ok) return null
    const blob = await res.blob()
    return new File([blob], filename || 'asset', { type: blob.type || 'application/octet-stream' })
  } catch {
    return null
  }
}

async function applyHistory(id: string) {
  let job: Job | undefined = store.jobs.find(j => j.id === id)
  try {
    job = await api.job(id)
  } catch {
    if (!job) {
      store.flash('找不到这条历史任务')
      return
    }
  }
  if (!job) return
  form.engine = asEngine(job.engine, job.mode)
  form.mode = job.mode
  form.prompt = job.prompt
  form.duration = job.duration || 5
  form.aspect_ratio = job.aspect_ratio || (form.engine === 'llada-image' ? '1:1' : '16:9')
  form.short_edge = job.short_edge
  form.seed = job.seed
  form.steps = job.steps
  form.flow_shift = job.flow_shift
  form.audio_flow_shift = job.audio_flow_shift
  form.quality = job.quality
  form.enhance_prompt = job.enhance_prompt
  form.priority = job.priority
  firstFrame.value = null
  lastFrame.value = null
  refImage.value = null
  refVideo.value = null
  refAudio.value = null
  for (const asset of job.assets || []) {
    const file = await fileFromUpload(asset.upload_id, asset.filename)
    if (!file) continue
    if (asset.role === 'keyframe' && asset.frame_index === -1) lastFrame.value = file
    else if (asset.role === 'keyframe') firstFrame.value = file
    else if (asset.type === 'video') refVideo.value = file
    else if (asset.type === 'audio') refAudio.value = file
    else refImage.value = file
  }
  store.flash('已载入历史参数')
}

watch(() => String(route.query.reuse || ''), async (id) => {
  if (!id) return
  userPickedEngine.value = true
  await applyHistory(id)
  router.replace({ name: 'studio' })
}, { immediate: true })

watch(
  () => (store.system?.endpoints || []).map(ep => `${ep.name}:${ep.healthy}`).join('|'),
  applyOnlineDefault,
  { immediate: true },
)

onMounted(() => {
  api.system().then(s => { store.system = s }).catch(() => {})
})

async function submit() {
  if (!form.prompt.trim()) {
    store.flash(isLLada.value ? '请填写画面提示' : '请填写镜头提示')
    return
  }
  if (needFirst.value && !firstFrame.value) {
    store.flash('请上传首帧')
    return
  }
  if (needLast.value && !lastFrame.value) {
    store.flash('请上传尾帧')
    return
  }
  if (form.mode === 'ref2va' && !refImage.value && !refVideo.value && !refAudio.value) {
    store.flash('参考生成至少需要一种素材')
    return
  }
  if (form.mode === 'i2i' && !refImage.value) {
    store.flash('指令编辑需要一张参考图')
    return
  }
  if ((form.engine === 'fasth3' || form.engine === 'h3-max') && form.mode !== 't2va') {
    store.flash('本地 FastH3 目前只支持文生影像')
    return
  }
  if (form.engine === 'h3-ref2va-int8' && form.mode !== 'ref2va') {
    store.flash('H3 Ref2VA INT8 只支持参考生成')
    return
  }
  if (form.engine === 'h3' && form.mode === 'ref2va') {
    store.flash('参考生成请改选「H3 Ref2VA INT8」')
    return
  }
  busy.value = true
  try {
    const conditions: any[] = []
    if (needFirst.value && firstFrame.value) {
      const up = await uploadIf(firstFrame.value)
      conditions.push({ upload_id: up?.id, type: 'image', role: 'keyframe', frame_index: 0 })
    }
    if (needLast.value && lastFrame.value) {
      const up = await uploadIf(lastFrame.value)
      conditions.push({ upload_id: up?.id, type: 'image', role: 'keyframe', frame_index: -1 })
    }
    if ((form.mode === 'ref2va' || form.mode === 'i2i') && refImage.value) {
      const up = await uploadIf(refImage.value)
      conditions.push({ upload_id: up?.id, type: 'image', role: 'reference' })
    }
    if (form.mode === 'ref2va') {
      if (refVideo.value) {
        const up = await uploadIf(refVideo.value)
        conditions.push({ upload_id: up?.id, type: 'video', role: 'reference', start_seconds: 0 })
      }
      if (refAudio.value) {
        const up = await uploadIf(refAudio.value)
        conditions.push({ upload_id: up?.id, type: 'audio', role: 'reference' })
      }
    }
    const job = await api.createJob({ ...form, conditions })
    store.upsertJob(job)
    store.flash('已投入队列')
  } catch (err: any) {
    store.flash(err.message || '投递失败')
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.studio { display: grid; grid-template-columns: 1.45fr 0.7fr; gap: 18px; align-items: start; }
.composer, .side { padding: 22px; display: grid; gap: 18px; }
.examples { display: flex; gap: 8px; flex-wrap: wrap; }
.click { cursor: pointer; }
.media { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.grid-3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; margin: 12px 0; }
.adv { border-top: 1px solid var(--line); padding-top: 12px; color: var(--muted); }
.adv summary { cursor: pointer; margin-bottom: 8px; }
.check { display: flex; gap: 8px; align-items: center; font-size: 13px; color: var(--muted); }
.actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.muted { color: var(--muted); font-size: 13px; }
.hint { font-size: 12px; color: var(--faint); line-height: 1.5; }
.ticks { margin-top: 4px; }
.seg button.offline { opacity: 0.58; }
.seg button small {
  display: inline-block;
  margin-left: 6px;
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.04em;
  opacity: 0.8;
}
.item { padding: 12px 0; border-bottom: 1px solid var(--line); }
.item-top { display: flex; justify-content: space-between; gap: 8px; margin-bottom: 6px; }
.bar { margin-top: 8px; height: 5px; background: rgba(255,255,255,0.08); border-radius: 99px; overflow: hidden; }
.bar i { display: block; height: 100%; background: var(--mint); }
.row-head h2 { font-size: 16px; }
input[type=range] { width: 100%; accent-color: var(--mint); }
@media (max-width: 1100px) {
  .studio, .media, .grid-2, .grid-3 { grid-template-columns: 1fr; }
}
</style>
