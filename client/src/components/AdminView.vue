<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { LogOut, RefreshCw, Save, ShieldCheck, SlidersHorizontal, Users } from 'lucide-vue-next'
import { api } from '../services/api'

const authed = ref(false)
const loading = ref(true)
const saving = ref(false)
const password = ref('')
const message = ref('')
const dashboard = ref(null)
const users = ref([])
const generations = ref([])
const providers = ref([])
const providerForm = ref(defaultProvider())
const concurrency = ref(1)
const busyItem = ref('')
let refreshTimer = null

onMounted(bootstrap)
onBeforeUnmount(stopAutoRefresh)

function defaultProvider() {
  return {
    id: null,
    name: 'GPT Image 2',
    provider_type: 'openai_compatible',
    model: 'gpt-image-2',
    api_base: '',
    api_key: '',
    enabled: true,
    is_default: true,
  }
}

async function bootstrap() {
  try {
    await api.adminMe()
    await loadAdminData()
    authed.value = true
    startAutoRefresh()
  } catch (error) {
    authed.value = false
    message.value = error.message === '请先登录管理员' ? '' : error.message
  } finally {
    loading.value = false
  }
}

async function login() {
  message.value = ''
  saving.value = true
  try {
    await api.adminLogin({ password: password.value })
    password.value = ''
    await loadAdminData()
    authed.value = true
    startAutoRefresh()
  } catch (error) {
    authed.value = false
    message.value = error.message
  } finally {
    saving.value = false
  }
}

async function logout() {
  stopAutoRefresh()
  await api.adminLogout()
  authed.value = false
}

function startAutoRefresh() {
  stopAutoRefresh()
  refreshTimer = window.setInterval(() => {
    loadAdminData(false)
  }, 3000)
}

function stopAutoRefresh() {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
    refreshTimer = null
  }
}

async function loadAdminData(updateProviderForm = true) {
  const [dash, userRows, recordRows, providerRows] = await Promise.all([
    api.adminDashboard(),
    api.adminUsers(),
    api.adminGenerations(),
    api.adminProviders(),
  ])
  dashboard.value = dash
  users.value = userRows
  generations.value = recordRows
  providers.value = providerRows
  concurrency.value = dash.concurrency
  const active = providerRows.find((item) => item.is_default) || providerRows[0]
  if (active && updateProviderForm) providerForm.value = { ...active, api_key: '' }
}

async function saveConcurrency() {
  saving.value = true
  try {
    await api.adminSetConcurrency(Number(concurrency.value))
    await loadAdminData(false)
    message.value = '并发数已更新'
  } catch (error) {
    message.value = error.message
  } finally {
    saving.value = false
  }
}

async function saveProvider() {
  saving.value = true
  try {
    const payload = { ...providerForm.value }
    if (!payload.api_key) payload.api_key = null
    await api.adminSaveProvider(payload, payload.id)
    await loadAdminData()
    message.value = '模型提供商已保存'
  } catch (error) {
    message.value = error.message
  } finally {
    saving.value = false
  }
}

async function toggleUserAdmin(user) {
  busyItem.value = `user-${user.id}`
  try {
    await api.adminSetUserAdmin(user.id, !user.is_admin)
    await loadAdminData(false)
    message.value = user.is_admin ? '已取消管理员' : '已设置管理员'
  } catch (error) {
    message.value = error.message
  } finally {
    busyItem.value = ''
  }
}

async function deleteGeneration(item) {
  const ok = window.confirm(`确定删除作品 #${item.id} 吗？这会从画廊移除该记录和本地图片文件。`)
  if (!ok) return
  busyItem.value = `generation-${item.id}`
  try {
    await api.adminDeleteGeneration(item.id)
    await loadAdminData(false)
    message.value = `已删除作品 #${item.id}`
  } catch (error) {
    message.value = error.message
  } finally {
    busyItem.value = ''
  }
}

function editProvider(provider) {
  providerForm.value = { ...provider, api_key: '' }
}

function newProvider() {
  providerForm.value = {
    ...defaultProvider(),
    id: null,
    name: 'Grok Image',
    model: 'grok-image',
    api_base: '',
    is_default: false,
  }
}

function shortPrompt(text) {
  return text.length > 64 ? `${text.slice(0, 64)}...` : text
}
</script>

