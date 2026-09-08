<template>
  <div class="page">
    <section class="stats">
      <article class="panel stat">
        <div class="kicker">Accounts</div>
        <div class="num display">{{ users.length }}</div>
        <div class="hint">全部账号</div>
      </article>
      <article class="panel stat">
        <div class="kicker">Admins</div>
        <div class="num display">{{ adminCount }}</div>
        <div class="hint">管理员</div>
      </article>
      <article class="panel stat">
        <div class="kicker">Members</div>
        <div class="num display">{{ users.length - adminCount }}</div>
        <div class="hint">普通用户</div>
      </article>
      <article class="panel stat register">
        <div class="kicker">Signup</div>
        <div class="switch-row">
          <div>
            <div class="switch-title">{{ allowRegister ? '公开注册开启' : '公开注册关闭' }}</div>
            <div class="hint">关闭后只能由管理员开户</div>
          </div>
          <button class="switch" type="button" :class="{ on: allowRegister }" :aria-pressed="allowRegister" @click="toggleRegister">
            <i></i>
          </button>
        </div>
      </article>
    </section>

    <section class="panel table-card">
      <div class="toolbar">
        <div>
          <h2>用户</h2>
          <p class="hint">管理工坊登录账号、角色与密码。</p>
        </div>
        <div class="toolbar-actions">
          <label class="search">
            <span>搜索</span>
            <input v-model="query" class="input" placeholder="按用户名筛选" />
          </label>
          <button class="btn btn-primary" type="button" @click="openCreate">创建用户</button>
        </div>
      </div>

      <p v-if="banner" class="banner" :class="banner.type">{{ banner.text }}</p>

      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>账号</th>
              <th>角色</th>
              <th>任务</th>
              <th>创建时间</th>
              <th class="right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="filtered.length === 0">
              <td colspan="5" class="empty">{{ users.length === 0 ? '还没有账号。' : '没有匹配的用户。' }}</td>
            </tr>
            <tr v-for="u in filtered" :key="u.id">
              <td>
                <div class="who">
                  <span class="avatar" :class="u.role === 'admin' ? 'admin' : ''">{{ initial(u.username) }}</span>
                  <div>
                    <div class="name">
                      {{ u.username }}
                      <span v-if="u.id === auth.user?.id" class="me">当前登录</span>
                    </div>
                    <div class="id mono">{{ u.id.slice(0, 8) }}</div>
                  </div>
                </div>
              </td>
              <td>
                <span class="role" :class="u.role === 'admin' ? 'admin' : 'user'">
                  {{ u.role === 'admin' ? '管理员' : '普通用户' }}
                </span>
              </td>
              <td class="mono jobs">{{ u.job_count ?? 0 }}</td>
              <td class="muted">{{ formatTime(u.created_at) }}</td>
              <td class="right">
                <div class="ops">
                  <button class="text-btn" type="button" @click="openReset(u)">重置密码</button>
                  <button class="text-btn" type="button" @click="openRole(u)">调整角色</button>
                  <button
                    class="text-btn danger"
                    type="button"
                    :disabled="u.id === auth.user?.id"
                    @click="openDelete(u)"
                  >删除</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <Modal :open="dialog === 'create'" title="创建用户" kicker="New account" :busy="busy" @close="closeDialog">
      <form id="create-form" class="form" @submit.prevent="create">
        <div class="field">
          <label>用户名</label>
          <input v-model="form.username" class="input" autocomplete="off" autofocus />
        </div>
        <div class="field">
          <label>初始密码</label>
          <input v-model="form.password" class="input" type="password" autocomplete="new-password" />
        </div>
        <div class="field">
          <label>确认密码</label>
          <input v-model="form.confirm" class="input" type="password" autocomplete="new-password" />
        </div>
        <div class="field">
          <label>角色</label>
          <div class="seg">
            <button type="button" :class="{ active: form.role === 'user' }" @click="form.role = 'user'">普通用户</button>
            <button type="button" :class="{ active: form.role === 'admin' }" @click="form.role = 'admin'">管理员</button>
          </div>
        </div>
        <p v-if="dialogError" class="error">{{ dialogError }}</p>
      </form>
      <template #footer>
        <button class="btn btn-ghost" type="button" :disabled="busy" @click="closeDialog">取消</button>
        <button class="btn btn-primary" type="submit" form="create-form" :disabled="busy">{{ busy ? '创建中…' : '创建' }}</button>
      </template>
    </Modal>

    <Modal :open="dialog === 'reset'" :title="target ? `重置「${target.username}」的密码` : '重置密码'" kicker="Security" :busy="busy" @close="closeDialog">
      <form id="reset-form" class="form" @submit.prevent="confirmReset">
        <p class="lead">对方的其他登录会话会被立即踢出，下次需用新密码进入工坊。</p>
        <div class="field">
          <label>新密码</label>
          <input v-model="resetPass" class="input" type="password" autocomplete="new-password" />
        </div>
        <div class="field">
          <label>确认新密码</label>
          <input v-model="resetConfirm" class="input" type="password" autocomplete="new-password" />
        </div>
        <p v-if="dialogError" class="error">{{ dialogError }}</p>
      </form>
      <template #footer>
        <button class="btn btn-ghost" type="button" :disabled="busy" @click="closeDialog">取消</button>
        <button class="btn btn-primary" type="submit" form="reset-form" :disabled="busy">{{ busy ? '保存中…' : '确认重置' }}</button>
      </template>
    </Modal>

    <Modal :open="dialog === 'role'" :title="target ? `调整「${target.username}」的角色` : '调整角色'" kicker="Access" :busy="busy" @close="closeDialog">
      <div class="form">
        <p class="lead">管理员可以管理账号、开关公开注册；普通用户只能使用自己的工坊任务。</p>
        <div class="seg">
          <button type="button" :class="{ active: nextRole === 'user' }" @click="nextRole = 'user'">普通用户</button>
          <button type="button" :class="{ active: nextRole === 'admin' }" @click="nextRole = 'admin'">管理员</button>
        </div>
        <p v-if="dialogError" class="error">{{ dialogError }}</p>
      </div>
      <template #footer>
        <button class="btn btn-ghost" type="button" :disabled="busy" @click="closeDialog">取消</button>
        <button class="btn btn-primary" type="button" :disabled="busy" @click="confirmRole">{{ busy ? '保存中…' : '保存角色' }}</button>
      </template>
    </Modal>

    <Modal :open="dialog === 'delete'" :title="target ? `删除「${target.username}」` : '删除用户'" kicker="Danger" :busy="busy" @close="closeDialog">
      <div class="form">
        <p class="lead warn">此操作不可恢复。该账号下的任务、素材和成片会一并删除。</p>
        <p v-if="dialogError" class="error">{{ dialogError }}</p>
      </div>
      <template #footer>
        <button class="btn btn-ghost" type="button" :disabled="busy" @click="closeDialog">取消</button>
        <button class="btn btn-danger" type="button" :disabled="busy" @click="confirmDelete">{{ busy ? '删除中…' : '确认删除' }}</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import Modal from '../components/Modal.vue'
