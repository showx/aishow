<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="open"
        class="overlay"
        @mousedown.self="tryClose"
      >
        <div
          ref="dialogEl"
          class="dialog panel"
          role="dialog"
          aria-modal="true"
          tabindex="-1"
          :style="{ maxWidth: width }"
          @keydown.esc.prevent="tryClose"
        >
          <header class="head">
            <div>
              <div v-if="kicker" class="kicker">{{ kicker }}</div>
              <h2>{{ title }}</h2>
            </div>
            <button class="close" type="button" aria-label="关闭" :disabled="busy" @click="tryClose">×</button>
          </header>
          <div class="body">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="foot">
            <slot name="footer" />
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { watch, onUnmounted, ref, nextTick } from 'vue'

const props = withDefaults(defineProps<{
  open: boolean
  title: string
  kicker?: string
  width?: string
  busy?: boolean
}>(), {
  kicker: '',
  width: '440px',
  busy: false,
})

const emit = defineEmits<{ close: [] }>()
const dialogEl = ref<HTMLElement | null>(null)

watch(() => props.open, async (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
  if (open) {
    await nextTick()
    dialogEl.value?.focus()
  }
})
onUnmounted(() => {
  document.body.style.overflow = ''
})

function tryClose() {
  if (props.busy) return
  emit('close')
}
</script>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(4, 6, 10, 0.72);
  backdrop-filter: blur(10px);
}
.dialog {
  width: 100%;
  padding: 0;
  overflow: hidden;
  outline: none;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 22px 22px 0;
}
.head h2 { font-size: 22px; margin-top: 4px; }
.close {
  width: 36px;
  height: 36px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--muted);
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
}
.close:hover { background: var(--bg-soft); color: var(--text); }
.close:disabled { opacity: 0.4; cursor: not-allowed; }
.body { padding: 18px 22px 8px; }
.foot {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 22px 22px;
}
.modal-enter-active, .modal-leave-active { transition: opacity 0.16s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .dialog, .modal-leave-active .dialog { transition: transform 0.16s ease; }
.modal-enter-from .dialog, .modal-leave-to .dialog { transform: translateY(10px) scale(0.98); }
</style>
