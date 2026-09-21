<template>
  <div class="drama">
    <aside class="panel list">
      <div class="list-head">
        <strong>项目</strong>
        <button class="btn btn-primary" type="button" @click="openCreate">新建</button>
      </div>
      <input v-model="keyword" class="input" placeholder="搜索题材或标题" @input="loadList" />
      <button
        v-for="item in projects"
        :key="item.id"
        class="proj"
        :class="{ active: selectedId === item.id }"
        type="button"
        @click="select(item.id)"
      >
        <b>{{ item.title }}</b>
        <span>{{ stepLabel[item.step] }} · {{ statusLabel[item.status] || item.status }}</span>
      </button>
      <div v-if="projects.length === 0" class="empty">还没有短剧项目</div>
    </aside>

    <section v-if="!project" class="panel empty-main">
      <h2>从一条题材开始</h2>
      <p>从题材写剧本、拆分镜、LLaDA 出图、本机 H3 成片，最后用 ffmpeg 拼成竖屏短片。剧本和分镜默认走本地 Chat（Ollama 等），也可以手写。不调用远程模型。</p>
      <button class="btn btn-primary" type="button" @click="openCreate">新建项目</button>
    </section>

    <section v-else class="workspace">
      <div class="panel head-card">
        <div>
          <div class="kicker">{{ project.title }}</div>
          <p class="idea">{{ project.idea }}</p>
        </div>
        <div class="head-ops">
          <button class="btn" type="button" :disabled="busy" @click="save">保存</button>
          <button class="btn btn-danger" type="button" @click="remove">删除项目</button>
        </div>
      </div>

      <div class="seg stepper">
        <button
          v-for="st in steps"
          :key="st.id"
          :class="{ active: project.step === st.id }"
          :disabled="!!stepReason(st.id)"
          type="button"
          @click="goto(st.id)"
        >{{ st.no }} {{ st.title }}</button>
      </div>
      <p v-if="stepHint" class="hint">{{ stepHint }}</p>
      <p v-if="error" class="err">{{ error }}</p>

      <div v-if="project.step === 'write'" class="panel form">
        <div class="grid-2">
          <div class="field">
            <label>标题</label>
            <input v-model="project.title" class="input" />
          </div>
          <div class="field">
            <label>目标时长 {{ project.target_sec }}s</label>
            <input v-model.number="project.target_sec" type="range" min="10" max="180" step="5" />
          </div>
        </div>
        <div class="field">
          <label>题材 / 意图</label>
          <textarea v-model="project.idea" class="textarea short" />
        </div>
        <div class="grid-2">
          <div class="field">
            <label>风格</label>
            <input v-model="project.style" class="input" placeholder="例如：夜雨霓虹、口语对白" />
          </div>
          <div class="field">
            <label>风格注意事项</label>
            <input v-model="project.style_notes" class="input" placeholder="人物外形、镜头禁忌等" />
          </div>
        </div>
        <div class="field">
          <label>角色 / 场景参考（最多 16）</label>
          <div class="refs">
            <article v-for="(ref, ri) in (project.image_refs || [])" :key="ref.upload_id" class="ref">
              <img :src="ref.url || `/api/v1/uploads/${ref.upload_id}/raw`" alt="" />
              <input v-model="ref.name" class="input" placeholder="名字" />
              <select v-model="ref.kind" class="select">
                <option value="character">人物</option>
                <option value="scene">场景</option>
                <option value="">未分类</option>
              </select>
              <button class="btn btn-danger" type="button" @click="removeRef(ri)">删除</button>
            </article>
            <label class="btn add-ref" :class="{ disabled: (project.image_refs || []).length >= 16 }">
              添加参考图
              <input type="file" accept="image/*" multiple hidden :disabled="(project.image_refs || []).length >= 16" @change="addRefs" />
            </label>
          </div>
          <p class="hint">出图时第一张参考进 LLaDA 指令编辑，其余名字写进提示。第 2 镜起默认仍带上一镜成图。</p>
        </div>
        <div class="field">
          <label>剧本</label>
          <textarea v-model="project.script_text" class="textarea" placeholder="可手写，或点「生成本地剧本」。分场写清场景、情绪和对白。" />
          <p v-if="project.write_model" class="hint">最近一次：{{ project.write_model }}</p>
          <p v-if="project.write_result" class="err">{{ project.write_result }}</p>
        </div>
        <div class="actions">
          <button class="btn" type="button" :disabled="busy" @click="save">保存剧本</button>
          <button class="btn" type="button" :disabled="busy || llmBusy || !project.idea.trim()" @click="generateScript">{{ project.status === 'writing' ? '正在写剧本…' : '生成本地剧本' }}</button>
          <button class="btn btn-primary" type="button" :disabled="busy || llmBusy || !project.script_text.trim()" @click="goto('storyboard')">下一步：分镜</button>
        </div>
      </div>

      <div v-else-if="project.step === 'storyboard'" class="board">
        <div class="panel form">
          <p class="hint">可按时长拆空镜后手填，或让本地 Chat 按剧本拆成不超过 8 镜。每镜可再重写出图 / 成片 / 衔接提示。</p>
          <p v-if="project.storyboard_model" class="hint">最近一次拆镜：{{ project.storyboard_model }}</p>
          <p v-if="project.storyboard_result" class="err">{{ project.storyboard_result }}</p>
          <div class="actions">
            <button class="btn" type="button" :disabled="busy || llmBusy || shots.length >= 8" @click="addShot">加一镜</button>
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="fillShots(false)">按时长拆空镜</button>
            <button class="btn" type="button" :disabled="busy || llmBusy || shots.length === 0" @click="fillShots(true)">覆盖重拆</button>
            <button class="btn" type="button" :disabled="busy || llmBusy || !project.script_text.trim()" @click="generateStoryboard">{{ project.status === 'storyboard' ? '正在拆分镜…' : '本地拆分镜' }}</button>
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="saveShots">保存分镜</button>
            <button class="btn btn-primary" type="button" :disabled="busy || llmBusy || !storyboardReady" @click="goto('image')">下一步：出图</button>
          </div>
        </div>
        <article v-for="(shot, i) in shots" :key="shot.index" class="panel shot">
          <header>
            <strong>镜 {{ pad(shot.index) }}</strong>
            <div class="shot-ops">
              <button class="btn" type="button" :disabled="i === 0" @click="moveShot(i, -1)">上移</button>
              <button class="btn" type="button" :disabled="i === shots.length - 1" @click="moveShot(i, 1)">下移</button>
              <button class="btn btn-danger" type="button" @click="removeShot(i)">删除</button>
            </div>
          </header>
          <div class="grid-2">
            <div class="field"><label>标题</label><input v-model="shot.title" class="input" /></div>
            <div class="field"><label>时长 {{ shot.duration }}s</label><input v-model.number="shot.duration" type="range" min="2" max="15" step="1" /></div>
          </div>
          <div class="field"><label>画面</label><textarea v-model="shot.scene" class="textarea short" placeholder="这一镜看见什么、情绪如何" /></div>
          <div class="field"><label>对白</label><textarea v-model="shot.dialogue" class="textarea short" placeholder="可选。会写进成片提示，交给 H3 出声" /></div>
          <div class="field"><label>出图提示</label><textarea v-model="shot.image_prompt" class="textarea short" placeholder="一个机位、一个瞬间，中文" /></div>
          <div class="field"><label>成片提示</label><textarea v-model="shot.video_prompt" class="textarea short" placeholder="镜头运动、动作、情绪。第 2 镜起可开续写" /></div>
          <div class="shot-ops">
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="rewriteShot(shot.index, 'image_prompt')">重写出图</button>
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="rewriteShot(shot.index, 'video_prompt')">重写成片</button>
            <button class="btn" type="button" :disabled="busy || llmBusy || shot.index === 1" @click="rewriteShot(shot.index, 'continue')">优化衔接</button>
          </div>
          <label class="check"><input v-model="shot.continue_from_prev" type="checkbox" :disabled="shot.index === 1" /> 引用上一镜成片续写</label>
          <label class="check"><input v-model="shot.skip_prev_images" type="checkbox" /> 出图时不要带上一镜人物参考</label>
        </article>
      </div>

      <div v-else-if="project.step === 'image'" class="board">
        <div class="panel form">
          <div class="grid-2">
            <div class="field">
              <label>出图引擎</label>
              <div class="seg"><button class="active" type="button">LLaDA-Image</button></div>
            </div>
            <div class="field">
              <label>档位</label>
              <div class="seg">
                <button :class="{ active: project.image_quality === 'turbo' }" type="button" @click="project.image_quality = 'turbo'">Turbo</button>
                <button :class="{ active: project.image_quality === 'base' }" type="button" @click="project.image_quality = 'base'">Base</button>
              </div>
            </div>
          </div>
          <div class="field">
            <label>画幅 / 短边</label>
            <div class="seg">
              <button v-for="r in ratios" :key="r" :class="{ active: project.image_aspect === r }" type="button" @click="project.image_aspect = r">{{ r }}</button>
            </div>
            <div class="seg" style="margin-top:8px">
              <button :class="{ active: project.image_short_edge === 768 }" type="button" @click="project.image_short_edge = 768">768</button>
              <button :class="{ active: project.image_short_edge === 1024 }" type="button" @click="project.image_short_edge = 1024">1024</button>
            </div>
          </div>
          <div class="actions">
            <button class="btn" type="button" :disabled="busy" @click="saveSettings">保存设定</button>
            <button class="btn btn-primary" type="button" :disabled="busy || imageBusy" @click="runImages()">一键出图（串行）</button>
            <button class="btn" type="button" :disabled="busy || !imagesReady" @click="goto('video')">下一步：成片</button>
          </div>
          <p class="hint">24GB 会先把 LLaDA 挂满，一镜成图后再把上一镜当人物参考开下一镜。</p>
        </div>
        <article v-for="shot in shots" :key="'img-'+shot.index" class="panel shot">
          <header>
            <strong>镜 {{ pad(shot.index) }} {{ shot.title }}</strong>
            <span class="pill" :class="jobClass(shot.image_job, shot.queue_image)">{{ jobText(shot.image_job, shot.queue_image, '待出图') }}</span>
          </header>
          <p class="prompt">{{ shot.image_prompt || shot.scene || '（缺少出图提示）' }}</p>
          <img v-if="shot.image_url" class="preview" :src="shot.image_url" alt="" />
          <div v-else-if="shot.image_job?.status === 'succeeded'" class="poster">模拟完成，没有真实 PNG</div>
          <p v-if="shot.image_job?.error_message" class="err">{{ shot.image_job.error_message }}</p>
          <div class="shot-ops">
            <button class="btn" type="button" :disabled="busy" @click="runImages([shot.index])">只出这一镜</button>
            <button class="btn" type="button" :disabled="busy || !shot.image_job" @click="retryImage(shot.index)">重跑</button>
          </div>
        </article>
      </div>

      <div v-else-if="project.step === 'video'" class="board">
        <div class="panel form">
          <div class="field">
            <label>首镜引擎</label>
            <div class="seg wrap">
              <button v-for="e in videoEngines" :key="e.id" :class="{ active: project.video_engine === e.id }" type="button" @click="project.video_engine = e.id">{{ e.label }}</button>
            </div>
          </div>
          <div class="field">
            <label>续写引擎（第 2 镜起）</label>
            <div class="seg wrap">
              <button v-for="e in continueEngines" :key="e.id" :class="{ active: project.continue_engine === e.id }" type="button" @click="project.continue_engine = e.id">{{ e.label }}</button>
            </div>
          </div>
          <div class="grid-2">
            <div class="field">
              <label>画幅</label>
              <div class="seg">
                <button v-for="r in ratios" :key="'v'+r" :class="{ active: project.video_aspect === r }" type="button" @click="project.video_aspect = r">{{ r }}</button>
              </div>
            </div>
            <div class="field">
              <label>短边 {{ project.video_short_edge }}</label>
              <div class="seg">
                <button :class="{ active: project.video_short_edge === 480 }" type="button" @click="project.video_short_edge = 480">480</button>
                <button :class="{ active: project.video_short_edge === 768 }" type="button" @click="project.video_short_edge = 768">768</button>
              </div>
            </div>
          </div>
          <div class="actions">
            <button class="btn" type="button" :disabled="busy" @click="saveSettings">保存设定</button>
            <button class="btn btn-primary" type="button" :disabled="busy || videoBusy" @click="runVideos()">一键成片（串行）</button>
            <button class="btn" type="button" :disabled="busy || !someVideoReady" @click="goto('compile')">下一步：合成</button>
          </div>
          <p class="hint">默认首镜 H3 首帧。续写选 Ref2VA / Director 用上一镜成片；选 H3 / Turbo / PinkCherry 会抽上一镜尾帧当本镜首帧。对白写进提示，由 H3 出声。</p>
        </div>
        <article v-for="shot in shots" :key="'vid-'+shot.index" class="panel shot">
          <header>
            <strong>镜 {{ pad(shot.index) }} {{ shot.title }}</strong>
            <span class="pill" :class="jobClass(shot.video_job, shot.queue_video)">{{ jobText(shot.video_job, shot.queue_video, '待成片') }}</span>
          </header>
          <p class="prompt">{{ shot.video_prompt || shot.scene || shot.image_prompt }}</p>
          <video v-if="shot.video_url" class="preview" :src="shot.video_url" controls />
          <div v-else-if="shot.video_job?.status === 'succeeded'" class="poster">模拟完成，没有真实 MP4</div>
          <img v-else-if="shot.image_url" class="preview dim" :src="shot.image_url" alt="" />
          <p v-if="shot.last_frame_url" class="hint">已抽尾帧，下一镜可当首帧续写</p>
          <img v-if="shot.last_frame_url" class="preview dim" :src="shot.last_frame_url" alt="" />
          <p v-if="shot.video_job?.error_message" class="err">{{ shot.video_job.error_message }}</p>
          <div class="shot-ops">
            <button class="btn" type="button" :disabled="busy" @click="runVideos([shot.index])">只做这一镜</button>
            <button class="btn" type="button" :disabled="busy || !shot.video_job" @click="retryVideo(shot.index)">重跑</button>
          </div>
        </article>
      </div>

      <div v-else class="board">
        <div class="panel form">
          <p class="hint">勾选已成片镜头，按序号用 ffmpeg 转码对齐后拼接。模拟任务没有真实文件，合不了。</p>
          <label v-for="shot in shots" :key="'c'+shot.index" class="check">
            <input v-model="compilePick" type="checkbox" :value="shot.index" :disabled="!shot.video_url && shot.video_job?.status !== 'succeeded'" />
            镜 {{ pad(shot.index) }} {{ shot.title }}
            <span v-if="shot.video_url">· 有成片</span>
            <span v-else-if="shot.video_job?.status === 'succeeded'">· 模拟</span>
            <span v-else>· 未完成</span>
          </label>
          <label class="check"><input v-model="project.burn_subtitles" type="checkbox" /> 烧录字幕（用各镜对白）</label>
          <label class="check"><input v-model="project.mix_tts" type="checkbox" /> 叠本地语音（需在推理节点填 TTS；没有则只用 H3 音轨）</label>
          <div class="actions">
            <button class="btn btn-primary" type="button" :disabled="busy || compiling" @click="compile">合成全片</button>
          </div>
          <p v-if="project.compile_result" class="err">{{ project.compile_result }}</p>
        </div>
        <div v-if="project.compile_url || project.compile_job_id" class="panel shot">
          <header><strong>全成短片</strong><span class="pill status-succeeded">{{ project.compile_status || 'done' }}</span></header>
          <video class="preview" :src="project.compile_url || `/api/v1/drama-projects/${project.id}/video`" controls />
        </div>
      </div>
    </section>

    <Modal :open="showCreate" title="新建短剧" kicker="Drama" :busy="creating" @close="showCreate = false">
      <form id="drama-create" class="create" @submit.prevent="create">
        <div class="field"><label>标题</label><input v-model="draft.title" class="input" placeholder="可空，默认截取题材" /></div>
        <div class="field"><label>题材 / 意图</label><textarea v-model="draft.idea" class="textarea short" required placeholder="必填，例如：雨夜便利店，一对前任偶遇" /></div>
        <div class="field"><label>风格</label><input v-model="draft.style" class="input" /></div>
        <div class="field"><label>风格注意事项</label><input v-model="draft.style_notes" class="input" /></div>
        <div class="field"><label>目标时长 {{ draft.target_sec }}s</label><input v-model.number="draft.target_sec" type="range" min="10" max="180" step="5" /></div>
      </form>
      <template #footer>
        <button class="btn btn-ghost" type="button" :disabled="creating" @click="showCreate = false">取消</button>
        <button class="btn btn-primary" type="submit" form="drama-create" :disabled="creating">创建</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import Modal from '../components/Modal.vue'