import { api, type User } from '../api/http'
import { useAppStore } from '../stores/app'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const store = useAppStore()
const users = ref<User[]>([])
const allowRegister = ref(false)
const query = ref('')
const busy = ref(false)
const dialog = ref<'create' | 'reset' | 'role' | 'delete' | ''>('')
const dialogError = ref('')
const banner = ref<{ type: 'error' | 'ok'; text: string } | null>(null)
const target = ref<User | null>(null)
const resetPass = ref('')
const resetConfirm = ref('')
const nextRole = ref('user')
const form = reactive({ username: '', password: '', confirm: '', role: 'user' })

const adminCount = computed(() => users.value.filter(u => u.role === 'admin').length)
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter(u => u.username.toLowerCase().includes(q))
})

async function load() {
  const data = await api.users()
  users.value = data.users
  allowRegister.value = data.allow_register
  auth.status = { has_users: data.users.length > 0, allow_register: data.allow_register }
}

onMounted(() => {
  load().catch(err => { banner.value = { type: 'error', text: err instanceof Error ? err.message : '加载失败' } })
})

function closeDialog() {
  if (busy.value) return
  dialog.value = ''
  dialogError.value = ''
  target.value = null
}

function finishDialog() {
  busy.value = false
  dialog.value = ''
  dialogError.value = ''
  target.value = null
}

