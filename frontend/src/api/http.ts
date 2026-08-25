export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
export type JobMode = 't2va' | 'i2va' | 'l2va' | 'fl2va' | 'ref2va'

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
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
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
  if (shortEdge >= 1008) return '1080p'
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
