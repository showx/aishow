<template>
  <div class="page">
    <div class="toolbar">
      <div class="seg">
        <button :class="{ active: filter === '' }" @click="filter = ''">全部</button>
        <button v-for="m in modes" :key="m" :class="{ active: filter === m }" @click="filter = m">{{ modeLabel[m] }}</button>
      </div>
    </div>

    <div v-if="items.length === 0" class="panel empty">片库还是空的。去工坊生成第一条。</div>
    <div class="grid">
      <article v-for="job in items" :key="job.id" class="panel card" @click="active = job">
        <div class="frame">
          <img v-if="job.has_image" :src="`/api/v1/jobs/${job.id}/image`" alt="" />
          <video v-else-if="job.has_video" :src="`/api/v1/jobs/${job.id}/video`" muted></video>
          <div v-else class="poster">
            <span>模拟成品</span>
          </div>
          <div class="shade">
            <span class="tag">{{ engineName(job.engine) }} · {{ modeLabel[job.mode] }}</span>
            <span>{{ jobMeta(job) }}</span>
          </div>
        </div>
        <div class="body">
          <h3>{{ job.title }}</h3>
          <p>{{ job.prompt }}</p>
        </div>
      </article>
    </div>

    <div v-if="active" class="lightbox" @click.self="active = null">
      <div class="panel player">
        <img v-if="active.has_image" class="preview" :src="`/api/v1/jobs/${active.id}/image`" alt="" />
        <video v-else-if="active.has_video" :src="`/api/v1/jobs/${active.id}/video`" controls autoplay></video>
        <div v-else class="poster big">该任务在模拟模式下完成，没有真实成品。</div>
        <div class="info">
          <h2>{{ active.title }}</h2>
          <p>{{ active.enhanced_prompt || active.prompt }}</p>
          <div class="meta">{{ engineName(active.engine) }} · {{ jobMeta(active) }} · seed {{ active.seed }} · {{ active.steps }} steps · {{ active.quality }}</div>
          <a v-if="active.has_image" class="btn btn-primary" :href="`/api/v1/jobs/${active.id}/image`" download>下载图片</a>
          <a v-else-if="active.has_video" class="btn btn-primary" :href="`/api/v1/jobs/${active.id}/video`" download>下载成片</a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '../stores/app'
import { engineName, isImageJob, modeLabel, resolutionLabel, type Job, type JobMode } from '../api/http'

const store = useAppStore()
const filter = ref('')
const active = ref<Job | null>(null)
const modes: JobMode[] = ['t2va', 'i2va', 'l2va', 'fl2va', 'ref2va', 't2i', 'i2i']
const items = computed(() => store.gallery.filter(j => !filter.value || j.mode === filter.value))

function jobMeta(job: Job) {
  const size = `${resolutionLabel(job.short_edge)} · ${job.aspect_ratio}`
  if (isImageJob(job)) return size
  return `${job.duration}s · ${size}`
}
</script>

<style scoped>
.page { display: grid; gap: 18px; }
.grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; }
.card { overflow: hidden; cursor: pointer; }
.frame { position: relative; aspect-ratio: 16/10; background: #0b0d14; }
.frame video, .frame img, .poster { width: 100%; height: 100%; object-fit: cover; }
.preview { width: 100%; border-radius: 12px; background: #000; display: block; }
.poster {
  display: grid; place-items: center;
  background:
    radial-gradient(circle at 20% 20%, rgba(94,234,212,0.25), transparent 45%),
    radial-gradient(circle at 80% 80%, rgba(232,184,109,0.2), transparent 40%),
    #10131b;
  color: var(--muted);
}
.shade {
  position: absolute; inset: auto 0 0 0; padding: 10px 12px;
  display: flex; justify-content: space-between;
  background: linear-gradient(transparent, rgba(0,0,0,0.7));
  font-size: 12px;
}
.tag { color: var(--mint); }
.body { padding: 14px 16px 16px; }
.body h3 { font-size: 16px; margin-bottom: 6px; }
.body p { color: var(--muted); font-size: 13px; line-height: 1.6; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.empty { padding: 40px; color: var(--muted); text-align: center; }
.lightbox {
  position: fixed; inset: 0; z-index: 30;
  background: rgba(0,0,0,0.62);
  display: grid; place-items: center; padding: 24px;
}
.player { width: min(980px, 100%); padding: 16px; display: grid; gap: 12px; }
.player video { width: 100%; border-radius: 12px; background: #000; }
.info p { color: var(--muted); line-height: 1.7; margin: 8px 0 12px; white-space: pre-wrap; }
.meta { color: var(--faint); font-size: 12px; margin-bottom: 12px; }
.poster.big { min-height: 280px; border-radius: 12px; }
@media (max-width: 1100px) { .grid { grid-template-columns: 1fr; } }
</style>
