<template>
  <label class="drop" :class="{ over, has: !!preview }" @dragover.prevent="over = true" @dragleave="over = false" @drop.prevent="onDrop">
    <input type="file" :accept="accept" hidden @change="onPick" />
    <img v-if="preview && kind === 'image'" :src="preview" alt="" />
    <video v-else-if="preview && kind === 'video'" :src="preview" muted />
    <div v-else class="empty">
      <div class="plus">＋</div>
      <div class="t">{{ title }}</div>
      <div class="h">拖入或点击选择</div>
    </div>
    <button v-if="modelValue" type="button" class="clear" @click.stop.prevent="$emit('update:modelValue', null)">清除</button>
  </label>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = defineProps<{
  title: string
  accept: string
  kind?: 'image' | 'video' | 'audio' | 'file'
  modelValue: File | null
}>()
const emit = defineEmits<{ 'update:modelValue': [File | null] }>()
const over = ref(false)
const preview = ref('')

const kind = computed(() => props.kind || 'file')

watch(() => props.modelValue, (file) => {
  if (preview.value) URL.revokeObjectURL(preview.value)
  preview.value = file ? URL.createObjectURL(file) : ''
})

function take(file?: File) {
  over.value = false
  if (file) emit('update:modelValue', file)
}

function onDrop(e: DragEvent) {
  take(e.dataTransfer?.files?.[0])
}
function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  take(input.files?.[0])
}
</script>

<style scoped>
.drop {
  position: relative;
  min-height: 148px;
  border: 1px dashed var(--line-strong);
  border-radius: 16px;
  overflow: hidden;
  cursor: pointer;
  background: rgba(0,0,0,0.22);
  display: grid;
  place-items: center;
}
.drop.over { border-color: var(--mint); background: var(--mint-dim); }
.drop img, .drop video { width: 100%; height: 148px; object-fit: cover; }
.empty { text-align: center; color: var(--muted); padding: 16px; }
.plus { font-size: 22px; color: var(--mint); }
.t { margin-top: 6px; color: var(--text); }
.h { font-size: 12px; margin-top: 4px; }
.clear {
  position: absolute; top: 8px; right: 8px;
  border: 0; border-radius: 8px; background: rgba(0,0,0,0.55); color: #fff; padding: 4px 8px; cursor: pointer;
}
</style>
