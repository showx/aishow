<template>
  <div class="page">
    <section class="panel form" v-if="form">
      <div class="intro">
        <h2>对接 MiniMax-H3</h2>
        <p>本控制面不在本机加载权重，而是调度你已经拉起的 H3-Base、本地 FastH3（FastVideo 4-step）或 LLaDA-Image。FL2VA 负责本地 t2va / 首尾帧，Ref2VA 负责参考生成。</p>
      </div>

      <div class="grid">
        <div class="field">
          <label>推理模式</label>
          <select v-model="form.inference_mode" class="select">
            <option value="auto">auto · 调用 SGLang</option>
            <option value="mock">mock · 无 GPU 演示队列</option>
          </select>
        </div>
        <div class="field">
          <label>工位并发</label>
          <input v-model.number="form.worker_concurrency" class="input" type="number" min="1" />
        </div>
        <div class="field">
          <label>FL2VA 地址</label>
          <input v-model="form.sglang_fl2va_url" class="input" placeholder="http://127.0.0.1:30010" />
        </div>
        <div class="field">
          <label>Ref2VA 地址</label>
          <input v-model="form.sglang_ref2va_url" class="input" placeholder="http://127.0.0.1:30011" />
        </div>
        <div class="field">
          <label>FastH3 地址</label>
          <input v-model="form.fasth3_url" class="input" placeholder="http://127.0.0.1:8000" />
        </div>
        <div class="field">
          <label>素材 URI 模式</label>
          <select v-model="form.uri_mode" class="select">
            <option value="file">file:// 映射到推理容器</option>
            <option value="http">http 回源本控制面</option>
          </select>
        </div>
        <div class="field">
          <label>file:// 前缀</label>
          <input v-model="form.media_file_prefix" class="input" />
        </div>
        <div class="field">
          <label>Public Base URL</label>
          <input v-model="form.public_base_url" class="input" />
        </div>
        <div class="field">
          <label>MiniMax API Base</label>
          <input v-model="form.minimax_api_base" class="input" />
        </div>
        <div class="field span">
          <label>MiniMax Token（可选，H3-Context-IR）</label>
          <input v-model="form.minimax_api_token" class="input" :placeholder="form.has_minimax_token ? '已保存，留空不改' : 'Bearer token'" />
        </div>
        <div class="field span">
          <label>LLaDA-Image 地址</label>
          <input v-model="form.llada_image_url" class="input" placeholder="http://127.0.0.1:30020" />
        </div>
      </div>

      <div class="actions">
        <button class="btn btn-primary" @click="save">保存节点配置</button>
        <span class="muted">修改工位并发后需重启后端才会生效。</span>
      </div>
    </section>

    <section class="panel docs">
      <h2>本机 LLaDA-Image（开源生图）</h2>
      <p>工坊选择「LLaDA-Image」后，任务走本机边车 <code>/v1/images</code>。支持文生图与指令编辑。默认 Turbo（4 步）；高品质 Base 把启动脚本里的 <code>LLADA_MODEL</code> 改成 <code>inclusionAI/LLaDA-Image</code>。推理模式需设成 auto。</p>
      <pre class="mono">git clone https://github.com/inclusionAI/LLaDA-Image.git
conda create -n llada-image python=3.11 -y
conda activate llada-image
pip install -r requirements.txt
D:\code\aishow\scripts\start_llada_image.bat</pre>
    </section>

    <section class="panel docs">
      <h2>本机 FastH3（FastVideo 4-step）</h2>
      <p>工坊选择「FastH3 本地」后，任务打到本机 <code>/v1/videos</code>，模型别名 <code>fasth3</code>。权重是 <a href="https://huggingface.co/FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2" target="_blank" rel="noreferrer">FastVideo-Minimax-FastH3-Preview-v0.2</a>，采样梯子 <code>[999, 749, 500, 250]</code>。当前 Preview 只蒸馏了文生，没有首尾帧 / 参考生成。</p>
      <pre class="mono">git clone https://github.com/hao-ai-lab/FastVideo.git
cd FastVideo
UV_TORCH_BACKEND=cu126 uv pip install -e ".[fasth3]"
hf download FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2 --local-dir E:\MiniMax-H3\models\FastVideo-Minimax-FastH3-Preview-v0.2
D:\code\aishow\scripts\start_fasth3.bat</pre>
      <p>官方 4 卡路径：<code>fastvideo serve --config D:\code\aishow\scripts\openai_fasth3.yaml</code>。本页 FastH3 地址填 <code>http://127.0.0.1:8000</code>，推理模式改成 auto。</p>
    </section>

    <section class="panel docs">
      <h2>本机 NF4（DiffSynth）</h2>
      <p>你已经下好的是 DiffSynth-Studio 的 NF4 量化包，不是 SGLang 权重。RTX 3090 Ti 24GB 可以跑 FL2VA（文生 / 首尾帧）。先启动推理边车，再把本页模式改成 auto。</p>
      <pre class="mono">D:\code\aishow\scripts\start_h3_nf4.bat</pre>
      <p>权重目录：<code>E:\MiniMax-H3\models\MiniMax-H3-NF4</code>。FL2VA 四件套已齐；<code>minimax-h3-ref2va-nf4.safetensors</code> 还是 incomplete，参考生成暂时不能用。下完那个文件后才能开 Ref2VA。</p>
    </section>

    <section class="panel docs">
      <h2>拉起官方 H3-Base（SGLang，多卡）</h2>
      <pre class="mono">sglang serve \
  --model-path MiniMaxAI/MiniMax-H3 \
  --num-gpus 4 \
  --ulysses-degree 4 \
  --performance-mode speed \
  --host 0.0.0.0 \
  --port 30010 \
  --model-variant fl2va

sglang serve \
  --model-path MiniMaxAI/MiniMax-H3 \
  --num-gpus 4 \
  --ulysses-degree 4 \
  --performance-mode speed \
  --host 0.0.0.0 \
  --port 30011 \
  --model-variant ref2va</pre>
      <p>将本仓库 `backend/data/media` 挂到推理容器的 `/data/minimax-h3`，即可让 `file://` 条件素材被 SGLang 读到。</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type SettingsPayload } from '../api/http'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const form = ref<SettingsPayload | null>(null)

onMounted(async () => {
  form.value = await api.settings()
})

async function save() {
  if (!form.value) return
  form.value = await api.saveSettings(form.value)
  store.settings = form.value
  store.flash('节点配置已保存')
  store.system = await api.system()
}
</script>

<style scoped>
.page { display: grid; gap: 18px; }
.form, .docs { padding: 22px; }
.intro p, .docs p, .muted { color: var(--muted); line-height: 1.7; margin: 8px 0 16px; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.span { grid-column: 1 / -1; }
.actions { display: flex; gap: 12px; align-items: center; margin-top: 18px; }
pre {
  background: rgba(0,0,0,0.35);
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 16px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.65;
  color: #c5f6ec;
}
@media (max-width: 900px) { .grid { grid-template-columns: 1fr; } }
</style>
