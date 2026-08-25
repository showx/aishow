<template>
  <div class="studio">
    <section class="panel composer">
      <div class="seg">
        <button v-for="m in modes" :key="m.id" :class="{ active: form.mode === m.id }" @click="form.mode = m.id">{{ m.label }}</button>
      </div>

      <div class="field">
        <label>镜头提示</label>
        <textarea v-model="form.prompt" class="textarea" :placeholder="placeholder"></textarea>
        <div class="examples">
          <button v-for="ex in examples" :key="ex" class="pill click" @click="form.prompt = ex">{{ ex.slice(0, 18) }}…</button>
        </div>
      </div>

      <div class="media" v-if="form.mode !== 't2va'">
        <DropZone v-if="needFirst" v-model="firstFrame" title="首帧" accept="image/*" kind="image" />
        <DropZone v-if="needLast" v-model="lastFrame" title="尾帧" accept="image/*" kind="image" />
        <DropZone v-if="form.mode === 'ref2va'" v-model="refImage" title="参考图" accept="image/*" kind="image" />
        <DropZone v-if="form.mode === 'ref2va'" v-model="refVideo" title="参考视频" accept="video/*" kind="video" />
        <DropZone v-if="form.mode === 'ref2va'" v-model="refAudio" title="参考音频" accept="audio/*" kind="audio" />
      </div>

      <div class="grid-2">
        <div class="field">
          <label>时长 {{ form.duration }}s</label>
          <input v-model.number="form.duration" type="range" min="2" max="15" step="1" />
          <div class="seg ticks">
            <button v-for="d in durations" :key="d" :class="{ active: form.duration === d }" @click="form.duration = d">{{ d }}s</button>
          </div>
        </div>
        <div class="field">
          <label>分辨率 {{ sizeHint }}</label>
          <div class="seg">
            <button
              v-for="r in resolutionPresets"
              :key="r.short"
              :class="{ active: form.short_edge === r.short }"
              @click="form.short_edge = r.short"
            >{{ r.label }}</button>
          </div>
          <div class="hint">NF4 24GB 推荐 480p；720p / 1080p 更慢，也可能撑满显存。</div>
        </div>
      </div>

      <div class="field">
        <label>画幅</label>
        <div class="seg">
          <button v-for="r in ratios" :key="r" :class="{ active: form.aspect_ratio === r }" @click="form.aspect_ratio = r">{{ r }}</button>
        </div>
      </div>

      <details class="adv">
        <summary>高级采样</summary>
        <div class="grid-3">
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
        <label class="check">
          <input v-model="form.enhance_prompt" type="checkbox" />
          使用官方 H3-Context-IR 增强提示（需要 MiniMax Token）
        </label>
      </details>

      <div class="actions">
        <div class="muted">将生成任务投入本地队列，由工位顺序调用 SGLang `/v1/videos`。</div>
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
        <div class="muted">{{ job.stage }} · {{ job.duration }}s · {{ resolutionLabel(job.short_edge) }}</div>
        <div class="bar"><i :style="{ width: job.progress + '%' }"></i></div>
      </div>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import DropZone from '../components/DropZone.vue'
import { api, canvasSize, resolutionLabel, resolutionPresets, statusLabel, type JobMode } from '../api/http'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const busy = ref(false)
const firstFrame = ref<File | null>(null)
const lastFrame = ref<File | null>(null)
const refImage = ref<File | null>(null)
const refVideo = ref<File | null>(null)
const refAudio = ref<File | null>(null)

const modes = [
  { id: 't2va' as JobMode, label: '文生影像' },
  { id: 'i2va' as JobMode, label: '首帧续写' },
  { id: 'l2va' as JobMode, label: '尾帧倒叙' },
  { id: 'fl2va' as JobMode, label: '首尾桥接' },
  { id: 'ref2va' as JobMode, label: '参考生成' },
]
const ratios = ['16:9', '9:16', '1:1', '4:3', '21:9', 'auto']
const durations = [2, 5, 8, 10, 15]
const examples = [
  '夜色卧室：主人熟睡时，三只猫列队闯入吹奏微型铜管，随即若无其事离开。',
  '女舰长背对镜头立于星舰舰桥观景窗前，舰队跃迁的蓝白强光吞没整座舰桥。',
  '日式食堂特写拉面升腾热气，焦点缓缓推向后方热闹的家庭聚餐。',
]
const form = reactive({
  mode: 't2va' as JobMode,
  prompt: examples[0],
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

const needFirst = computed(() => form.mode === 'i2va' || form.mode === 'fl2va')
const needLast = computed(() => form.mode === 'l2va' || form.mode === 'fl2va')
const placeholder = '用镜头语言写：主体、运动、光、声音与时间点。'
const preview = computed(() => store.jobs.slice(0, 8))
const sizeHint = computed(() => {
  const { width, height } = canvasSize(form.aspect_ratio, form.short_edge)
  return `${width}×${height}`
})

async function uploadIf(file: File | null) {
  if (!file) return null
  return api.upload(file)
}

async function submit() {
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
    if (form.mode === 'ref2va') {
      if (refImage.value) {
        const up = await uploadIf(refImage.value)
        conditions.push({ upload_id: up?.id, type: 'image', role: 'reference' })
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
