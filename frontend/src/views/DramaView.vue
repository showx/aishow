<template>
  <div class="drama">
    <aside class="panel rail">
      <div class="rail-head">
        <strong>项目</strong>
        <button class="btn btn-primary btn-sm" type="button" @click="openCreate">新建</button>
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
      <div class="kicker">漫剧制作台</div>
      <h2>从一条题材开拍</h2>
      <p>先定角色和剧本，再拆分镜、出图、成片、拼接。角色本全程可见，分镜条随时跳镜。出图和成片可以穿插做，不必等全部出完图才去成片。</p>
      <ol class="flow">
        <li>写题材，丢人物 / 场景参考</li>
        <li>拆成竖屏分镜，一镜一个瞬间</li>
        <li>出图锁定外形，成片接上动作和声音</li>
      </ol>
      <button class="btn btn-primary" type="button" @click="openCreate">新建项目</button>
    </section>

    <section v-else class="stage">
      <header class="panel producer">
        <div class="who">
          <input v-model="project.title" class="title-input" placeholder="未命名短剧" @change="save" />
          <p class="idea">{{ project.idea }}</p>
        </div>
        <div class="budget">
          <span class="mono">{{ shotSec }}s / {{ project.target_sec }}s</span>
          <i class="bar"><em :style="{ width: budgetPct + '%' }"></em></i>
          <small>{{ shots.length }} 镜 · {{ imageDone }}/{{ shots.length || 0 }} 图 · {{ videoDone }}/{{ shots.length || 0 }} 片</small>
        </div>
        <div class="head-ops">
          <button class="btn" type="button" :disabled="busy" @click="saveCurrent">保存</button>
          <button class="btn btn-danger" type="button" @click="remove">删除</button>
        </div>
      </header>

      <nav class="pipe">
        <button
          v-for="st in steps"
          :key="st.id"
          class="pipe-step"
          :class="{ active: project.step === st.id, done: stepDone(st.id) }"
          :disabled="!!stepLock(st.id)"
          type="button"
          @click="goto(st.id)"
        >
          <span class="no">{{ st.no }}</span>
          <b>{{ st.title }}</b>
          <small>{{ stepMeta(st.id) }}</small>
        </button>
      </nav>
      <p v-if="stepHint" class="hint">{{ stepHint }}</p>
      <p v-if="error" class="err">{{ error }}</p>

      <div v-if="shots.length" class="strip panel">
        <button
          v-for="(shot, i) in shots"
          :key="'st'+shot.index"
          class="cell"
          :class="{ active: focusIndex === i, ok: shot.video_url, img: !shot.video_url && shot.image_url }"
          type="button"
          @click="focusShot(i)"
        >
          <video v-if="shot.video_url" :src="shot.video_url" muted />
          <img v-else-if="shot.image_url" :src="shot.image_url" alt="" />
          <div v-else class="ph">{{ pad(shot.index) }}</div>
          <span>{{ pad(shot.index) }} · {{ shot.duration }}s</span>
        </button>
      </div>

      <div v-if="project.step !== 'compile'" class="panel bible">
        <button class="bible-tog" type="button" @click="bibleOpen = !bibleOpen">
          角色本 · {{ (project.image_refs || []).length }} 张
          <span>{{ bibleOpen ? '收起' : '展开' }}</span>
        </button>
        <div v-if="bibleOpen" class="bible-body">
          <DramaRefsEditor
            :refs="project.image_refs || []"
            compact
            :disabled="busy"
            @files="addProjectRefFiles"
            @remove="removeRef"
            @change="persistProjectRefs"
          />
          <p class="hint">人物、场景参考全程跟着走。LLaDA 出图只吃第一张，其余写进提示；Director / Ref2VA 成片最多带 9 张，可再给单镜加图。</p>
        </div>
      </div>

      <div v-if="project.step === 'write'" class="desk write">
        <div class="panel form">
          <div class="grid-2">
            <div class="field">
              <label>题材 / 意图</label>
              <textarea v-model="project.idea" class="textarea short" placeholder="雨夜便利店，一对前任偶遇" />
            </div>
            <div class="stack">
              <div class="field">
                <label>风格</label>
                <input v-model="project.style" class="input" placeholder="夜雨霓虹、口语对白" />
              </div>
              <div class="field">
                <label>注意事项</label>
                <input v-model="project.style_notes" class="input" placeholder="人物外形、镜头禁忌" />
              </div>
              <div class="field">
                <label>目标 {{ project.target_sec }}s</label>
                <div class="seg">
                  <button v-for="s in durationPresets" :key="s" :class="{ active: project.target_sec === s }" type="button" @click="project.target_sec = s">{{ s }}s</button>
                </div>
              </div>
            </div>
          </div>
          <div class="field">
            <label>剧本</label>
            <textarea v-model="project.script_text" class="textarea script" placeholder="分场写清场景、情绪和对白。可手写，或让本地 Chat 先打一稿。" />
            <p v-if="project.write_model" class="hint">最近一次：{{ project.write_model }}</p>
            <p v-if="project.write_result" class="err">{{ project.write_result }}</p>
          </div>
          <div class="actions sticky">
            <button class="btn" type="button" :disabled="busy" @click="save">保存剧本</button>
            <button class="btn" type="button" :disabled="busy || llmBusy || !project.idea.trim()" @click="generateScript">{{ project.status === 'writing' ? '正在写剧本…' : '生成本地剧本' }}</button>
            <button class="btn btn-primary" type="button" :disabled="busy || llmBusy || !project.script_text.trim()" @click="generateStoryboard">{{ project.status === 'storyboard' ? '正在拆分镜…' : '拆分镜' }}</button>
            <button class="btn" type="button" :disabled="busy || llmBusy || !project.script_text.trim()" @click="goto('storyboard')">只进分镜</button>
          </div>
        </div>
      </div>

      <div v-else-if="project.step === 'storyboard'" class="desk board-desk">
        <div class="panel toolbar">
          <p class="hint">点分镜条选镜，右侧改这一镜。一镜一个瞬间。第 2 镜默认续写上一镜成片。</p>
          <p v-if="project.storyboard_model" class="hint">最近一次拆镜：{{ project.storyboard_model }}</p>
          <p v-if="project.storyboard_result" class="err">{{ project.storyboard_result }}</p>
          <div class="actions">
            <button class="btn" type="button" :disabled="busy || llmBusy || shots.length >= 8" @click="addShot">加一镜</button>
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="fillShots(false)">按时长拆空镜</button>
            <button class="btn" type="button" :disabled="busy || llmBusy || shots.length === 0" @click="fillShots(true)">覆盖重拆</button>
            <button class="btn btn-primary" type="button" :disabled="busy || llmBusy || !project.script_text.trim()" @click="generateStoryboard">{{ project.status === 'storyboard' ? '正在拆分镜…' : '本地拆分镜' }}</button>
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="saveShots">保存分镜</button>
            <button class="btn btn-primary" type="button" :disabled="busy || llmBusy || !storyboardReady" @click="goto('image')">去出图</button>
          </div>
        </div>
        <article v-if="focused" class="panel inspector">
          <header>
            <strong>镜 {{ pad(focused.index) }}</strong>
            <div class="shot-ops">
              <button class="btn btn-sm" type="button" :disabled="focusIndex === 0" @click="moveShot(focusIndex, -1)">上移</button>
              <button class="btn btn-sm" type="button" :disabled="focusIndex === shots.length - 1" @click="moveShot(focusIndex, 1)">下移</button>
              <button class="btn btn-danger btn-sm" type="button" @click="removeShot(focusIndex)">删除</button>
            </div>
          </header>
          <div class="grid-2">
            <div class="field"><label>标题</label><input v-model="focused.title" class="input" /></div>
            <div class="field">
              <label>时长 {{ focused.duration }}s</label>
              <input v-model.number="focused.duration" type="range" min="2" max="15" step="1" />
            </div>
          </div>
          <div class="grid-2">
            <div class="field"><label>画面</label><textarea v-model="focused.scene" class="textarea short" placeholder="这一镜看见什么、情绪如何" /></div>
            <div class="field"><label>对白</label><textarea v-model="focused.dialogue" class="textarea short" placeholder="可选。交给 H3 出声" /></div>
          </div>
          <div class="grid-2">
            <div class="field"><label>出图提示</label><textarea v-model="focused.image_prompt" class="textarea short" placeholder="一个机位、一个瞬间，中文" /></div>
            <div class="field"><label>成片提示</label><textarea v-model="focused.video_prompt" class="textarea short" placeholder="镜头运动、动作、情绪" /></div>
          </div>
          <div class="shot-ops">
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="rewriteShot(focused.index, 'image_prompt')">重写出图提示</button>
            <button class="btn" type="button" :disabled="busy || llmBusy" @click="rewriteShot(focused.index, 'video_prompt')">重写成片提示</button>
            <button class="btn" type="button" :disabled="busy || llmBusy || focused.index === 1" @click="rewriteShot(focused.index, 'continue')">优化衔接</button>
          </div>
          <label class="check"><input v-model="focused.continue_from_prev" type="checkbox" :disabled="focused.index === 1" /> 引用上一镜成片续写</label>
          <label class="check"><input v-model="focused.skip_prev_images" type="checkbox" /> 不要带上一镜人物参考</label>
        </article>
        <div v-else class="panel empty">还没有分镜。按时长拆空镜，或让本地 Chat 拆。</div>
      </div>

      <div v-else-if="project.step === 'image' || project.step === 'video'" class="desk make">
        <div class="panel toolbar">
          <template v-if="project.step === 'image'">
            <div class="grid-2">
              <div class="field">
                <label>档位</label>
                <div class="seg">
                  <button :class="{ active: project.image_quality === 'turbo' }" type="button" @click="project.image_quality = 'turbo'">Turbo</button>
                  <button :class="{ active: project.image_quality === 'base' }" type="button" @click="project.image_quality = 'base'">Base</button>
                </div>
              </div>
              <div class="field">
                <label>画幅 / 短边</label>
                <div class="seg wrap">
                  <button v-for="r in ratios" :key="r" :class="{ active: project.image_aspect === r }" type="button" @click="project.image_aspect = r">{{ r }}</button>
                  <button :class="{ active: project.image_short_edge === 768 }" type="button" @click="project.image_short_edge = 768">768</button>
                  <button :class="{ active: project.image_short_edge === 1024 }" type="button" @click="project.image_short_edge = 1024">1024</button>
                </div>
              </div>
            </div>
            <div class="actions sticky">
              <button class="btn" type="button" :disabled="busy" @click="saveSettings">保存设定</button>
              <button class="btn btn-primary" type="button" :disabled="busy || imageBusy" @click="runImages()">全部出图</button>
              <button class="btn" type="button" :disabled="busy || !imageDone" @click="goto('video')">去成片</button>
            </div>
          </template>
          <template v-else>
            <div class="field">
              <label>首镜引擎</label>
              <div class="seg wrap">
                <button v-for="e in videoEngines" :key="e.id" :class="{ active: project.video_engine === e.id }" type="button" @click="project.video_engine = e.id">{{ e.label }}</button>
              </div>
            </div>
            <div class="field">
              <label>续写引擎</label>
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
                <label>短边</label>
                <div class="seg">
                  <button :class="{ active: project.video_short_edge === 480 }" type="button" @click="project.video_short_edge = 480">480</button>
                  <button :class="{ active: project.video_short_edge === 768 }" type="button" @click="project.video_short_edge = 768">768</button>
                </div>
              </div>
            </div>
            <p class="hint">{{ videoHint }}</p>
            <div class="actions sticky">
              <button class="btn" type="button" :disabled="busy" @click="saveSettings">保存设定</button>
              <button class="btn btn-primary" type="button" :disabled="busy || videoBusy" @click="runVideos()">全部成片</button>
              <button class="btn" type="button" :disabled="busy || !videoDone" @click="goto('compile')">去合成</button>
            </div>
          </template>
        </div>

        <div v-if="focused" class="make-grid">
          <div class="panel preview-wrap">
            <div class="frame" :style="{ aspectRatio: previewRatio }">
              <template v-if="project.step === 'video'">
                <video v-if="focused.video_url" class="preview" :src="focused.video_url" controls />
                <img v-else-if="focused.image_url" class="preview dim" :src="focused.image_url" alt="" />
                <div v-else class="poster">这一镜还没有画面</div>
              </template>
              <template v-else>
                <img v-if="focused.image_url" class="preview" :src="focused.image_url" alt="" />
                <div v-else-if="focused.image_job?.status === 'succeeded'" class="poster">模拟完成，没有真实 PNG</div>
                <div v-else class="poster">选一镜，出这一镜的图</div>
              </template>
            </div>
            <p v-if="focused.last_frame_url && project.step === 'video'" class="hint">已抽尾帧，下一镜可当首帧</p>
            <img v-if="focused.last_frame_url && project.step === 'video'" class="tail" :src="focused.last_frame_url" alt="" />
          </div>
          <article class="panel inspector">
            <header>
              <strong>镜 {{ pad(focused.index) }} {{ focused.title }}</strong>
              <span class="pill" :class="jobClass(project.step === 'video' ? focused.video_job : focused.image_job, project.step === 'video' ? focused.queue_video : focused.queue_image)">
                {{ jobText(project.step === 'video' ? focused.video_job : focused.image_job, project.step === 'video' ? focused.queue_video : focused.queue_image, project.step === 'video' ? '待成片' : '待出图') }}
              </span>
            </header>
            <div class="field">
              <label>{{ project.step === 'video' ? '成片提示' : '出图提示' }}</label>
              <textarea
                v-if="project.step === 'video'"
                v-model="focused.video_prompt"
                class="textarea short"
                placeholder="镜头运动、动作、情绪"
              />
              <textarea
                v-else
                v-model="focused.image_prompt"
                class="textarea short"
                placeholder="一个机位、一个瞬间，中文"
              />
            </div>
            <p v-if="project.step === 'video' && focused.dialogue" class="line">对白：{{ focused.dialogue }}</p>
            <p v-if="focused.image_job?.error_message || focused.video_job?.error_message" class="err">{{ focused.image_job?.error_message || focused.video_job?.error_message }}</p>
            <div class="field">
              <label>本镜额外参考</label>
              <DramaRefsEditor
                :refs="shotRefs(focused)"
                compact
                :max="8"
                add-label="添加本镜参考"
                :disabled="busy"
                @files="files => addShotRefFiles(focused, files)"
                @remove="ri => removeShotRef(focused, ri)"
                @change="saveShots"
              />
              <p v-if="focusIndex > 0 && !focused.skip_prev_images" class="hint">还会引用上一镜成图。</p>
            </div>
            <div class="shot-ops">
              <template v-if="project.step === 'image'">
                <button class="btn btn-primary" type="button" :disabled="busy" @click="runImages([focused.index])">出这一镜</button>
                <button class="btn" type="button" :disabled="busy || !focused.image_job" @click="retryImage(focused.index)">重跑</button>
              </template>
              <template v-else>
                <button class="btn btn-primary" type="button" :disabled="busy" @click="runVideos([focused.index])">成这一镜</button>
                <button class="btn" type="button" :disabled="busy || !focused.video_job" @click="retryVideo(focused.index)">重跑</button>
              </template>
            </div>
          </article>
        </div>
      </div>

      <div v-else class="desk compile">
        <div class="panel form">
          <p class="hint">勾选要进成片的镜头，按序号对齐拼接。左右切分镜条也能预览各镜。</p>
          <div class="timeline">
            <label v-for="shot in shots" :key="'c'+shot.index" class="clip" :class="{ on: compilePick.includes(shot.index), off: !shot.video_url && shot.video_job?.status !== 'succeeded' }">
              <input v-model="compilePick" type="checkbox" :value="shot.index" :disabled="!shot.video_url && shot.video_job?.status !== 'succeeded'" />
              <video v-if="shot.video_url" :src="shot.video_url" muted />
              <img v-else-if="shot.image_url" :src="shot.image_url" alt="" />
              <div v-else class="ph">{{ pad(shot.index) }}</div>
              <span>镜 {{ pad(shot.index) }} · {{ shot.duration }}s</span>
            </label>
          </div>
          <label class="check"><input v-model="project.burn_subtitles" type="checkbox" /> 烧录字幕（各镜对白）</label>
          <label class="check"><input v-model="project.mix_tts" type="checkbox" /> 叠本地语音（推理节点填 TTS；没有则只用 H3 音轨）</label>
          <div class="actions sticky">
            <button class="btn btn-primary" type="button" :disabled="busy || compiling || !compilePick.length" @click="compile">合成全片</button>
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
        <div class="field">
          <label>目标时长 {{ draft.target_sec }}s</label>
          <div class="seg">
            <button v-for="s in durationPresets" :key="'d'+s" :class="{ active: draft.target_sec === s }" type="button" @click="draft.target_sec = s">{{ s }}s</button>
          </div>
        </div>
      </form>
      <template #footer>
        <button class="btn btn-ghost" type="button" :disabled="creating" @click="showCreate = false">取消</button>
        <button class="btn btn-primary" type="submit" form="drama-create" :disabled="creating">创建</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import Modal from '../components/Modal.vue'
