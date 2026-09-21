<template>
  <div class="refs" :class="{ compact }">
    <article v-for="(ref, i) in refs" :key="ref.upload_id" class="ref">
      <img :src="ref.url || `/api/v1/uploads/${ref.upload_id}/raw`" alt="" />
      <input v-model="ref.name" class="input" placeholder="名字" :disabled="disabled" @change="$emit('change')" />
      <select v-model="ref.kind" class="select" :disabled="disabled" @change="$emit('change')">
        <option value="character">人物</option>
        <option value="scene">场景</option>
        <option value="">未分类</option>
      </select>
      <button class="btn btn-danger" type="button" :disabled="disabled" @click="$emit('remove', i)">删除</button>
    </article>
    <label v-if="refs.length < max" class="btn add-ref" :class="{ disabled: disabled }">
      {{ addLabel }}
      <input type="file" accept="image/*" multiple hidden :disabled="disabled" @change="onPick" />
    </label>
  </div>
</template>

<script setup lang="ts">
import type { DramaImageRef } from '../api/http'

withDefaults(defineProps<{
  refs: DramaImageRef[]
  max?: number
  compact?: boolean
  disabled?: boolean
  addLabel?: string
}>(), {
  max: 16,
  compact: false,
  disabled: false,
  addLabel: '添加参考图',
})

const emit = defineEmits<{
  files: [files: File[]]
  remove: [index: number]
  change: []
}>()

function onPick(ev: Event) {
  const input = ev.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (files.length) emit('files', files)
}
</script>

<style scoped>
.refs { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 10px; }
.refs.compact { grid-template-columns: repeat(auto-fill, minmax(110px, 1fr)); }
.ref { display: grid; gap: 6px; border: 1px solid var(--line); border-radius: 12px; padding: 8px; background: rgba(0,0,0,0.2); }
.ref img { width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 8px; background: #000; }
.add-ref { display: grid; place-items: center; min-height: 140px; cursor: pointer; }
.compact .add-ref { min-height: 110px; font-size: 13px; }
.add-ref.disabled { opacity: 0.4; pointer-events: none; }
</style>