import { api, statusLabel as jobStatusLabel, type DramaImageRef, type DramaProject, type DramaShot, type DramaStep, type Job, type JobEngine } from '../api/http'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const projects = ref<DramaProject[]>([])
const project = ref<DramaProject | null>(null)
const shots = ref<DramaShot[]>([])
const selectedId = ref('')
const keyword = ref('')
const busy = ref(false)
const creating = ref(false)
const error = ref('')
const showCreate = ref(false)
const compilePick = ref<number[]>([])
const draft = reactive({ title: '', idea: '', style: '', style_notes: '', target_sec: 60 })
let poll = 0

const steps: { id: DramaStep; no: string; title: string }[] = [
  { id: 'write', no: '01', title: '剧本' },
  { id: 'storyboard', no: '02', title: '分镜' },
  { id: 'image', no: '03', title: '出图' },
  { id: 'video', no: '04', title: '成片' },
  { id: 'compile', no: '05', title: '合成' },
]
const stepLabel: Record<DramaStep, string> = { write: '剧本', storyboard: '分镜', image: '出图', video: '成片', compile: '合成' }
const statusLabel: Record<string, string> = { draft: '草稿', writing: '写剧本', storyboard: '拆分镜', imaging: '出图中', videoing: '成片中', compiling: '合成中', done: '完成', failed: '失败' }
const ratios = ['9:16', '16:9', '1:1', '3:4', '4:3']
const videoEngines: { id: JobEngine; label: string }[] = [
  { id: 'h3', label: 'H3-Base' },
  { id: 'h3-turbo', label: 'Turbo' },
  { id: 'h3-pinkcherry-int8', label: 'PinkCherry' },
  { id: 'h3-director', label: 'Director' },
  { id: 'fasth3', label: 'FastH3' },
]
const continueEngines: { id: JobEngine; label: string }[] = [
  { id: 'h3-ref2va-int8', label: 'Ref2VA' },
  { id: 'h3-director', label: 'Director' },
  { id: 'h3', label: 'H3 首帧' },
  { id: 'h3-turbo', label: 'Turbo 首帧' },
  { id: 'h3-pinkcherry-int8', label: 'PinkCherry 首帧' },
]