import DramaRefsEditor from '../components/DramaRefsEditor.vue'
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
const bibleOpen = ref(true)
const focusIndex = ref(0)
const compilePick = ref<number[]>([])
const draft = reactive({ title: '', idea: '', style: '', style_notes: '', target_sec: 60 })
const durationPresets = [30, 45, 60, 90, 120]
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
const imageBusy = computed(() => shots.value.some(s => s.queue_image || jobBusy(s.image_job)))
const videoBusy = computed(() => shots.value.some(s => s.queue_video || jobBusy(s.video_job)))
const compiling = computed(() => project.value?.status === 'compiling' || project.value?.compile_status === 'compiling')
const llmBusy = computed(() => project.value?.status === 'writing' || project.value?.status === 'storyboard')
const shotSec = computed(() => Math.round(shots.value.reduce((n, s) => n + (s.duration || 0), 0)))
const imageDone = computed(() => shots.value.filter(s => s.image_job?.status === 'succeeded').length)
const videoDone = computed(() => shots.value.filter(s => s.video_job?.status === 'succeeded').length)
const budgetPct = computed(() => {
  const cap = project.value?.target_sec || 1
  return Math.min(100, Math.round((shotSec.value / cap) * 100))
})
const focused = computed(() => shots.value[focusIndex.value] || null)
const previewRatio = computed(() => {
  const a = project.value?.step === 'video' ? project.value.video_aspect : project.value?.image_aspect
  return (a || '9:16').replace(':', ' / ')
})
const stepHint = computed(() => project.value ? stepLock(project.value.step) : '')
const videoUsesImageRefs = computed(() => {
  const first = project.value?.video_engine
  const cont = project.value?.continue_engine
  return first === 'h3-director' || cont === 'h3-director' || cont === 'h3-ref2va-int8'
})
const videoHint = computed(() => {
  if (videoUsesImageRefs.value) {
    return 'Director / Ref2VA 会把本镜出图和角色本一起送进模型（最多 9 张）。续写再加上一镜成片。左右方向键切镜。'
  }
  return 'H3 / Turbo / PinkCherry 用首尾帧，带不了角色本里的额外图。续写会抽上一镜尾帧。左右方向键切镜。'
})

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