function openCreate() {
  form.username = ''
  form.password = ''
  form.confirm = ''
  form.role = 'user'
  dialogError.value = ''
  dialog.value = 'create'
}

function openReset(u: User) {
  target.value = u
  resetPass.value = ''
  resetConfirm.value = ''
  dialogError.value = ''
  dialog.value = 'reset'
}

function openRole(u: User) {
  target.value = u
  nextRole.value = u.role === 'admin' ? 'admin' : 'user'
  dialogError.value = ''
  dialog.value = 'role'
}

function openDelete(u: User) {
  target.value = u
  dialogError.value = ''
  dialog.value = 'delete'
}

async function toggleRegister() {
  banner.value = null
  try {
    const next = !allowRegister.value
    const res = await api.setRegisterPolicy(next)
    allowRegister.value = res.allow_register
    auth.status = { has_users: true, allow_register: res.allow_register }
    store.flash(next ? '已开放公开注册' : '已关闭公开注册')
  } catch (err) {
    banner.value = { type: 'error', text: err instanceof Error ? err.message : '保存失败' }
  }
}

async function create() {
  dialogError.value = ''
  if (!form.username.trim()) {
    dialogError.value = '请填写用户名'
    return
  }
  if (form.password.length < 6) {
    dialogError.value = '密码至少 6 位'
    return
  }
  if (form.password !== form.confirm) {
    dialogError.value = '两次输入的密码不一致'
    return
  }
  busy.value = true
  try {
    await api.createUser({ username: form.username.trim(), password: form.password, role: form.role })
    store.flash('账号已创建')
    finishDialog()
    await load()
  } catch (err) {
    dialogError.value = err instanceof Error ? err.message : '创建失败'
  } finally {
    busy.value = false
  }
}

async function confirmReset() {
  if (!target.value) return
  dialogError.value = ''
  if (resetPass.value.length < 6) {
    dialogError.value = '密码至少 6 位'
    return
  }
  if (resetPass.value !== resetConfirm.value) {
    dialogError.value = '两次输入的密码不一致'
    return
  }
  busy.value = true
  try {
    await api.patchUser(target.value.id, { password: resetPass.value })
    store.flash(`已重置 ${target.value.username} 的密码`)
    finishDialog()
  } catch (err) {
    dialogError.value = err instanceof Error ? err.message : '重置失败'
  } finally {
    busy.value = false
  }
}

async function confirmRole() {
  if (!target.value) return
  dialogError.value = ''
  busy.value = true
  try {
    await api.patchUser(target.value.id, { role: nextRole.value })
    if (target.value.id === auth.user?.id) {
      auth.user = { ...auth.user, role: nextRole.value }
    }
    store.flash('角色已更新')
    finishDialog()
    await load()
  } catch (err) {
    dialogError.value = err instanceof Error ? err.message : '更新失败'
  } finally {
    busy.value = false
  }
}

async function confirmDelete() {
  if (!target.value) return
  dialogError.value = ''
  busy.value = true
  try {
    const name = target.value.username
    await api.deleteUser(target.value.id)
    store.flash(`已删除 ${name}`)
    finishDialog()
    await load()
  } catch (err) {
    dialogError.value = err instanceof Error ? err.message : '删除失败'
  } finally {
    busy.value = false
  }
}

