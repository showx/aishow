import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, type Job, type JobAsset, type JobEngine } from '../api/http'
import { useAppStore } from '../stores/app'

export function asEngine(engine?: string, mode?: string): JobEngine {
  if (engine === 'fasth3' || engine === 'h3-max') return 'fasth3'
  if (engine === 'h3-turbo') return 'h3-turbo'
  if (engine === 'h3-ref2va-int8') return 'h3-ref2va-int8'
  if (engine === 'h3-pinkcherry-int8') return 'h3-pinkcherry-int8'
  if (engine === 'llada-image') return 'llada-image'
  if (mode === 'ref2va') return 'h3-ref2va-int8'
  return 'h3'
}

export function randomSeed() {
  return Math.floor(Date.now() % 100000)
}

export function conditionsFromAssets(assets?: JobAsset[]) {
  return (assets || [])
    .filter(a => a.upload_id)
    .map(a => ({
      upload_id: a.upload_id,
      type: a.type,
      role: a.role,
      frame_index: a.frame_index ?? undefined,
      start_seconds: a.start_seconds ?? undefined,
    }))
}

export function payloadFromJob(job: Job, opts?: { seed?: number; prompt?: string }) {
  return {
    engine: asEngine(job.engine, job.mode),
    mode: job.mode,
    prompt: (opts?.prompt ?? job.prompt).trim(),
    duration: job.duration,
    aspect_ratio: job.aspect_ratio,
    short_edge: job.short_edge,
    seed: opts?.seed ?? job.seed,
    steps: job.steps,
    flow_shift: job.flow_shift,
    audio_flow_shift: job.audio_flow_shift,
    quality: job.quality,
    enhance_prompt: job.enhance_prompt,
    priority: job.priority,
    conditions: conditionsFromAssets(job.assets),
  }
}

export function useJobReuse() {
  const router = useRouter()
  const store = useAppStore()
  const regenerating = ref<Record<string, boolean>>({})

  function isRegenerating(id: string) {
    return !!regenerating.value[id]
  }

  function reuseInStudio(jobId: string, opts?: { enhanced?: boolean }) {
    router.push({
      name: 'studio',
      query: {
        reuse: jobId,
        ...(opts?.enhanced ? { enhanced: '1' } : {}),
      },
    })
  }

  async function regenerate(job: Job) {
    if (regenerating.value[job.id]) return
    regenerating.value = { ...regenerating.value, [job.id]: true }
    try {
      const full = await api.job(job.id)
      const created = await api.createJob(payloadFromJob(full, { seed: randomSeed() }))
      store.upsertJob(created)
      store.flash('已按相同参数重新排队（已换新 Seed）')
      return created
    } catch (e: any) {
      store.flash(e.message || '再生成失败')
    } finally {
      const next = { ...regenerating.value }
      delete next[job.id]
      regenerating.value = next
    }
  }

  return { reuseInStudio, regenerate, isRegenerating }
}