watch(() => shots.value.length, n => {
  if (focusIndex.value >= n) focusIndex.value = Math.max(0, n - 1)
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

function stepMeta(id: DramaStep) {
  if (id === 'storyboard') return shots.value.length ? `${shots.value.length} 镜` : ''
  if (id === 'image') return shots.value.length ? `${imageDone.value}/${shots.value.length}` : ''
  if (id === 'video') return shots.value.length ? `${videoDone.value}/${shots.value.length}` : ''
  if (id === 'compile') return project.value?.compile_url ? '已合成' : ''
  return project.value?.script_text?.trim() ? '已写' : ''
}

function stepDone(id: DramaStep) {
  if (id === 'write') return !!project.value?.script_text?.trim()
  if (id === 'storyboard') return storyboardReady.value
  if (id === 'image') return shots.value.length > 0 && imageDone.value === shots.value.length
  if (id === 'video') return shots.value.length > 0 && videoDone.value === shots.value.length
  return !!project.value?.compile_url
}

function parseShots(p?: DramaProject | null): DramaShot[] {
  let list: DramaShot[] = []
  if (p?.shots?.length) list = p.shots
  else if (p?.shots_json) {
    try {
      const raw = JSON.parse(p.shots_json) as DramaShot[]
      list = Array.isArray(raw) ? raw : []
    } catch {
      list = []
    }
  }
  return list.map(s => ({
    ...s,
    image_refs: withRefUrls(s.image_ref_views?.length ? s.image_ref_views : s.image_refs),
  }))
}

function emptyShot(index: number): DramaShot {
  return { index, title: `第 ${index} 镜`, scene: '', dialogue: '', image_prompt: '', video_prompt: '', duration: 5, continue_from_prev: index > 1, beats: [], image_refs: [] }
}

function stepLock(step: DramaStep) {
  const script = project.value?.script_text || ''
  if (step === 'write') return ''
  if (!script.trim()) return '请先填写剧本'
  if (step === 'storyboard') return ''
  if (!shots.value.length) return '请先拆分镜'
  return ''
}

function focusShot(i: number) {
  if (i < 0 || i >= shots.value.length) return
  focusIndex.value = i
  const p = project.value
  if (!p) return
  if (p.step === 'write') return
  if (p.step === 'compile') return
}

function nudgeFocus(dir: number) {
  focusShot(focusIndex.value + dir)
}

function onDeskKey(e: KeyboardEvent) {
  const tag = (e.target as HTMLElement | null)?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
  if (e.key === 'ArrowLeft') {
    e.preventDefault()
    nudgeFocus(-1)
  }
  if (e.key === 'ArrowRight') {
    e.preventDefault()
    nudgeFocus(1)
  }
}

async function loadList() {
  const q = keyword.value.trim() ? `?keyword=${encodeURIComponent(keyword.value.trim())}` : ''
  projects.value = await api.dramaProjects(q)
}

async function select(id: string) {
  selectedId.value = id
  error.value = ''
  project.value = await api.dramaProject(id)
  if (project.value) project.value.image_refs = withRefUrls(project.value.image_refs)
  shots.value = parseShots(project.value)
  compilePick.value = shots.value.filter(s => s.video_url).map(s => s.index)
  focusIndex.value = 0
  bibleOpen.value = true
  armPoll()
}

async function reload() {
  if (!selectedId.value) return
  project.value = await api.dramaProject(selectedId.value)
  if (project.value) project.value.image_refs = withRefUrls(project.value.image_refs)
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
    if (project.value) project.value.image_refs = withRefUrls(project.value.image_refs)
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
  return shots.value.map(({ image_job, extra_image_jobs, video_job, image_url, image_urls, video_url, last_frame_url, image_ref_views, ...shot }) => ({
    ...shot,
    image_refs: serialImageRefs(shot.image_refs),
  }))
}

function serialImageRefs(refs?: DramaImageRef[]) {
  return (refs || []).map(({ url, ...ref }) => ref)
}

function withRefUrls(refs?: DramaImageRef[]) {
  return (refs || []).map(r => ({
    ...r,
    url: r.url || `/api/v1/uploads/${r.upload_id}/raw`,
  }))
}

function shotRefs(shot: DramaShot) {
  if (!shot.image_refs) shot.image_refs = []
  return shot.image_refs
}

async function persistProjectRefs() {
  if (!project.value) return
  await act(() => api.patchDramaProject(project.value!.id, { image_refs: serialImageRefs(project.value!.image_refs) }))
}

async function addProjectRefFiles(files: File[]) {
  if (!project.value || !files.length) return
  if (!project.value.image_refs) project.value.image_refs = []
  const next = [...project.value.image_refs]
  try {
    busy.value = true
    error.value = ''
    for (const file of files) {
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
  }
}

async function addShotRefFiles(shot: DramaShot, files: File[]) {
  if (!files.length) return
  const next = [...(shot.image_refs || [])]
  try {
    busy.value = true
    error.value = ''
    for (const file of files) {
      if (next.length >= 8) break
      const up = await api.upload(file)
      next.push({
        upload_id: up.id,
        name: file.name.replace(/\.[^.]+$/, ''),
        kind: 'character',
        url: `/api/v1/uploads/${up.id}/raw`,
      })
    }
    shot.image_refs = next
    await saveShots()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '上传参考图失败'
    busy.value = false
  }
}

async function removeShotRef(shot: DramaShot, i: number) {
  shot.image_refs = (shot.image_refs || []).filter((_, idx) => idx !== i)
  await saveShots()
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

async function saveCurrent() {
  if (!project.value) return
  if (project.value.step === 'write') return save()
  if (project.value.step === 'storyboard') return saveShots()
  return saveSettings()
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
    image_refs: serialImageRefs(project.value!.image_refs),
    shots: serialShots(),
  }))
}

async function goto(step: DramaStep) {
  if (!project.value) return
  const reason = stepLock(step)
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
  await act(() => api.patchDramaProject(project.value!.id, { shots: serialShots(), image_refs: serialImageRefs(project.value!.image_refs), step }))
}

function addShot() {
  if (shots.value.length >= 8) return
  shots.value = [...shots.value, emptyShot(shots.value.length + 1)]
  focusIndex.value = shots.value.length - 1
}

function removeShot(i: number) {
  shots.value = shots.value.filter((_, idx) => idx !== i).map((s, idx) => ({ ...s, index: idx + 1, continue_from_prev: idx > 0 ? s.continue_from_prev : false }))
  focusIndex.value = Math.min(i, Math.max(0, shots.value.length - 1))
}

function moveShot(i: number, dir: number) {
  const j = i + dir
  if (j < 0 || j >= shots.value.length) return
  const copy = shots.value.slice()
  const tmp = copy[i]
  copy[i] = copy[j]
  copy[j] = tmp
  shots.value = copy.map((s, idx) => ({ ...s, index: idx + 1 }))
  focusIndex.value = j
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
      title: project.value!.title,
      idea: project.value!.idea,
      style: project.value!.style,
      style_notes: project.value!.style_notes,
      target_sec: project.value!.target_sec,
      script_text: project.value!.script_text,
      step: 'storyboard',
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
onMounted(() => window.addEventListener('keydown', onDeskKey))
onUnmounted(() => {
  window.clearInterval(poll)
  window.removeEventListener('keydown', onDeskKey)
})
</script>

<style scoped>
.drama { display: grid; grid-template-columns: 220px 1fr; gap: 16px; align-items: start; }
.rail { padding: 14px; display: grid; gap: 10px; min-height: 480px; align-content: start; }
.rail-head { display: flex; justify-content: space-between; align-items: center; }
.btn-sm { padding: 6px 10px; font-size: 12px; }
.proj {
  text-align: left; border: 1px solid var(--line); background: rgba(0,0,0,0.2);
  border-radius: 12px; padding: 10px 12px; cursor: pointer; display: grid; gap: 4px;
}
.proj b { font-size: 14px; }
.proj span { font-size: 12px; color: var(--muted); }
.proj.active { border-color: var(--mint); background: var(--mint-dim); }
.empty, .empty-main { color: var(--muted); padding: 24px; }
.empty-main { display: grid; gap: 14px; align-content: start; }
.empty-main h2 { font-size: 28px; }
.flow { margin: 0; padding-left: 18px; display: grid; gap: 8px; color: var(--text); }
.stage { display: grid; gap: 12px; min-width: 0; }
.producer { padding: 14px 16px; display: grid; grid-template-columns: 1fr auto auto; gap: 16px; align-items: center; }
.title-input {
  width: 100%; border: 0; background: transparent; font-size: 20px; font-weight: 600;
  font-family: "Sora", "Noto Sans SC", sans-serif; padding: 0;
}
.title-input:focus { outline: none; }
.idea { color: var(--muted); margin-top: 4px; font-size: 13px; }
.budget { display: grid; gap: 6px; min-width: 160px; }
.budget small { color: var(--muted); font-size: 12px; }
.bar { display: block; height: 4px; border-radius: 99px; background: rgba(255,255,255,0.08); overflow: hidden; }
.bar em { display: block; height: 100%; background: var(--mint); }
.head-ops { display: flex; gap: 8px; }
.pipe { display: grid; grid-template-columns: repeat(5, 1fr); gap: 8px; }
.pipe-step {
  display: grid; gap: 2px; text-align: left; padding: 10px 12px; border-radius: 14px;
  border: 1px solid var(--line); background: rgba(0,0,0,0.22); color: var(--muted); cursor: pointer;
}
.pipe-step .no { font-size: 11px; letter-spacing: 0.12em; }
.pipe-step b { color: var(--text); font-size: 14px; }
.pipe-step small { font-size: 11px; color: var(--faint); }
.pipe-step.active { border-color: var(--mint); background: var(--mint-dim); color: var(--mint); }
.pipe-step.done:not(.active) { border-color: rgba(134,239,172,0.35); }
.pipe-step:disabled { opacity: 0.35; cursor: not-allowed; }
.strip { display: flex; gap: 8px; padding: 10px; overflow-x: auto; }
.cell {
  flex: 0 0 72px; display: grid; gap: 4px; padding: 0; border: 1px solid var(--line);
  background: #000; border-radius: 12px; overflow: hidden; cursor: pointer; color: var(--muted); font-size: 11px;
}
.cell img, .cell video, .cell .ph { width: 72px; height: 96px; object-fit: cover; background: #111; }
.cell .ph { display: grid; place-items: center; color: var(--faint); }
.cell span { padding: 0 6px 6px; }
.cell.active { border-color: var(--mint); box-shadow: 0 0 0 2px var(--mint-dim); }
.cell.ok { border-color: rgba(134,239,172,0.45); }
.bible { padding: 10px 14px; }
.bible-tog {
  width: 100%; display: flex; justify-content: space-between; border: 0; background: transparent;
  color: var(--muted); cursor: pointer; padding: 4px 0; font-size: 13px;
}
.bible-body { margin-top: 10px; display: grid; gap: 8px; }
.form, .inspector, .toolbar, .shot { padding: 16px 18px; display: grid; gap: 12px; }
.desk, .board-desk, .make, .compile, .write { display: grid; gap: 12px; }
.make-grid { display: grid; grid-template-columns: minmax(240px, 0.9fr) 1.1fr; gap: 12px; align-items: start; }
.preview-wrap { padding: 12px; }
.frame { width: 100%; max-height: 520px; background: #000; border-radius: 14px; overflow: hidden; display: grid; }
.preview { width: 100%; height: 100%; max-height: 520px; object-fit: contain; background: #000; }
.preview.dim { opacity: 0.75; }
.tail { width: 88px; border-radius: 8px; margin-top: 8px; }
.poster { border: 1px dashed var(--line); border-radius: 12px; padding: 48px 16px; color: var(--faint); text-align: center; }
.inspector header, .shot header { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.shot-ops { display: flex; gap: 8px; flex-wrap: wrap; }
.hint { color: var(--muted); font-size: 13px; }
.err { color: var(--rose); font-size: 13px; }
.line { color: var(--text); font-size: 13px; }
.actions { display: flex; gap: 8px; flex-wrap: wrap; }
.sticky { position: sticky; bottom: 8px; }
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.stack { display: grid; gap: 12px; }
.textarea.short { min-height: 88px; }
.textarea.script { min-height: 280px; }
.check { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 13px; }
.seg.wrap { flex-wrap: wrap; }
.timeline { display: flex; gap: 10px; overflow-x: auto; padding-bottom: 4px; }
.clip {
  flex: 0 0 110px; display: grid; gap: 6px; padding: 8px; border: 1px solid var(--line);
  border-radius: 12px; background: rgba(0,0,0,0.25); color: var(--muted); font-size: 12px; cursor: pointer;
}
.clip img, .clip video, .clip .ph { width: 100%; height: 140px; object-fit: cover; border-radius: 8px; background: #111; }
.clip .ph { display: grid; place-items: center; }
.clip.on { border-color: var(--mint); }
.clip.off { opacity: 0.45; }
.create { display: grid; gap: 12px; }
@media (max-width: 1100px) {
  .drama { grid-template-columns: 1fr; }
  .producer, .make-grid, .grid-2 { grid-template-columns: 1fr; }
  .pipe { grid-template-columns: 1fr 1fr; }
}
</style>