const storyboardReady = computed(() => shots.value.length > 0 && shots.value.every(s => !!(s.scene?.trim() || s.image_prompt?.trim())))
const imagesReady = computed(() => shots.value.length > 0 && shots.value.every(s => s.image_job?.status === 'succeeded'))
const someVideoReady = computed(() => shots.value.some(s => s.video_job?.status === 'succeeded'))
const imageBusy = computed(() => shots.value.some(s => s.queue_image || jobBusy(s.image_job)))
const videoBusy = computed(() => shots.value.some(s => s.queue_video || jobBusy(s.video_job)))
const compiling = computed(() => project.value?.status === 'compiling' || project.value?.compile_status === 'compiling')
const llmBusy = computed(() => project.value?.status === 'writing' || project.value?.status === 'storyboard')
const stepHint = computed(() => project.value ? stepReason(project.value.step) : '')

watch(() => store.jobs.map(j => `${j.id}:${j.status}:${j.progress}`).join('|'), () => {
  if (project.value && shouldPoll(project.value)) reload().catch(() => {})
})

watch(() => [selectedId.value, project.value?.status] as const, ([id, status], [prevId, prevStatus]) => {
  if (!id || id !== prevId || !project.value) return
  if (prevStatus === 'writing' && status === 'draft') store.flash('本地剧本已写好')
  if (prevStatus === 'storyboard' && status === 'draft') store.flash('本地分镜已拆好')
  if (status === 'failed') {
    error.value = project.value.write_result || project.value.storyboard_result || '本地 Chat 失败'
  }
})

