export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
export type JobMode = 't2va' | 'i2va' | 'l2va' | 'fl2va' | 'ref2va' | 't2i' | 'i2i'
export type JobEngine = 'h3' | 'fasth3' | 'h3-max' | 'llada-image'

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
  llada_image_url: string
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
  'llada-image': 'LLaDA-Image',
}

export function engineName(engine?: string) {
  if (engine === 'fasth3' || engine === 'h3-max') return engineLabel.fasth3
  if (engine === 'llada-image') return engineLabel['llada-image']
  return engineLabel.h3
}

export function isImageJob(job: { engine?: string; mode?: string }) {
  return job.engine === 'llada-image' || job.mode === 't2i' || job.mode === 'i2i'
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
    '21:9': [round32((s * 21) / 9), s],
  }
  const key = aspect === 'auto' ? '16:9' : aspect
  const [w, h] = table[key] || table['16:9']
  return { width: w, height: h }
}
