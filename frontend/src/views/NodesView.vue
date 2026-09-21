<template>
  <div class="page">
    <section class="panel form" v-if="form">
      <div class="intro">
        <h2>对接 MiniMax-H3</h2>
        <p>本控制面不在本机加载权重，而是调度 H3-Base NF4、H3 Turbo LoRA、H3 PinkCherry INT8、H3 Ref2VA INT8、H3 Timeline Director、本地 FastH3 或 LLaDA-Image。PinkCherry 是单独一套（权重目录 + ComfyUI 8189），不要和 FastH3 / Ref2VA / Director 的 8188 混用。打开「排队时自动切换模型」后，可以混着堆队列：工位会等当前引擎跑完，再停旧模型、启新模型。</p>
        <p>每条任务会记下文字理解用了哪个编码器（以及是否走了 H3-Context-IR 提示改写），片库参数和队列日志里都能看到，方便对照「提示词 vs 成片动作」。</p>
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
        <div class="field span">
          <label class="check">
            <input v-model="form.auto_switch_engine" type="checkbox" />
            排队时自动切换模型（离线引擎也能投）
          </label>
          <p class="hint">「离线」不是禁用。打开后可以混着堆队列：先跑完当前引擎，再结束旧边车、启动新边车。关掉则只有已在线的引擎能跑。</p>
        </div>
        <div class="field">
          <label>同时最多加载几个模型</label>
          <input v-model.number="form.max_loaded_engines" class="input" type="number" min="1" max="4" />
          <p class="hint">默认 1。24GB 单卡请保持 1：新模型起来后，后台会把多出来的边车关掉。</p>
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
          <label>H3 Turbo LoRA 地址</label>
          <input v-model="form.h3_turbo_url" class="input" placeholder="http://127.0.0.1:30012" />
        </div>
        <div class="field">
          <label>H3 PinkCherry INT8 地址</label>
          <input v-model="form.h3_pinkcherry_url" class="input" placeholder="http://127.0.0.1:30013" />
        </div>
        <div class="field">
          <label>H3 Timeline Director 地址</label>
          <input v-model="form.h3_director_url" class="input" placeholder="http://127.0.0.1:30014" />
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
        <div class="field">
          <label>本地 Chat 地址</label>
          <input v-model="form.chat_url" class="input" placeholder="http://127.0.0.1:11434" />
          <p class="hint">OpenAI 兼容对话接口，默认 Ollama。短剧写剧本 / 拆分镜 / 重写提示走这里。不是 H3 的 Qwen3-VL。</p>
        </div>
        <div class="field">
          <label>本地 Chat 模型</label>
          <input v-model="form.chat_model" class="input" list="chat-models" placeholder="qwen3.5:4b" />
          <datalist id="chat-models">
            <option v-for="name in chatModels" :key="name" :value="name" />
          </datalist>
          <p class="hint">短剧默认 qwen3.5:4b。不要选 qwen3-vl（视觉模型，写不了剧本）。mock 且连不上会退回模板文本。</p>
        </div>
        <div class="field span">
          <label>本地 Chat Token（可选）</label>
          <input v-model="form.chat_api_token" class="input" :placeholder="form.has_chat_token ? '已保存，留空不改' : 'Ollama 通常不需要'" />
        </div>
        <div class="field">
          <label>本地 TTS 地址（可选）</label>
          <input v-model="form.tts_url" class="input" placeholder="http://127.0.0.1:8880" />
          <p class="hint">OpenAI 兼容 /v1/audio/speech。短剧合成勾选「叠本地语音」时用。留空则只用 H3 音轨。</p>
        </div>
        <div class="field">
          <label>TTS 模型</label>
          <input v-model="form.tts_model" class="input" placeholder="可选" />
        </div>
        <div class="field">
          <label>TTS 音色</label>
          <input v-model="form.tts_voice" class="input" placeholder="alloy" />
        </div>
        <div class="field">
          <label>TTS Token（可选）</label>
          <input v-model="form.tts_api_token" class="input" :placeholder="form.has_tts_token ? '已保存，留空不改' : '通常不需要'" />
        </div>
      </div>

      <div class="actions">
        <button class="btn btn-primary" @click="save">保存节点配置</button>
        <button class="btn" type="button" @click="pullChatModels">拉取 Chat 模型</button>
        <span class="muted">修改工位并发后需重启后端才会生效。</span>
      </div>
      <p v-if="chatError" class="hint">{{ chatError }}</p>
    </section>

    <section class="panel docs">
      <h2>文字理解（Text Encoder）</h2>
      <p>生成时真正读提示词的是文本编码器，不是 DiT 本身。任务落库字段：<code>text_encoder_label</code>、<code>text_encoder</code>、<code>prompt_rewriter</code>。</p>
      <pre class="mono">H3-Base NF4     Qwen3-VL 32B NF4     minimax-h3-text-encoder-nf4.safetensors