function shouldPoll(p: DramaProject) {
  if (p.status === 'writing' || p.status === 'storyboard' || p.status === 'imaging' || p.status === 'videoing' || p.status === 'compiling' || p.compile_status === 'compiling') return true
  return (p.shots || shots.value).some(s => jobBusy(s.image_job) || jobBusy(s.video_job) || s.queue_image || s.queue_video)
}

function jobBusy(job?: Job) {
  return !!job && (job.status === 'queued' || job.status === 'running')
}

function jobText(job: Job | undefined, queued: boolean | undefined, idle: string) {
  if (queued && !job) return '排队等上一镜'
  if (!job) return idle
  const extra = job.status === 'running' ? ` ${job.progress || 0}%` : ''
  return (jobStatusLabel[job.status] || job.status) + extra
}

function jobClass(job: Job | undefined, queued?: boolean) {
  if (queued && !job) return 'status-queued'
  if (!job) return ''
  return 'status-' + job.status
}

function pad(n: number) {
  return String(n).padStart(2, '0')
}

function parseShots(p?: DramaProject | null): DramaShot[] {
  if (p?.shots?.length) return p.shots
  if (!p?.shots_json) return []
  try {
    const raw = JSON.parse(p.shots_json) as DramaShot[]
    return Array.isArray(raw) ? raw : []
  } catch {
    return []
  }
}

