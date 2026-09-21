<template>
  <div class="studio">
    <section class="panel composer">
      <div class="callout" :class="autoSwitch ? 'ok' : 'warn'">
        <strong>{{ autoSwitch ? '离线也能用，先投入队列即可' : '当前不会自动启动离线模型' }}</strong>
        <p v-if="autoSwitch">「在线」= 已经加载到显存。「可排队」= 还没加载，但现在就能投。后台默认同时只加载 1 个模型：新的起来后，其它边车会被关掉。24GB 请保持这个上限。</p>
        <p v-else>关掉自动切换后，只有「在线」引擎能真正跑起来。离线引擎仍可点选填写，但提交后会失败，除非你先手动开 bat，或重新打开下方自动切换。</p>
      </div>
      <div v-if="reusedTitle" class="callout ok">
        <strong>已载入「{{ reusedTitle }}」的参数</strong>
        <p>原任务不会被覆盖。改提示词、素材或 Seed 后点「投入队列」即可再生成。</p>
        <div class="reuse-ops">
          <button class="btn" type="button" @click="rollSeed">换一个 Seed</button>
          <button v-if="reusedEnhanced" class="btn" type="button" @click="applyEnhancedPrompt">用增强提示</button>
        </div>
      </div>
      <div class="seg">
        <button
          v-for="e in visibleEngines"
          :key="e.id"
          :class="{ active: form.engine === e.id, queued: !e.online && autoSwitch, offline: !e.online && !autoSwitch }"
          @click="pickEngine(e.id)"
        >{{ e.label }}<small>{{ engineBadge(e.online) }}</small></button>
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
        <DropZone v-if="form.mode === 'i2i'" v-model="refImage" title="参考图" accept="image/*" kind="image" />
        <div v-if="form.mode === 'ref2va'" class="refs">
          <DropZone v-model="refImages" title="参考图" accept="image/*" kind="image" multiple :max="9" />
        </div>
        <DropZone v-if="form.mode === 'ref2va'" v-model="refVideo" title="参考视频" accept="video/*" kind="video" />
        <DropZone v-if="form.mode === 'ref2va'" v-model="refAudio" title="参考音频" accept="audio/*" kind="audio" />
      </div>

      <div class="grid-2">
        <div class="field" v-if="!isLLada">
          <label>时长 {{ form.duration }}s</label>
          <input v-model.number="form.duration" type="range" :min="minDuration" :max="maxDuration" step="1" />
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
          <div class="field"><label>Seed <button class="link" type="button" @click="rollSeed">随机</button></label><input v-model.number="form.seed" class="input" type="number"></div>
          <div class="field"><label>Steps</label><input v-model.number="form.steps" class="input" type="number"></div>
          <div class="field"><label>Guidance</label><input v-model.number="form.flow_shift" class="input" type="number" step="0.1"></div>
          <div class="field"><label>优先级</label><input v-model.number="form.priority" class="input" type="number"></div>
        </div>
        <div v-else-if="!isFastH3" class="grid-3">
          <div class="field"><label>Seed <button class="link" type="button" @click="rollSeed">随机</button></label><input v-model.number="form.seed" class="input" type="number"></div>
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
          <div class="field"><label>Seed <button class="link" type="button" @click="rollSeed">随机</button></label><input v-model.number="form.seed" class="input" type="number"></div>
          <div class="field"><label>Steps</label><input class="input" type="number" :value="5" disabled></div>
          <div class="field"><label>优先级</label><input v-model.number="form.priority" class="input" type="number"></div>
        </div>
        <label v-if="!isLLada" class="check">
          <input v-model="form.enhance_prompt" type="checkbox" />
          使用官方 H3-Context-IR 增强提示（需要 MiniMax Token）
        </label>
      </details>

      <div class="actions">
        <div class="action-copy">
          <div class="muted">{{ actionHint }}</div>
          <label class="check switch">
            <input type="checkbox" :checked="autoSwitch" @change="setAutoSwitch(($event.target as HTMLInputElement).checked)" />
            排队时自动切换模型（离线引擎也能投；跑完当前引擎再关旧启新）
          </label>
        </div>
        <button class="btn btn-primary" :disabled="busy || historyBusy" @click="submit">{{ submitLabel }}</button>
      </div>
    </section>

    <aside class="panel side">
      <div class="row-head"><h2>队列预览</h2></div>
      <div v-if="preview.length === 0" class="muted">离线引擎提交后也会出现在这里，轮到时再启动模型。</div>
      <div v-for="job in preview" :key="job.id" class="item">
        <div class="item-top">
          <strong>{{ job.title }}</strong>
          <span class="pill" :class="'status-' + job.status">{{ statusLabel[job.status] }}</span>
        </div>
        <div class="muted">{{ engineName(job.engine) }} · {{ textUnderstandingShort(job) }} · {{ job.stage }} · {{ jobMeta(job) }}</div>
        <div class="muted">{{ jobTime(job) }}</div>
        <div class="bar"><i :style="{ width: job.progress + '%' }"></i></div>
        <div class="item-ops">
          <button class="btn" type="button" @click="fillFromJob(job)">填回</button>
          <button class="btn" type="button" :disabled="isRegenerating(job.id)" @click="regenerate(job)">{{ isRegenerating(job.id) ? '排队中…' : '再生成' }}</button>
          <button class="btn btn-danger" type="button" @click="askDelete(job)">删除</button>
        </div>
      </div>
    </aside>
  </div>

  <JobDeleteModal :job="pending" :busy="deleting" :error="deleteError" @close="closeDelete" @confirm="confirmDelete" />
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DropZone from '../components/DropZone.vue'
import JobDeleteModal from '../components/JobDeleteModal.vue'
import { api, canvasSize, engineName, isImageJob, resolutionLabel, resolutionPresets, statusLabel, textUnderstandingShort, type Job, type JobEngine, type JobMode } from '../api/http'
import { asEngine, randomSeed, useJobReuse } from '../composables/useJobReuse'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const route = useRoute()
const router = useRouter()
const { regenerate, isRegenerating } = useJobReuse()
const busy = ref(false)
const historyBusy = ref(false)
const pending = ref<Job | null>(null)
const deleting = ref(false)
const deleteError = ref('')
const reusedTitle = ref('')
const reusedEnhanced = ref('')
const firstFrame = ref<File | null>(null)
const lastFrame = ref<File | null>(null)
const refImage = ref<File | null>(null)
const refImages = ref<File[]>([])
const refVideo = ref<File | null>(null)
const refAudio = ref<File | null>(null)