H3 Turbo LoRA   Qwen3-VL 32B NF4     同上，再叠 lightx2v 4 步 LoRA
H3 PinkCherry   Qwen3-VL 32B 量化     %H3_PINKCHERRY_ROOT%\text_encoders（独立目录）
H3 Ref2VA INT8  Qwen3-VL 32B 量化     %FASTH3_GGUF_ROOT%\text_encoders
H3 Director     Qwen3-VL 32B 量化     同上，工作流走 TimelineDirector 二采
FastH3 GGUF     Qwen3-VL 32B 量化     同上
LLaDA-Image     LLaDA-Image 6B        模型自身语言骨干（Turbo / Base）
提示改写        H3-Context-IR         仅在勾选「增强提示」且非 LLaDA 时</pre>
    </section>

    <section class="panel docs">
      <h2>本地 TTS（短剧对白，可选）</h2>
      <p>合成时若勾选「叠本地语音」，会把各镜对白打到 OpenAI 兼容的 <code>/v1/audio/speech</code>，再混进成片。没配地址就跳过，仍用 H3 自己的音轨。不是 GPU 边车，不要和 H3 抢卡。</p>
    </section>

    <section class="panel docs">
      <h2>本地 Chat（短剧编剧）</h2>
      <p>短剧页的写剧本、拆分镜、重写提示走 OpenAI 兼容的 <code>/v1/chat/completions</code>，默认 <code>http://127.0.0.1:11434</code>（Ollama）。Token 可选。H3 边上的 Qwen3-VL 只做视频文本编码，不要填成这里的对话模型。</p>
      <pre class="mono">scripts\start_drama_chat.bat
# 默认用本机 Ollama 的 qwen3.5:4b（约 3.4GB）
# 推理节点：地址 http://127.0.0.1:11434 ，模型 qwen3.5:4b
# 不要选 qwen3-vl</pre>
    </section>

    <section class="panel docs">
      <h2>本机 LLaDA-Image（开源生图）</h2>
      <p>工坊选择「LLaDA-Image」后，任务走本机边车 <code>/v1/images</code>。支持文生图与指令编辑。默认 Turbo（4 步）；高品质 Base 把启动脚本里的 <code>LLADA_MODEL</code> 改成 <code>inclusionAI/LLaDA-Image</code>。推理模式需设成 auto。</p>
      <pre class="mono">git clone https://github.com/inclusionAI/LLaDA-Image.git
# 在 scripts\paths.bat 填写 LLADA_REPO、LLADA_MODEL
scripts\start_llada_image.bat</pre>
    </section>

    <section class="panel docs">
      <h2>本机 FastH3（GGUF Q4）</h2>
      <p>24GB 机器走 ComfyUI + Q4 GGUF，不要加载完整 FastVideo Preview。工坊选「FastH3 本地」后，任务打到 <code>http://127.0.0.1:8000/v1/videos</code>，边车再转给 ComfyUI <code>8188</code>。只支持文生，4 步 + VSA。</p>
      <pre class="mono">powershell -File scripts\download_fasth3_gguf.ps1
scripts\start_fasth3_gguf.bat</pre>
      <p>权重目录由 <code>scripts\paths.bat</code> 的 <code>FASTH3_GGUF_ROOT</code>（或 <code>MODELS_ROOT\fasth3-gguf</code>）决定。本页 FastH3 地址保持 <code>http://127.0.0.1:8000</code>，推理模式改成 auto。完整 bf16 FastVideo 边车是 <code>start_fasth3.bat</code>，显存和内存都远高于 GGUF 路径。</p>
    </section>

    <section class="panel docs">
      <h2>本机 H3 Turbo LoRA（DiffSynth NF4 + 4 步）</h2>
      <p>对应 <a href="https://huggingface.co/spaces/Pepe104/MiniMax-H3-Turbo-Lora-UNCENSORED" target="_blank" rel="noreferrer">Hugging Face Space</a> 的 Turbo LoRA，但<strong>不是</strong> Space 那套未量化 bf16（官方 Space 约 72GB 显存）。本仓库走 NF4 底模 + <code>lightx2v/Minimax-h3-Turbo</code>，4 步，磁盘卸载，24GB 单卡可跑。</p>
      <pre class="mono">powershell -File scripts\download_h3_turbo_lora.ps1
scripts\start_h3_turbo_lora.bat</pre>
      <p>边车 <code>http://127.0.0.1:30012</code>。工坊选「H3 Turbo LoRA」，24GB 先 480p / 5 秒 / 4 步。不要和 H3-Base NF4 同时加载。</p>
    </section>

    <section class="panel docs">
      <h2>本机 PinkCherry（独立 INT8 FL2VA）</h2>
      <p>和 FastH3 / Ref2VA <strong>不是同一套</strong>：权重在 <code>H3_PINKCHERRY_ROOT</code>，ComfyUI 走 <code>8189</code>，边车 <code>30013</code>。不要把 PinkCherry 文件放进 fasth3-gguf，也不要复用 8188。权重来自 <a href="https://huggingface.co/SexGod1979/PinkCherry_MiniMax-H3" target="_blank" rel="noreferrer">SexGod1979/PinkCherry_MiniMax-H3</a> 的 pruned INT8 beta-0.6。工坊选「H3 PinkCherry INT8」。只做文生 / 首尾帧。</p>
      <pre class="mono">powershell -File scripts\download_h3_pinkcherry_int8.ps1
