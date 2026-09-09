<template>
  <div v-if="multiple" class="multi" :class="{ over }" @dragover.prevent="over = true" @dragleave="over = false" @drop.prevent="onDropMany">
    <div v-for="(item, i) in items" :key="item.key" class="tile">
      <img v-if="kind === 'image' && item.url" :src="item.url" alt="" />
      <video v-else-if="kind === 'video' && item.url" :src="item.url" muted />
      <div v-else class="empty file">{{ item.file.name }}</div>
      <span class="idx">{{ i + 1 }}</span>
      <button type="button" class="clear" @click.stop.prevent="removeAt(i)">清除</button>
    </div>
    <label v-if="canAdd" class="drop add">
      <input type="file" :accept="accept" multiple hidden @change="onPickMany" />
      <div class="empty">
        <div class="plus">＋</div>
        <div class="t">{{ title }}</div>
        <div class="h">{{ addHint }}</div>
      </div>
    </label>
  </div>
  <label v-else class="drop" :class="{ over, has: !!preview }" @dragover.prevent="over = true" @dragleave="over = false" @drop.prevent="onDrop">
    <input type="file" :accept="accept" hidden @change="onPick" />
    <img v-if="preview && kind === 'image'" :src="preview" alt="" />
    <video v-else-if="preview && kind === 'video'" :src="preview" muted />
    <div v-else class="empty">
      <div class="plus">＋</div>
      <div class="t">{{ title }}</div>
      <div class="h">拖入或点击选择</div>
    </div>
    <button v-if="singleFile" type="button" class="clear" @click.stop.prevent="$emit('update:modelValue', null)">清除</button>
  </label>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  title: string
  accept: string
  kind?: 'image' | 'video' | 'audio' | 'file'
  modelValue: File | File[] | null
  multiple?: boolean
  max?: number
}>(), {
  kind: 'file',
  multiple: false,
  max: 9,
})
const emit = defineEmits<{ 'update:modelValue': [File | File[] | null] }>()
const over = ref(false)
const preview = ref('')
const urls = ref<string[]>([])

const kind = computed(() => props.kind || 'file')
const maxCount = computed(() => Math.max(1, props.max || 9))
const files = computed(() => Array.isArray(props.modelValue) ? props.modelValue : [])
const singleFile = computed(() => props.modelValue instanceof File ? props.modelValue : null)
const canAdd = computed(() => files.value.length < maxCount.value)
const addHint = computed(() => {
  const n = files.value.length
  return n ? `再加一张 · ${n}/${maxCount.value}` : `拖入或点击，最多 ${maxCount.value} 张`
})
const items = computed(() => files.value.map((file, i) => ({
  file,
  key: `${file.name}-${file.size}-${file.lastModified}-${i}`,
  url: urls.value[i] || '',
})))

watch(() => props.modelValue, (file) => {
  if (props.multiple) return
  if (preview.value) URL.revokeObjectURL(preview.value)
  preview.value = file instanceof File ? URL.createObjectURL(file) : ''
})

watch(files, (next) => {
  if (!props.multiple) return
  for (const url of urls.value) URL.revokeObjectURL(url)
  urls.value = next.map(file => URL.createObjectURL(file))
}, { immediate: true })

onBeforeUnmount(() => {
  if (preview.value) URL.revokeObjectURL(preview.value)
  for (const url of urls.value) URL.revokeObjectURL(url)
})

function matches(file: File) {
  if (kind.value === 'image') return file.type.startsWith('image/')
  if (kind.value === 'video') return file.type.startsWith('video/')
  if (kind.value === 'audio') return file.type.startsWith('audio/')
  return true
}

function sameFile(a: File, b: File) {
  return a.name === b.name && a.size === b.size && a.lastModified === b.lastModified
}

function take(file?: File) {
  over.value = false
  if (file && matches(file)) emit('update:modelValue', file)
}

function merge(incoming: File[]) {
  over.value = false
  const picked = incoming.filter(matches)
  if (!picked.length) return
  const next = [...files.value]
  for (const file of picked) {
    if (next.length >= maxCount.value) break
    if (next.some(existing => sameFile(existing, file))) continue
    next.push(file)
  }
  emit('update:modelValue', next)
}

function removeAt(index: number) {
  emit('update:modelValue', files.value.filter((_, i) => i !== index))
}

function onDrop(e: DragEvent) {
  take(e.dataTransfer?.files?.[0])
}

function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  take(input.files?.[0])
  input.value = ''
}

function onDropMany(e: DragEvent) {
  merge(Array.from(e.dataTransfer?.files || []))
}

function onPickMany(e: Event) {
  const input = e.target as HTMLInputElement
  merge(Array.from(input.files || []))
  input.value = ''
}
</script>

<style scoped>
.multi {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(128px, 1fr));
  gap: 10px;
  min-height: 148px;
}
.multi.over { outline: 1px dashed var(--mint); outline-offset: 4px; border-radius: 16px; }
.tile, .drop {
  position: relative;
  min-height: 148px;
  border: 1px dashed var(--line-strong);
  border-radius: 16px;
  overflow: hidden;
  background: rgba(0,0,0,0.22);
  display: grid;
  place-items: center;
}
.drop { cursor: pointer; }
.drop.over, .drop.add:hover { border-color: var(--mint); background: var(--mint-dim); }
.drop.has { border-style: solid; }
.tile { border-style: solid; }
.tile img, .tile video, .drop img, .drop video { width: 100%; height: 148px; object-fit: cover; }
.empty { text-align: center; color: var(--muted); padding: 16px; }
.empty.file { font-size: 12px; word-break: break-all; }
.plus { font-size: 22px; color: var(--mint); }
.t { margin-top: 6px; color: var(--text); }
.h { font-size: 12px; margin-top: 4px; }
.idx {
  position: absolute; top: 8px; left: 8px;
  min-width: 22px; height: 22px; padding: 0 6px;
  border-radius: 8px; background: rgba(0,0,0,0.55); color: #fff;
  font-size: 11px; line-height: 22px; text-align: center;
}
.clear {
  position: absolute; top: 8px; right: 8px;
  border: 0; border-radius: 8px; background: rgba(0,0,0,0.55); color: #fff; padding: 4px 8px; cursor: pointer;
}
</style>