function emptyShot(index: number): DramaShot {
  return { index, title: `第 ${index} 镜`, scene: '', dialogue: '', image_prompt: '', video_prompt: '', duration: 5, continue_from_prev: index > 1, beats: [] }
}

function stepReason(step: DramaStep) {
  const script = project.value?.script_text || ''
  if (step === 'write') return ''
  if (!script.trim()) return '请先填写剧本'
  if (step === 'storyboard') return ''
  if (!storyboardReady.value) return shots.value.length ? '还有分镜不完整' : '请先拆分镜'
  if (step === 'image') return ''
  if (!imagesReady.value) return '请先让每一镜都出完图'
  if (step === 'video') return ''
  if (!someVideoReady.value) return '请先完成至少一镜成片'
  return ''
}

async function loadList() {
  const q = keyword.value.trim() ? `?keyword=${encodeURIComponent(keyword.value.trim())}` : ''
  projects.value = await api.dramaProjects(q)
}

async function select(id: string) {
  selectedId.value = id
  error.value = ''
  project.value = await api.dramaProject(id)
  shots.value = parseShots(project.value)
  compilePick.value = shots.value.filter(s => s.video_url).map(s => s.index)
  armPoll()
}

async function reload() {
  if (!selectedId.value) return
  project.value = await api.dramaProject(selectedId.value)
  shots.value = parseShots(project.value)
}

