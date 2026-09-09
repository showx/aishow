<template>
  <Modal :open="!!job" :title="title" kicker="Danger" :busy="busy" @close="emit('close')">
    <p class="lead warn">{{ hint }}</p>
    <p v-if="error" class="error">{{ error }}</p>
    <template #footer>
      <button class="btn btn-ghost" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button class="btn btn-danger" type="button" :disabled="busy" @click="emit('confirm')">{{ busy ? '删除中…' : '确认删除' }}</button>
    </template>
  </Modal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Modal from './Modal.vue'
import type { Job } from '../api/http'

const props = defineProps<{
  job: Job | null
  busy?: boolean
  error?: string
}>()

const emit = defineEmits<{ close: []; confirm: [] }>()

const title = computed(() => props.job ? `删除「${props.job.title}」` : '删除任务')
const hint = computed(() => {
  const status = props.job?.status
  if (status === 'running') return '正在推理的任务会被中止，记录和成品一并删除，不可恢复。'
  if (status === 'queued') return '该任务将从队列移除，记录不会保留。'
  if (status === 'succeeded') return '成片 / 图片和任务记录会一并删除，此操作不可恢复。'
  return '将删除该任务记录，此操作不可恢复。'
})
</script>

<style scoped>
.lead { color: var(--muted); line-height: 1.65; margin: 0; }
.lead.warn { color: var(--rose); }
.error { color: var(--rose); font-size: 13px; margin: 10px 0 0; }
</style>
