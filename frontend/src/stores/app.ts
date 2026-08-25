import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, type Hardware, type Job, type SettingsPayload, type SystemStatus } from '../api/http'

export const useAppStore = defineStore('app', () => {
  const jobs = ref<Job[]>([])
  const system = ref<SystemStatus | null>(null)
  const hardware = ref<Hardware | null>(null)
  const settings = ref<SettingsPayload | null>(null)
  const connected = ref(false)
  const notice = ref('')

  const queued = computed(() => jobs.value.filter(j => j.status === 'queued'))
  const running = computed(() => jobs.value.filter(j => j.status === 'running'))
  const gallery = computed(() => jobs.value.filter(j => j.status === 'succeeded'))

  async function refresh() {
    const [j, s, st, hw] = await Promise.all([api.jobs('?limit=120'), api.system(), api.settings(), api.metrics()])
    jobs.value = j
    system.value = s
    settings.value = st
    hardware.value = hw
  }

  async function refreshMetrics() {
    hardware.value = await api.metrics()
  }

  function upsertJob(job: Job) {
    const i = jobs.value.findIndex(x => x.id === job.id)
    if (i >= 0) jobs.value[i] = { ...jobs.value[i], ...job }
    else jobs.value.unshift(job)
  }

  function connectEvents() {
    const es = new EventSource('/api/v1/events')
    es.onopen = () => { connected.value = true }
    es.onerror = () => { connected.value = false }
    es.onmessage = (e) => {
      try {
        const payload = JSON.parse(e.data)
        if (payload.type === 'job.created' || payload.type === 'job.updated') {
          upsertJob(payload.data)
        }
        if (payload.type === 'queue.changed') {
          api.system().then(s => { system.value = s })
        }
      } catch {}
    }
    return es
  }

  function flash(msg: string) {
    notice.value = msg
    setTimeout(() => { if (notice.value === msg) notice.value = '' }, 2800)
  }

  return { jobs, system, hardware, settings, connected, notice, queued, running, gallery, refresh, refreshMetrics, upsertJob, connectEvents, flash }
})