<template>
  <main class="admin-shell">
    <section v-if="loading" class="admin-card admin-login-card">正在加载</section>

    <section v-else-if="!authed" class="admin-card admin-login-card">
      <div class="admin-brand">
        <ShieldCheck :size="32" />
        <div>
          <h1>管理后台</h1>
          <p>普通账号登录状态下，仍需输入管理员密码或使用管理员账号进入。</p>
        </div>
      </div>
      <form class="auth-form" @submit.prevent="login">
        <label>管理员密码<input v-model="password" type="password" autocomplete="current-password" required /></label>
        <button class="primary-button wide" :disabled="saving"><ShieldCheck :size="18" /> {{ saving ? '正在进入' : '进入后台' }}</button>
      </form>
      <p v-if="message" class="message">{{ message }}</p>
    </section>

    <template v-else-if="dashboard">
      <header class="admin-header">
        <div>
          <span class="kicker">Admin Console</span>
          <h1>图像画廊管理</h1>
        </div>
        <button class="ghost-button compact" @click="logout"><LogOut :size="18" /> 退出</button>
      </header>

      <section class="admin-stats">
        <div class="admin-card"><small>用户数</small><strong>{{ dashboard.users }}</strong></div>
        <div class="admin-card"><small>队列中</small><strong>{{ dashboard.images.queued || 0 }}</strong></div>
        <div class="admin-card"><small>生成中</small><strong>{{ dashboard.images.running || 0 }}</strong></div>
        <div class="admin-card"><small>成功 / 失败</small><strong>{{ dashboard.images.ready || 0 }} / {{ dashboard.images.failed || 0 }}</strong></div>
      </section>

      <section class="admin-grid">
        <div class="admin-card">
          <div class="admin-section-title"><SlidersHorizontal :size="20" /><h2>生成配置</h2></div>
          <label class="admin-label">同时并发数
            <input v-model.number="concurrency" type="number" min="1" max="8" />
          </label>
          <button class="primary-button" :disabled="saving" @click="saveConcurrency">
            <Save :size="18" /> 保存并发数
          </button>
        </div>

        <div class="admin-card provider-card">
          <div class="admin-section-title"><RefreshCw :size="20" /><h2>模型提供商</h2></div>
          <div class="provider-list">
            <button v-for="provider in providers" :key="provider.id" @click="editProvider(provider)">
              <strong>{{ provider.name }}</strong>
              <span>{{ provider.model }} · {{ provider.api_key_set ? provider.api_key_preview : '未设置 key' }}</span>
            </button>
          </div>
          <button class="ghost-button compact" @click="newProvider">新增提供商</button>
        </div>
      </section>

      <section class="admin-card">
        <div class="admin-section-title"><Save :size="20" /><h2>提供商表单</h2></div>
        <div class="provider-form">
          <label>名称<input v-model="providerForm.name" /></label>
          <label>类型<input v-model="providerForm.provider_type" /></label>
          <label>模型<input v-model="providerForm.model" /></label>
          <label>API 地址<input v-model="providerForm.api_base" /></label>
          <label>API Key<input v-model="providerForm.api_key" type="password" placeholder="留空则保持原 key" /></label>
          <label class="check-row"><input v-model="providerForm.enabled" type="checkbox" />启用</label>
          <label class="check-row"><input v-model="providerForm.is_default" type="checkbox" />设为默认</label>
        </div>
        <button class="primary-button" :disabled="saving" @click="saveProvider"><Save :size="18" /> 保存提供商</button>
        <p v-if="message" class="message">{{ message }}</p>
      </section>

      <section class="admin-card">
        <div class="admin-section-title"><Users :size="20" /><h2>用户管理</h2></div>
        <div class="admin-table-wrap">
          <table class="admin-table">
            <thead><tr><th>用户</th><th>管理员</th><th>最近登录 IP</th><th>最近登录</th><th>总生成</th><th>成功</th><th>失败</th><th>进行中</th></tr></thead>
            <tbody>
              <tr v-for="item in users" :key="item.id">
                <td><strong>{{ item.display_name }}</strong><br /><span>@{{ item.username }}</span></td>
                <td>
                  <button class="tiny-button" :class="{ active: item.is_admin }" :disabled="busyItem === `user-${item.id}`" @click="toggleUserAdmin(item)">
                    {{ item.is_admin ? '已启用' : '设为管理员' }}
                  </button>
                </td>
                <td>{{ item.last_login_ip || '-' }}</td>
                <td>{{ item.last_login_at || '-' }}</td>
                <td>{{ item.total_generations }}</td>
                <td>{{ item.ready_count || 0 }}</td>
                <td>{{ item.failed_count || 0 }}</td>
                <td>{{ item.active_count || 0 }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="admin-card">
        <div class="admin-section-title split-title">
          <div><RefreshCw :size="20" /><h2>生成记录</h2></div>
          <button class="ghost-button compact" @click="loadAdminData(false)">刷新</button>
        </div>
        <div class="admin-table-wrap">
          <table class="admin-table">
            <thead><tr><th>ID</th><th>类型</th><th>用户</th><th>状态</th><th>IP</th><th>模型</th><th>提示词</th><th>完成时间</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="item in generations" :key="item.id">
                <td>{{ item.id }}</td>
                <td>{{ item.task_type === 'edit' ? '编辑' : '生成' }}</td>
                <td>{{ item.display_name }}<br /><span>@{{ item.username }}</span></td>
                <td>{{ item.status }}</td>
                <td>{{ item.request_ip || '-' }}</td>
                <td>{{ item.model || '-' }}</td>
                <td>{{ shortPrompt(item.prompt) }}</td>
                <td>{{ item.completed_at || item.created_at }}</td>
                <td>
                  <button class="tiny-button danger" :disabled="busyItem === `generation-${item.id}`" @click="deleteGeneration(item)">删除</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <section v-else class="admin-card admin-login-card">
      <div class="admin-brand">
        <ShieldCheck :size="32" />
        <div>
          <h1>管理后台</h1>
          <p>{{ message || '管理数据加载失败，请重新登录。' }}</p>
        </div>
      </div>
      <button class="primary-button wide" @click="authed = false">重新登录</button>
    </section>
  </main>
</template>