function armPoll() {
  window.clearInterval(poll)
  poll = window.setInterval(() => {
    if (project.value && shouldPoll(project.value)) reload().catch(() => {})
  }, 4000)
}

function openCreate() {
  draft.title = ''
  draft.idea = ''
  draft.style = ''
  draft.style_notes = ''
  draft.target_sec = 60
  showCreate.value = true
}

async function create() {
  creating.value = true
  error.value = ''
  try {
    const p = await api.createDramaProject({ ...draft })
    showCreate.value = false
    await loadList()
    await select(p.id)
    store.flash('已创建短剧项目')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建失败'
  } finally {
    creating.value = false
  }
}

async function act(fn: () => Promise<DramaProject>) {
  busy.value = true
  error.value = ''
  try {
    project.value = await fn()
    shots.value = parseShots(project.value)
    await loadList()
    armPoll()
    return true
  } catch (err) {
    error.value = err instanceof Error ? err.message : '操作失败'
    return false
  } finally {
    busy.value = false
  }
}

function serialShots() {
  return shots.value.map(({ image_job, extra_image_jobs, video_job, image_url, image_urls, video_url, last_frame_url, image_ref_views, ...shot }) => shot)
}

async function save() {
  if (!project.value) return
  await act(() => api.patchDramaProject(project.value!.id, {
    title: project.value!.title,
    idea: project.value!.idea,
    style: project.value!.style,
    style_notes: project.value!.style_notes,
    target_sec: project.value!.target_sec,
    script_text: project.value!.script_text,
    image_refs: serialImageRefs(project.value!.image_refs),
  }))
}

function serialImageRefs(refs?: DramaImageRef[]) {
  return (refs || []).map(({ url, ...ref }) => ref)
}