function initial(name: string) {
  return (name || '?').slice(0, 1).toUpperCase()
}

function formatTime(iso: string) {
  try {
    return new Date(iso).toLocaleString('zh-CN', { hour12: false })
  } catch {
    return iso
  }
}
</script>

<style scoped>
.page { display: grid; gap: 18px; }
.stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; }
.stat { padding: 16px 18px; }
.num { font-size: 32px; margin: 8px 0 2px; }
.hint { color: var(--muted); font-size: 13px; line-height: 1.5; }
.register .switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 10px;
}
.switch-title { font-size: 14px; font-weight: 600; }
.switch {
  width: 44px;
  height: 26px;
  border: 0;
  border-radius: 99px;
  background: rgba(255,255,255,0.12);
  position: relative;
  cursor: pointer;
  flex-shrink: 0;
}
.switch i {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #d7dde8;
  transition: 0.16s ease;
}
.switch.on { background: var(--mint); }
.switch.on i { left: 21px; background: #06221d; }
.table-card { padding: 8px 0 0; overflow: hidden; }
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 16px;
  padding: 18px 22px 14px;
}
.toolbar-actions { display: flex; gap: 10px; align-items: end; }
.search { display: grid; gap: 6px; }
.search span { font-size: 12px; color: var(--muted); letter-spacing: 0.08em; text-transform: uppercase; }
.search .input { width: 220px; padding: 10px 12px; }
.banner { margin: 0 22px 8px; font-size: 13px; }
.banner.error { color: var(--rose); }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; }
th, td { text-align: left; padding: 14px 22px; border-top: 1px solid var(--line); vertical-align: middle; }
th { color: var(--muted); font-size: 11px; letter-spacing: 0.12em; text-transform: uppercase; font-weight: 500; }
tbody tr:hover td { background: rgba(255,255,255,0.02); }
.right { text-align: right; }
.who { display: flex; align-items: center; gap: 12px; }
.avatar {
  width: 36px; height: 36px; border-radius: 12px;
  display: grid; place-items: center;
  background: rgba(255,255,255,0.06);
  color: var(--text);
  font-family: Sora, sans-serif;
  font-weight: 700;
}
.avatar.admin { background: var(--mint-dim); color: var(--mint); }
.name { display: flex; align-items: center; gap: 8px; font-weight: 600; }
.id { font-size: 11px; color: var(--faint); margin-top: 2px; }
.me {
  font-size: 11px;
  color: var(--mint);
  border: 1px solid rgba(94,234,212,0.28);
  border-radius: 999px;
  padding: 1px 8px;
  font-weight: 500;
}
.role {
  display: inline-flex;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  border: 1px solid var(--line);
}
.role.admin { color: var(--mint); border-color: rgba(94,234,212,0.28); background: var(--mint-dim); }
.role.user { color: var(--muted); }
.jobs { font-size: 14px; }
.muted { color: var(--muted); font-size: 13px; }
.ops { display: inline-flex; gap: 4px; justify-content: flex-end; }
.text-btn {
  border: 0;
  background: transparent;
  color: var(--muted);
  padding: 6px 8px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
}
.text-btn:hover { background: var(--bg-soft); color: var(--text); }
.text-btn.danger { color: var(--rose); }
.text-btn:disabled { opacity: 0.35; cursor: not-allowed; }
.empty { text-align: center; color: var(--muted); padding: 36px 22px !important; }
.form { display: grid; gap: 14px; }
.lead { color: var(--muted); line-height: 1.65; }
.lead.warn { color: var(--rose); }
.error { color: var(--rose); font-size: 13px; }
@media (max-width: 1100px) {
  .stats { grid-template-columns: 1fr 1fr; }
  .toolbar, .toolbar-actions { flex-direction: column; align-items: stretch; }
  .search .input { width: 100%; }
}
</style>
