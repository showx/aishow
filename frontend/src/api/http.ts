export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
export type JobMode = 't2va' | 'i2va' | 'l2va' | 'fl2va' | 'ref2va' | 't2i' | 'i2i'
export type JobEngine = 'h3' | 'fasth3' | 'h3-max' | 'h3-turbo' | 'h3-ref2va-int8' | 'h3-pinkcherry-int8' | 'h3-director' | 'llada-image' | 'qwen-image' | 'hunyuan-video' | 'ltx-2.3'

export interface JobAsset {
  id: string
  job_id: string
  upload_id: string
  role: string
  type: string
  frame_index?: number | null
  start_seconds?: number | null
  uri: string
  filename: string
}

export interface JobEvent {
  id: number
  job_id: string
  level: string
  message: string
  created_at: string
}

export interface Job {
  id: string
  title: string
  mode: JobMode
  engine?: JobEngine
  status: JobStatus
  priority: number
  prompt: string
  enhanced_prompt: string
  text_encoder?: string
  text_encoder_label?: string
  prompt_rewriter?: string
  duration: number
  aspect_ratio: string
  short_edge: number
  seed: number
  steps: number
  flow_shift: number
  audio_flow_shift: number
  quality: string
  enhance_prompt: boolean
  outputs: number
  progress: number
  stage: string
  error_message: string
  remote_id: string
  output_size: number
  has_video: boolean
  has_image: boolean
  queue_position: number
  created_at: string
  updated_at: string
  started_at?: string | null
  finished_at?: string | null
  assets: JobAsset[]
  events?: JobEvent[]
}

export interface UploadFile {
  id: string
  filename: string
  mime: string
  size: number
  kind: string
  created_at: string
}

export interface EndpointHealth {
  name: string
  url: string
  healthy: boolean
  latency_ms: number
  detail: string
  text_encoder?: string
  text_encoder_label?: string
}

export interface GPUStat {
  name: string
  util: number
  mem_used_mb: number
  mem_total_mb: number
  temp: number
}

export interface Hardware {
  cpu_percent: number
  cpu_cores: number
  ram_used_gb: number
  ram_total_gb: number
  ram_percent: number
  gpu_name: string
  gpu_percent: number
  gpu_mem_used_mb: number
  gpu_mem_total_mb: number
  gpu_temp: number
  gpus: GPUStat[]
}

export interface SystemStatus {
  app: string
  version: string
  inference_mode: string
  queue_depth: number
  running: number
  succeeded_today: number
  failed_today: number
  total_jobs: number
  worker_slots: number
  endpoints: EndpointHealth[]
  hardware: Hardware
  time: string
  auto_switch_engine?: boolean
  active_engine?: string
  max_loaded_engines?: number
}

export interface SettingsPayload {
  inference_mode: string
  sglang_fl2va_url: string
  sglang_ref2va_url: string
  media_file_prefix: string
  uri_mode: string
  worker_concurrency: number
  public_base_url: string
  minimax_api_base: string
  minimax_api_token: string
  has_minimax_token: boolean
  fasth3_url: string
  h3_turbo_url: string
  h3_pinkcherry_url: string
  h3_director_url?: string
  llada_image_url: string
  qwen_image_url?: string
  hunyuan_video_url?: string
  ltx23_url?: string
  chat_url?: string
  chat_model?: string
  chat_api_token?: string
  has_chat_token?: boolean
  chat_models?: string[]
  tts_url?: string
  tts_model?: string
  tts_voice?: string
  tts_api_token?: string
  has_tts_token?: boolean
  auto_switch_engine?: boolean
  max_loaded_engines?: number
}

export interface User {
  id: string
  username: string
  role?: 'admin' | 'user' | string
  created_at: string
  job_count?: number
}

export interface UserList {
  users: User[]
  allow_register: boolean
}

export interface AuthStatus {
  has_users: boolean
  allow_register: boolean
}