scripts\start_h3_pinkcherry_int8.bat</pre>
      <p>本页 PinkCherry 地址保持 <code>http://127.0.0.1:30013</code>。24GB 先用 480p / 5 秒 / 20 步。切换走自动停旧边车，8188 和 8189 不会同时留着。</p>
    </section>

    <section class="panel docs">
      <h2>本机 NF4（DiffSynth）</h2>
      <p>DiffSynth-Studio 的 NF4 量化包，不是 SGLang 权重。24GB 单卡可以跑 FL2VA（文生 / 首尾帧）。先启动推理边车，再把本页模式改成 auto。</p>
      <pre class="mono">scripts\start_h3_nf4.bat</pre>
      <p>权重目录由 <code>H3_NF4_DIR</code>（默认 <code>%H3_ROOT%\models\MiniMax-H3-NF4</code>）决定。参考生成请走下面的 INT8 边车，不要和 NF4 边车抢同一张卡。</p>
    </section>

    <section class="panel docs">
      <h2>本机 Ref2VA（Comfy-Org pruned INT8）</h2>
      <p>工坊选「H3 Ref2VA INT8」后，任务打到 <code>http://127.0.0.1:30011/v1/videos</code>，边车再转给 ComfyUI <code>8188</code>。权重是 <code>minimax_h3_ref2va_pruned_int8_convrot.safetensors</code>。文生请用 FastH3；首尾帧请用 H3-Base NF4。不要和 PinkCherry 的 8189 混用。</p>
      <pre class="mono">scripts\start_h3_ref2va_int8.bat</pre>
      <p>本页 Ref2VA 地址保持 <code>http://127.0.0.1:30011</code>，推理模式 auto。24GB 先用 480p / 5 秒 / 20 步。</p>
    </section>

    <section class="panel docs">
      <h2>本机 Timeline Director（SelfLift 二采）</h2>
      <p>工坊选「H3 Timeline Director」后，任务打到 <code>http://127.0.0.1:30014/v1/videos</code>，边车提交 <a href="https://github.com/Songssx/ComfyUI-MiniMaxH3-TimelineDirector" target="_blank" rel="noreferrer">ComfyUI-MiniMaxH3-TimelineDirector</a> 的素材规划台 + 有限分段采样。有 H3 Latent Upscaler 时开 SelfLift 二采：大部分步数在半分辨率跑，再抬回高清。文生和参考生成都能走这套图。和 FastH3 / Ref2VA 共用 ComfyUI <code>8188</code>。</p>
      <pre class="mono">powershell -File scripts\download_h3_latent_upscaler.ps1
scripts\start_h3_director.bat</pre>
      <p>本页 Director 地址保持 <code>http://127.0.0.1:30014</code>。24GB 先用 480p / 5 秒 / 8 步，和原来的 Ref2VA 20 步对照耗时。超过 15 秒会自动分段续写，最长 30 秒。</p>
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
const chatModels = ref<string[]>([])
const chatError = ref('')

onMounted(async () => {
  const next = await api.settings()
  if (next.auto_switch_engine === undefined) next.auto_switch_engine = true
  if (!next.max_loaded_engines || next.max_loaded_engines < 1) next.max_loaded_engines = 1
  if (!next.h3_turbo_url) next.h3_turbo_url = 'http://127.0.0.1:30012'
  if (!next.h3_pinkcherry_url) next.h3_pinkcherry_url = 'http://127.0.0.1:30013'
  if (!next.h3_director_url) next.h3_director_url = 'http://127.0.0.1:30014'
  if (!next.chat_url) next.chat_url = 'http://127.0.0.1:11434'
  if (!next.chat_model) next.chat_model = 'qwen3.5:4b'
  form.value = next
})

async function save() {
  if (!form.value) return
  form.value = await api.saveSettings(form.value)
  store.settings = form.value
  store.flash('节点配置已保存')
  store.system = await api.system()
}

async function pullChatModels() {
  if (!form.value) return
  chatError.value = ''
  await save()
  const res = await api.chatModels()
  chatModels.value = res.models || []
  chatError.value = res.error || (chatModels.value.length ? '' : '没有拉到模型，确认 Ollama / 兼容服务已启动')
  if (!form.value.chat_model && chatModels.value.length) {
    form.value.chat_model = chatModels.value[0]
  }
}
</script>

<style scoped>
.page { display: grid; gap: 18px; }
.form, .docs { padding: 22px; }
.intro p, .docs p, .muted { color: var(--muted); line-height: 1.7; margin: 8px 0 16px; }
.hint { font-size: 12px; color: var(--faint); line-height: 1.5; margin: 6px 0 0; }
.check { display: flex; gap: 8px; align-items: center; font-size: 14px; }
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