async function addRefs(ev: Event) {
  const input = ev.target as HTMLInputElement
  if (!project.value || !input.files?.length) return
  const next = [...(project.value.image_refs || [])]
  try {
    busy.value = true
    error.value = ''
    for (const file of Array.from(input.files)) {
      if (next.length >= 16) break
      const up = await api.upload(file)
      next.push({
        upload_id: up.id,
        name: file.name.replace(/\.[^.]+$/, ''),
        kind: 'character',
        url: `/api/v1/uploads/${up.id}/raw`,
      })
    }
    project.value.image_refs = next
    await act(() => api.patchDramaProject(project.value!.id, { image_refs: serialImageRefs(next) }))
  } catch (err) {
    error.value = err instanceof Error ? err.message : '上传参考图失败'
  } finally {
    busy.value = false
    input.value = ''
  }
}

async function removeRef(i: number) {
  if (!project.value) return
  const next = (project.value.image_refs || []).filter((_, idx) => idx !== i)
  project.value.image_refs = next
  await act(() => api.patchDramaProject(project.value!.id, { image_refs: serialImageRefs(next) }))
}

async function saveShots() {
  if (!project.value) return
  await act(() => api.patchDramaProject(project.value!.id, { shots: serialShots() }))
}

async function saveSettings() {
  if (!project.value) return false
  return act(() => api.patchDramaProject(project.value!.id, {
    image_engine: project.value!.image_engine,
    image_aspect: project.value!.image_aspect,
    image_short_edge: project.value!.image_short_edge,
    image_quality: project.value!.image_quality,
    video_engine: project.value!.video_engine,
    continue_engine: project.value!.continue_engine,
    video_aspect: project.value!.video_aspect,
    video_short_edge: project.value!.video_short_edge,
    video_duration: project.value!.video_duration,
    shots: serialShots(),
  }))
}

async function goto(step: DramaStep) {
  if (!project.value) return
  const reason = stepReason(step)
  if (reason && step !== project.value.step) {
    error.value = reason
    return
  }
  if (project.value.step === 'write') {
    await act(() => api.patchDramaProject(project.value!.id, {
      title: project.value!.title,
      idea: project.value!.idea,
      style: project.value!.style,
      style_notes: project.value!.style_notes,
      target_sec: project.value!.target_sec,
      script_text: project.value!.script_text,
      image_refs: serialImageRefs(project.value!.image_refs),
      step,
    }))
    return
  }
  if (project.value.step === 'storyboard') {
    await act(() => api.patchDramaProject(project.value!.id, { shots: serialShots(), step }))
    return
  }
  await act(() => api.patchDramaProject(project.value!.id, { step }))
}

function addShot() {
  if (shots.value.length >= 8) return
  shots.value = [...shots.value, emptyShot(shots.value.length + 1)]
}

function removeShot(i: number) {
  shots.value = shots.value.filter((_, idx) => idx !== i).map((s, idx) => ({ ...s, index: idx + 1, continue_from_prev: idx > 0 ? s.continue_from_prev : false }))
}

function moveShot(i: number, dir: number) {
  const j = i + dir
  if (j < 0 || j >= shots.value.length) return
  const copy = shots.value.slice()
  const tmp = copy[i]
  copy[i] = copy[j]
  copy[j] = tmp
  shots.value = copy.map((s, idx) => ({ ...s, index: idx + 1 }))
}

async function fillShots(replace: boolean) {
  if (!project.value) return
  await act(() => api.dramaStoryboard(project.value!.id, { replace }))
}

async function generateScript() {
  if (!project.value) return
  await act(async () => {
    await api.patchDramaProject(project.value!.id, {
      title: project.value!.title,
      idea: project.value!.idea,
      style: project.value!.style,
      style_notes: project.value!.style_notes,
      target_sec: project.value!.target_sec,
      script_text: project.value!.script_text,
    })
    return api.dramaWrite(project.value!.id)
  })
}

async function generateStoryboard() {
  if (!project.value) return
  if (shots.value.length && !window.confirm('本地拆分镜会覆盖现有分镜，继续？')) return
  await act(async () => {
    await api.patchDramaProject(project.value!.id, {
      style: project.value!.style,
      style_notes: project.value!.style_notes,
      script_text: project.value!.script_text,
    })
    return api.dramaStoryboard(project.value!.id, { mode: 'llm', replace: true })
  })
}

async function rewriteShot(index: number, target: 'image_prompt' | 'video_prompt' | 'continue') {
  if (!project.value) return
  await act(async () => {
    await api.patchDramaProject(project.value!.id, { shots: serialShots() })
    return api.dramaRewrite(project.value!.id, index, target)
  })
}