let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, { ...init, credentials: 'include' })
  if (res.status === 401 && !/\/api\/v1\/auth\/(login|register|status|logout|me|password)(\?|$)/.test(url)) {
    onUnauthorized?.()
  }
  if (!res.ok) {
    let msg = res.statusText
    try {
      const body = await res.json()
      msg = body.error || msg
    } catch {}
    throw new Error(msg)
  }
  return res.json()
}

export const api = {
  authStatus: () => request<AuthStatus>('/api/v1/auth/status'),
  me: () => request<User>('/api/v1/auth/me'),
  login: (username: string, password: string) =>
    request<User>('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    }),
  register: (username: string, password: string) =>
    request<User>('/api/v1/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ ok: boolean }>('/api/v1/auth/logout', { method: 'POST' }),
  changePassword: (old_password: string, new_password: string) =>
    request<{ ok: boolean }>('/api/v1/auth/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ old_password, new_password }),
    }),
  users: () => request<UserList>('/api/v1/users'),
  createUser: (body: { username: string; password: string; role?: string }) =>
    request<User>('/api/v1/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),
  patchUser: (id: string, body: { password?: string; role?: string }) =>
    request<User>(`/api/v1/users/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),
  deleteUser: (id: string) => request<{ ok: boolean }>(`/api/v1/users/${id}`, { method: 'DELETE' }),
  setRegisterPolicy: (allow_register: boolean) =>
    request<{ allow_register: boolean }>('/api/v1/auth/register-policy', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ allow_register }),
    }),
  system: () => request<SystemStatus>('/api/v1/system'),
  metrics: () => request<Hardware>('/api/v1/metrics'),
  settings: () => request<SettingsPayload>('/api/v1/settings'),
  saveSettings: (body: Partial<SettingsPayload>) =>
    request<SettingsPayload>('/api/v1/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),
  jobs: (q = '') => request<Job[]>(`/api/v1/jobs${q}`),
  job: (id: string) => request<Job>(`/api/v1/jobs/${id}`),
  createJob: (body: unknown) =>
    request<Job>('/api/v1/jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),
  cancelJob: (id: string) => request(`/api/v1/jobs/${id}/cancel`, { method: 'POST' }),
  retryJob: (id: string) => request<Job>(`/api/v1/jobs/${id}/retry`, { method: 'POST' }),
  bumpJob: (id: string) => request<Job>(`/api/v1/jobs/${id}/bump`, { method: 'POST' }),
  deleteJob: (id: string) => request(`/api/v1/jobs/${id}`, { method: 'DELETE' }),
  upload: async (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return request<UploadFile>('/api/v1/uploads', { method: 'POST', body: fd })
  },
  dramaProjects: (q = '') => request<DramaProject[]>(`/api/v1/drama-projects${q}`),
  dramaProject: (id: string) => request<DramaProject>(`/api/v1/drama-projects/${id}`),
  createDramaProject: (body: { title?: string; idea: string; style?: string; style_notes?: string; target_sec?: number }) =>
    request<DramaProject>('/api/v1/drama-projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),
  patchDramaProject: (id: string, body: Record<string, unknown>) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),
  deleteDramaProject: (id: string) => request<{ ok: boolean }>(`/api/v1/drama-projects/${id}`, { method: 'DELETE' }),
  dramaStoryboard: (id: string, body?: { count?: number; replace?: boolean; mode?: 'empty' | 'llm' }) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/storyboard`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    }),
  dramaWrite: (id: string) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/write`, { method: 'POST' }),
  dramaRewrite: (id: string, index: number, target: 'image_prompt' | 'video_prompt' | 'continue') =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/shots/${index}/rewrite`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target }),
    }),
  chatModels: () => request<{ url: string; model?: string; models: string[]; error?: string }>('/api/v1/chat/models'),
  dramaImages: (id: string, body?: { indexes?: number[]; queue?: boolean }) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/images`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    }),
  retryDramaImage: (id: string, index: number) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/images/${index}/retry`, { method: 'POST' }),
  dramaVideos: (id: string, body?: { indexes?: number[]; queue?: boolean }) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/videos`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    }),
  retryDramaVideo: (id: string, index: number) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/videos/${index}/retry`, { method: 'POST' }),
  dramaCompile: (id: string, body?: { indexes?: number[]; burn_subtitles?: boolean; mix_tts?: boolean }) =>
    request<DramaProject>(`/api/v1/drama-projects/${id}/compile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    }),
}

export const modeLabel: Record<JobMode, string> = {
  t2va: '文生影像',
  i2va: '首帧续写',
  l2va: '尾帧倒叙',
  fl2va: '首尾桥接',
  ref2va: '参考生成',
  t2i: '文生图',
  i2i: '指令编辑',
}

export const engineLabel: Record<JobEngine, string> = {
  h3: 'H3-Base',
  fasth3: 'FastH3',
  'h3-max': 'FastH3',
  'h3-turbo': 'H3 Turbo LoRA',
  'h3-ref2va-int8': 'H3 Ref2VA INT8',
  'h3-pinkcherry-int8': 'H3 PinkCherry INT8',
  'h3-director': 'H3 Timeline Director',
  'llada-image': 'LLaDA-Image',
  'qwen-image': 'Qwen-Image-2.1',
  'hunyuan-video': 'HunyuanVideo-1.5',
  'ltx-2.3': 'LTX-2.3',
}

export function engineName(engine?: string) {
  if (engine === 'fasth3' || engine === 'h3-max') return engineLabel.fasth3
  if (engine === 'h3-turbo') return engineLabel['h3-turbo']
  if (engine === 'h3-ref2va-int8') return engineLabel['h3-ref2va-int8']
  if (engine === 'h3-pinkcherry-int8') return engineLabel['h3-pinkcherry-int8']
  if (engine === 'h3-director') return engineLabel['h3-director']
  if (engine === 'llada-image') return engineLabel['llada-image']
  if (engine === 'qwen-image') return engineLabel['qwen-image']
  if (engine === 'hunyuan-video') return engineLabel['hunyuan-video']
  if (engine === 'ltx-2.3') return engineLabel['ltx-2.3']
  return engineLabel.h3
}

export function isImageEngine(engine?: string) {
  return engine === 'llada-image' || engine === 'qwen-image'
}

export function fallbackTextEncoder(engine?: string) {
  if (engine === 'llada-image') return 'LLaDA-Image 6B'
  if (engine === 'qwen-image') return 'Qwen-Image-2.1'
  if (engine === 'hunyuan-video') return 'HunyuanVideo-1.5 Qwen2.5-VL'
  if (engine === 'ltx-2.3') return 'LTX-2.3 Gemma 3'
  if (engine === 'h3-pinkcherry-int8') return 'PinkCherry Qwen3-VL 32B'
  if (engine === 'fasth3' || engine === 'h3-max' || engine === 'h3-ref2va-int8' || engine === 'h3-director') return 'Qwen3-VL 32B 量化'
  return 'Qwen3-VL 32B NF4'
}

export function textUnderstanding(job: {
  engine?: string
  text_encoder?: string
  text_encoder_label?: string
  prompt_rewriter?: string
  enhance_prompt?: boolean
}) {
  const label = job.text_encoder_label || fallbackTextEncoder(job.engine)
  const file = job.text_encoder && job.text_encoder !== label ? job.text_encoder : ''
  const rewrite = job.prompt_rewriter || (job.enhance_prompt && !isImageEngine(job.engine) ? 'H3-Context-IR' : '')
  let s = label
  if (file) s += ` · ${file}`
  if (rewrite) s += ` · 改写 ${rewrite}`
  return s
}

export function textUnderstandingShort(job: { engine?: string; text_encoder_label?: string }) {
  return job.text_encoder_label || fallbackTextEncoder(job.engine)
}

export function isImageJob(job: { engine?: string; mode?: string }) {
  return isImageEngine(job.engine) || job.mode === 't2i' || job.mode === 'i2i'
}

export type DramaStep = 'write' | 'storyboard' | 'image' | 'video' | 'compile'
export type DramaStatus = 'draft' | 'writing' | 'storyboard' | 'imaging' | 'videoing' | 'compiling' | 'done' | 'failed'

export interface DramaShotBeat {
  index: number
  time_range: string
  action: string
  image_prompt?: string
}

export interface DramaImageRef {
  upload_id: string
  name?: string
  kind?: 'character' | 'scene' | ''
  url?: string
}

export interface DramaShot {
  index: number
  title: string
  scene: string
  dialogue?: string
  image_prompt: string
  video_prompt: string
  duration: number
  image_job_id?: string
  extra_image_job_ids?: string[]
  video_job_id?: string
  image_upload_id?: string
  video_upload_id?: string
  last_frame_upload_id?: string
  skip_prev_images?: boolean
  queue_image?: boolean
  queue_video?: boolean
  continue_from_prev?: boolean
  image_refs?: DramaImageRef[]
  beats?: DramaShotBeat[]
  image_job?: Job
  extra_image_jobs?: Job[]
  video_job?: Job
  image_url?: string
  image_urls?: string[]
  video_url?: string
  last_frame_url?: string
  image_ref_views?: DramaImageRef[]
}

export interface DramaProject {
  id: string
  user_id: string
  title: string
  idea: string
  style: string
  style_notes: string
  target_sec: number
  step: DramaStep
  status: DramaStatus
  script_text: string
  shots_json: string
  write_model?: string
  write_result?: string
  storyboard_model?: string
  storyboard_result?: string
  image_engine: JobEngine
  image_aspect: string
  image_short_edge: number
  image_quality: string
  video_engine: JobEngine
  continue_engine: JobEngine
  video_aspect: string
  video_short_edge: number
  video_duration: number
  image_refs?: DramaImageRef[]
  burn_subtitles?: boolean
  mix_tts?: boolean
  compile_job_id?: string
  compile_status?: string
  compile_result?: string
  compile_indexes_json?: string
  created_at: string
  updated_at: string
  shots?: DramaShot[]
  compile_indexes?: number[]
  compile_url?: string
}

export const statusLabel: Record<JobStatus, string> = {
  queued: '排队',
  running: '推理',
  succeeded: '完成',
  failed: '失败',
  cancelled: '取消',
}

export const resolutionPresets = [
  { short: 480, label: '480p' },
  { short: 720, label: '720p' },
  { short: 1080, label: '1080p' },
] as const

export function resolutionLabel(shortEdge: number): string {
  if (shortEdge >= 1280) return `${shortEdge}`
  if (shortEdge >= 1080) return '1080p'
  if (shortEdge >= 1024) return '1024'
  if (shortEdge >= 768) return '768p'
  if (shortEdge >= 704) return '720p'
  return '480p'
}

function round32(n: number): number {
  return Math.max(32, Math.floor(n / 32) * 32)
}

export function canvasSize(aspect: string, short: number): { width: number; height: number } {
  const s = round32(short || 480)
  const table: Record<string, [number, number]> = {
    '16:9': [round32((s * 16) / 9), s],
    '9:16': [s, round32((s * 16) / 9)],
    '1:1': [s, s],
    '4:3': [round32((s * 4) / 3), s],
    '3:4': [s, round32((s * 4) / 3)],
    '3:2': [round32((s * 3) / 2), s],
    '2:3': [s, round32((s * 3) / 2)],
    '21:9': [round32((s * 21) / 9), s],
  }
  const key = aspect === 'auto' ? '16:9' : aspect
  const [w, h] = table[key] || table['16:9']
  return { width: w, height: h }
}