const engines = [
  { id: 'h3' as JobEngine, label: 'H3-Base 本地' },
  { id: 'h3-turbo' as JobEngine, label: 'H3 Turbo LoRA' },
  { id: 'h3-pinkcherry-int8' as JobEngine, label: 'H3 PinkCherry INT8' },
  { id: 'h3-ref2va-int8' as JobEngine, label: 'H3 Ref2VA INT8' },
  { id: 'h3-director' as JobEngine, label: 'H3 Timeline Director' },
  { id: 'fasth3' as JobEngine, label: 'FastH3 本地' },
  { id: 'llada-image' as JobEngine, label: 'LLaDA-Image' },
]
const enginePref: JobEngine[] = ['fasth3', 'h3-turbo', 'h3-pinkcherry-int8', 'h3', 'h3-director', 'h3-ref2va-int8', 'llada-image']
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
const directorDurations = [5, 8, 10, 15, 20, 30]
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
    'h3-turbo': false,
    'h3-pinkcherry-int8': false,
    'h3-ref2va-int8': false,
    'h3-director': false,
    'llada-image': false,
  }
  for (const ep of store.system?.endpoints || []) {
    if (!ep.healthy) continue
    const name = (ep.name || '').toLowerCase()
    if (name.includes('fasth3')) map.fasth3 = true
    else if (name.includes('llada')) map['llada-image'] = true
    else if (name.includes('pinkcherry')) map['h3-pinkcherry-int8'] = true
    else if (name.includes('director') || name.includes('timeline')) map['h3-director'] = true
    else if (name.includes('ref2va')) map['h3-ref2va-int8'] = true
    else if (name.includes('turbo')) map['h3-turbo'] = true
    else if (name.includes('fl2va') || name.includes('h3-base')) map.h3 = true
  }
  return map
})
const visibleEngines = computed(() =>
  engines.map(e => ({ ...e, online: !!engineOnline.value[e.id] })),
)