async function runImages(indexes?: number[]) {
  if (!project.value) return
  if (!await saveSettings()) return
  await act(() => api.dramaImages(project.value!.id, { indexes, queue: !indexes || indexes.length > 1 }))
}

async function retryImage(index: number) {
  if (!project.value) return
  await act(() => api.retryDramaImage(project.value!.id, index))
}

async function runVideos(indexes?: number[]) {
  if (!project.value) return
  if (!await saveSettings()) return
  await act(() => api.dramaVideos(project.value!.id, { indexes, queue: !indexes || indexes.length > 1 }))
}

async function retryVideo(index: number) {
  if (!project.value) return
  await act(() => api.retryDramaVideo(project.value!.id, index))
}

async function compile() {
  if (!project.value) return
  await act(() => api.dramaCompile(project.value!.id, {
    indexes: compilePick.value,
    burn_subtitles: !!project.value!.burn_subtitles,
    mix_tts: !!project.value!.mix_tts,
  }))
}

async function remove() {
  if (!project.value) return
  if (!window.confirm(`删除项目「${project.value.title}」？已生成的任务仍留在队列/作品库。`)) return
  busy.value = true
  try {
    await api.deleteDramaProject(project.value.id)
    project.value = null
    shots.value = []
    selectedId.value = ''
    await loadList()
    store.flash('项目已删除')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除失败'
  } finally {
    busy.value = false
  }
}

loadList().catch(err => { error.value = err instanceof Error ? err.message : '加载失败' })
onUnmounted(() => window.clearInterval(poll))
</script>

<style scoped>
.drama { display: grid; grid-template-columns: 280px 1fr; gap: 18px; align-items: start; }
.list { padding: 16px; display: grid; gap: 10px; min-height: 480px; }
.list-head { display: flex; justify-content: space-between; align-items: center; }
.proj {
  text-align: left; border: 1px solid var(--line); background: rgba(0,0,0,0.2);
  border-radius: 12px; padding: 10px 12px; cursor: pointer; display: grid; gap: 4px;
}
.proj b { font-size: 14px; }
.proj span { font-size: 12px; color: var(--muted); }
.proj.active { border-color: var(--mint); background: var(--mint-dim); }
.empty, .empty-main { color: var(--muted); padding: 24px; }
.empty-main { display: grid; gap: 12px; align-content: start; }
.workspace { display: grid; gap: 14px; min-width: 0; }
.head-card { padding: 16px 18px; display: flex; justify-content: space-between; gap: 16px; align-items: start; }
.idea { color: var(--muted); margin-top: 6px; }
.head-ops { display: flex; gap: 8px; }
.stepper button:disabled { opacity: 0.35; }
.form, .shot { padding: 16px 18px; display: grid; gap: 12px; }
.board { display: grid; gap: 12px; }
.shot header { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.shot-ops { display: flex; gap: 8px; flex-wrap: wrap; }
.prompt { color: var(--muted); white-space: pre-wrap; }
.preview { width: 100%; max-height: 420px; object-fit: contain; background: #000; border-radius: 12px; }
.preview.dim { opacity: 0.7; max-height: 220px; }
.poster { border: 1px dashed var(--line); border-radius: 12px; padding: 24px; color: var(--faint); text-align: center; }
.hint { color: var(--muted); font-size: 13px; }
.err { color: var(--rose); font-size: 13px; }
.actions { display: flex; gap: 8px; flex-wrap: wrap; }
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.textarea.short { min-height: 88px; }
.check { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 13px; }
.refs { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 10px; }
.ref { display: grid; gap: 6px; border: 1px solid var(--line); border-radius: 12px; padding: 8px; background: rgba(0,0,0,0.2); }
.ref img { width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 8px; background: #000; }
.add-ref { display: grid; place-items: center; min-height: 140px; cursor: pointer; }
.add-ref.disabled { opacity: 0.4; pointer-events: none; }
.seg.wrap { flex-wrap: wrap; }
.create { display: grid; gap: 12px; }
@media (max-width: 980px) {
  .drama { grid-template-columns: 1fr; }
  .grid-2 { grid-template-columns: 1fr; }
}
</style>