function engineBadge(online: boolean) {
  if (online) return '在线'
  return autoSwitch.value ? '可排队' : '离线'
}

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
const isH3Turbo = computed(() => form.engine === 'h3-turbo')
const isRef2VAInt8 = computed(() => form.engine === 'h3-ref2va-int8')
const isPinkCherry = computed(() => form.engine === 'h3-pinkcherry-int8')
const isDirector = computed(() => form.engine === 'h3-director')
const isLLada = computed(() => form.engine === 'llada-image')
const visibleModes = computed(() => {
  if (isLLada.value) return imageModes
  if (isFastH3.value) return modes.filter(m => m.id === 't2va')
  if (isRef2VAInt8.value) return modes.filter(m => m.id === 'ref2va')
  if (isDirector.value) return modes.filter(m => m.id === 't2va' || m.id === 'ref2va')
  return modes.filter(m => m.id !== 'ref2va')
})
const visibleDurations = computed(() => {
  if (isDirector.value) return directorDurations
  return (isFastH3.value || isH3Turbo.value || isRef2VAInt8.value || isPinkCherry.value) ? [2, 5, 8, 10, 15] : durations
})
const visibleResolutions = computed(() => {
  if (isLLada.value) return imageResolutions
  if (isFastH3.value || isH3Turbo.value || isRef2VAInt8.value || isPinkCherry.value || isDirector.value) return fastResolutions
  return resolutionPresets
})
const visibleRatios = computed(() => {
  if (isLLada.value) return imageRatios
  if (isFastH3.value || isH3Turbo.value || isRef2VAInt8.value || isPinkCherry.value || isDirector.value) return ratios.filter(r => r !== 'auto')
  return ratios
})
const minDuration = computed(() => 2)
const maxDuration = computed(() => isDirector.value ? 30 : 15)
const autoSwitch = computed(() => {
  if (store.settings?.auto_switch_engine !== undefined) return store.settings.auto_switch_engine
  if (store.system?.auto_switch_engine !== undefined) return store.system.auto_switch_engine
  return true
})
const selectedOnline = computed(() => !!engineOnline.value[form.engine])
const engineHint = computed(() => {
  const status = selectedOnline.value
    ? '这个引擎已加载，提交后会马上开跑。'
    : (autoSwitch.value
      ? '这个引擎还没加载，但可以使用：点「投入队列」即可，轮到时会自动启动。'
      : '这个引擎还没加载。先打开自动切换，或手动跑对应 bat，否则提交会失败。')
  if (isLLada.value) return `LLaDA-Image：开源文生图 / 指令编辑。本地 Turbo 权重约 46GB，第一次（或刚切过来）要读盘上 GPU，等几分钟是正常的。${status}`
  if (isFastH3.value) return `FastH3 GGUF Q4：只支持文生。${status}`
  if (isH3Turbo.value) return `H3 Turbo LoRA：NF4 底模 + lightx2v 4 步，文生 / 首尾帧。官方 Space 的未量化版约 72GB 显存，24GB 请用本仓库这条量化路径。${status}`
  if (isPinkCherry.value) return `PinkCherry INT8：独立边车 + 独立 ComfyUI :8189，权重在 H3_PINKCHERRY_ROOT，不和 FastH3 / Ref2VA 共用 8188。文生 / 首尾帧。${status}`
  if (isDirector.value) return `Timeline Director：文生 / 参考生成。有 Latent Upscaler 时走 SelfLift 二采（约 75% 低清 + 25% 高清），用来试能不能更快出片。超过 15 秒会自动分段。${status}`
  if (isRef2VAInt8.value) return `Ref2VA INT8：参考生成，最多 9 张图。${status}`
  return `H3-Base NF4：文生 / 首尾帧。参考生成请改选「H3 Ref2VA INT8」或「H3 Timeline Director」。${status}`
})
const resolutionHint = computed(() => {
  if (isLLada.value) return '文生图边长需能被 16 整除；指令编辑需能被 32 整除。默认 1024。'
  if (isFastH3.value) return '训练分辨率是 768×1344 / 5 秒。24GB 建议先用 480p / 5 秒试一条。'
  if (isH3Turbo.value) return 'LoRA 按 768p 训练。24GB 先用 480p / 5 秒 / 4 步；768p 更吃显存。'
  if (isPinkCherry.value) return 'PinkCherry 独立 INT8。24GB 建议 480p / 5 秒 / 20 步，不要和 FastH3 / Ref2VA 同时加载。'
  if (isDirector.value) return '二采时大部分步数在半分辨率跑。24GB 先用 480p / 5 秒 / 8 步对照原 Ref2VA。超过 15 秒会分段续写。'
  if (isRef2VAInt8.value) return 'INT8 24GB 建议 480p / 5 秒 / 20 步。768p 更吃显存。'
  return 'NF4 24GB 推荐 480p；720p / 1080p 更慢，也可能撑满显存。'
})
const actionHint = computed(() => {
  if (!autoSwitch.value) return '只有当前在线的引擎会真正推理。离线引擎请先开自动切换，或手动启动边车。'
  if (!selectedOnline.value) return '现在就可以投入队列，不必等它变成「在线」。同引擎会先跑完，再切换启动这个。'
  return '已在线，提交后马上推理。也可以继续往队列里堆其他离线引擎的任务。'
})
const submitLabel = computed(() => {
  if (busy.value) return '投递中…'
  if (historyBusy.value) return '载入历史参数…'
  if (autoSwitch.value && !selectedOnline.value) return '投入队列（离线可用）'
  return '投入队列'
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

async function setAutoSwitch(on: boolean) {
  try {
    const base = store.settings || await api.settings()
    const next = await api.saveSettings({ ...base, auto_switch_engine: on })
    store.settings = next
    if (store.system) store.system = { ...store.system, auto_switch_engine: on }
    store.flash(on ? '已开启自动切换模型' : '已关闭自动切换，只跑当前在线的引擎')
  } catch (err: any) {
    store.flash(err.message || '保存失败')
  }
}

function setMode(id: JobMode) {
  form.mode = id
  if (form.engine === 'h3-ref2va-int8' && form.steps === 50) form.steps = 20
  if (form.engine === 'h3-pinkcherry-int8' && form.steps === 50) form.steps = 20
  if (form.engine === 'h3-director' && (form.steps === 50 || form.steps === 20)) form.steps = 8
  if (form.engine === 'h3-turbo' && (form.steps === 50 || form.steps === 0)) form.steps = 4
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
  if (form.flow_shift === 1 || form.flow_shift === 5 || form.flow_shift === 6) form.flow_shift = 12
  if (form.steps === 4) form.steps = 50
  if (id === 'h3-turbo') {
    if (form.mode === 'ref2va') form.mode = 't2va'
    form.steps = 4
    form.flow_shift = 6
    form.quality = 'turbo'
    form.short_edge = form.short_edge >= 640 ? 768 : 480
    if (form.aspect_ratio === 'auto') form.aspect_ratio = '16:9'
    if (form.duration > 15) form.duration = 15
    return
  }
  if (id === 'h3-ref2va-int8') {
    form.mode = 'ref2va'
    form.steps = 20
    form.short_edge = form.short_edge >= 640 ? 768 : 480
    if (form.aspect_ratio === 'auto') form.aspect_ratio = '16:9'
    if (form.duration > 15) form.duration = 15
    return
  }
  if (id === 'h3-director') {
    if (form.mode !== 't2va' && form.mode !== 'ref2va') form.mode = 't2va'
    form.steps = 8
    form.short_edge = form.short_edge >= 640 ? 768 : 480
    if (form.aspect_ratio === 'auto') form.aspect_ratio = '16:9'
    if (form.duration > 30) form.duration = 30
    return
  }
  if (id === 'h3-pinkcherry-int8') {
    if (form.mode === 'ref2va') form.mode = 't2va'
    form.steps = 20
    form.short_edge = form.short_edge >= 640 ? 768 : 480
    if (form.aspect_ratio === 'auto') form.aspect_ratio = '16:9'
    if (form.duration > 15) form.duration = 15
    return
  }
  if (id === 'h3') {
    if (form.mode === 'ref2va') form.mode = 't2va'
    if (form.steps === 20 || form.steps === 8) form.steps = 50
    if (form.duration > 15) form.duration = 15
  }
  if (id === 'fasth3' || id === 'h3-max') {
    form.mode = 't2va'
    form.steps = 4
    form.short_edge = form.short_edge >= 640 ? 768 : 480
    if (form.aspect_ratio === 'auto') form.aspect_ratio = '16:9'
    if (form.duration > 15) form.duration = 15
  }
}

function setLLadaQuality(q: 'turbo' | 'base') {
  form.quality = q
  form.steps = q === 'base' ? 50 : 4
  form.flow_shift = q === 'base' ? 5 : 1
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
  deleting.value = true
  deleteError.value = ''
  try {
    await store.deleteJob(pending.value.id)
    store.flash('已删除')
    pending.value = null
  } catch (err: any) {
    deleteError.value = err.message || '删除失败'
  } finally {
    deleting.value = false
  }
}

function jobMeta(job: Job) {
  const size = `${resolutionLabel(job.short_edge)} · ${job.aspect_ratio}`
  if (isImageJob(job)) return size
  return `${job.duration}s · ${size}`
}

function clock(value?: string | null) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

function took(job: Job) {
  if (!job.started_at || !job.finished_at) return ''
  const start = new Date(job.started_at).getTime()
  const end = new Date(job.finished_at).getTime()
  if (Number.isNaN(start) || Number.isNaN(end)) return ''
  const sec = Math.max(0, Math.floor((end - start) / 1000))
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return m > 0 ? `${m} 分 ${s} 秒` : `${s} 秒`
}

function jobTime(job: Job) {
  if (job.status === 'queued') return `创建 ${clock(job.created_at)}`
  if (job.started_at && job.finished_at) {
    const cost = took(job)
    return `开始 ${clock(job.started_at)} · 完成 ${clock(job.finished_at)}${cost ? ` · 耗时 ${cost}` : ''}`
  }
  if (job.started_at) return `开始 ${clock(job.started_at)}`
  return `创建 ${clock(job.created_at)}`
}

async function uploadIf(file: File | null) {
  if (!file) return null
  return api.upload(file)
}

function rollSeed() {
  form.seed = randomSeed()
}

function applyEnhancedPrompt() {
  if (!reusedEnhanced.value) return
  form.prompt = reusedEnhanced.value
  store.flash('已换成增强提示，可继续修改')
}

async function fillFromJob(job: Job) {
  userPickedEngine.value = true
  await applyHistory(job.id)
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

async function applyHistory(id: string, useEnhanced = false) {
  historyBusy.value = true
  try {
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
    const enhanced = (job.enhanced_prompt || '').trim()
    reusedTitle.value = job.title || '历史任务'
    reusedEnhanced.value = enhanced && enhanced !== job.prompt ? enhanced : ''
    form.prompt = useEnhanced && reusedEnhanced.value ? reusedEnhanced.value : job.prompt
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
    refImages.value = []
    refVideo.value = null
    refAudio.value = null
    const images: File[] = []
    let missing = 0
    for (const asset of job.assets || []) {
      const file = await fileFromUpload(asset.upload_id, asset.filename)
      if (!file) {
        missing += 1
        continue
      }
      if (asset.role === 'keyframe' && asset.frame_index === -1) lastFrame.value = file
      else if (asset.role === 'keyframe') firstFrame.value = file
      else if (asset.type === 'video') refVideo.value = file
      else if (asset.type === 'audio') refAudio.value = file
      else images.push(file)
    }
    refImages.value = images.slice(0, 9)
    refImage.value = images[0] || null
    if (missing) store.flash(`已载入历史参数，有 ${missing} 个素材未能找回`)
    else store.flash('已载入历史参数，可修改后重新生成')
  } finally {
    historyBusy.value = false
  }
}

watch(() => String(route.query.reuse || ''), async (id) => {
  if (!id) return
  userPickedEngine.value = true
  const enhanced = String(route.query.enhanced || '') === '1'
  await applyHistory(id, enhanced)
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
  if (form.mode === 'ref2va' && refImages.value.length === 0 && !refVideo.value && !refAudio.value) {
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
  if ((form.engine === 'h3' || form.engine === 'h3-turbo' || form.engine === 'h3-pinkcherry-int8') && form.mode === 'ref2va') {
    store.flash('参考生成请改选「H3 Ref2VA INT8」或「H3 Timeline Director」')
    return
  }
  if (form.engine === 'h3-director' && form.mode !== 't2va' && form.mode !== 'ref2va') {
    store.flash('H3 Timeline Director 只支持文生和参考生成')
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
    if (form.mode === 'i2i' && refImage.value) {
      const up = await uploadIf(refImage.value)
      conditions.push({ upload_id: up?.id, type: 'image', role: 'reference' })
    }
    if (form.mode === 'ref2va') {
      for (const file of refImages.value.slice(0, 9)) {
        const up = await uploadIf(file)
        if (up?.id) conditions.push({ upload_id: up.id, type: 'image', role: 'reference' })
      }
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
    store.flash(autoSwitch.value && !engineOnline.value[form.engine]
      ? '已排队。引擎离线没关系，轮到时会自动启动。'
      : '已投入队列')
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
.refs { grid-column: 1 / -1; }
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.grid-3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; margin: 12px 0; }
.adv { border-top: 1px solid var(--line); padding-top: 12px; color: var(--muted); }
.adv summary { cursor: pointer; margin-bottom: 8px; }
.check { display: flex; gap: 8px; align-items: center; font-size: 13px; color: var(--muted); }
.actions { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; }
.action-copy { display: grid; gap: 8px; min-width: 0; }
.check.switch { margin-top: 0; }
.muted { color: var(--muted); font-size: 13px; }
.hint { font-size: 12px; color: var(--faint); line-height: 1.5; }
.callout {
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--line);
  line-height: 1.55;
}
.callout.ok { border-color: rgba(94, 234, 212, 0.35); background: var(--mint-dim); }
.callout.warn { border-color: rgba(232, 184, 109, 0.4); background: var(--gold-dim); }
.callout strong { display: block; font-size: 13px; margin-bottom: 4px; }
.callout.ok strong { color: var(--mint); }
.callout.warn strong { color: var(--gold); }
.callout p { margin: 0; font-size: 12px; color: var(--muted); }
.reuse-ops { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.link {
  border: 0; background: transparent; color: var(--mint); cursor: pointer;
  font-size: 11px; padding: 0; letter-spacing: 0.04em; text-transform: none;
}
.ticks { margin-top: 4px; }
.seg button.offline { opacity: 0.7; }
.seg button.queued small { color: var(--gold); }
.seg button.offline small { color: var(--rose); }
.seg button small {
  display: inline-block;
  margin-left: 6px;
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.04em;
  opacity: 0.9;
}
.item { padding: 12px 0; border-bottom: 1px solid var(--line); }
.item-top { display: flex; justify-content: space-between; gap: 8px; margin-bottom: 6px; }
.item-ops { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 10px; }
.bar { margin-top: 8px; height: 5px; background: rgba(255,255,255,0.08); border-radius: 99px; overflow: hidden; }
.bar i { display: block; height: 100%; background: var(--mint); }
.row-head h2 { font-size: 16px; }
input[type=range] { width: 100%; accent-color: var(--mint); }
@media (max-width: 1100px) {
  .studio, .media, .grid-2, .grid-3 { grid-template-columns: 1fr; }
}
</style>
