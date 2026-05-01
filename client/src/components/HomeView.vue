<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Bookmark,
  Heart,
  ImagePlus,
  LogOut,
  RefreshCw,
  Sparkles,
  UserRound,
  WandSparkles,
} from 'lucide-vue-next'
import { api, mediaUrl } from '../services/api'

const user = ref(null)
const authMode = ref('login')
const authForm = ref({ username: '', password: '', display_name: '' })
const prompt = ref('')
const images = ref([])
const myImages = ref([])
const sort = ref('latest')
const loading = ref(true)
const authLoading = ref(false)
const generating = ref(false)
const message = ref('')
const queueState = ref(null)
let eventSource = null

const stats = computed(() => {
  const ready = images.value.filter((item) => item.status === 'ready').length
  const likes = images.value.reduce((sum, item) => sum + item.likes, 0)
  return { ready, likes }
})

const currentName = computed(() => user.value?.display_name || '游客')
const sortedLabel = computed(() => {
  if (sort.value === 'popular') return '最多点赞'
  if (sort.value === 'favorites') return '我的收藏'
  return '最新生成'
})

onMounted(bootstrap)
onBeforeUnmount(closeEvents)

async function bootstrap() {
  loading.value = true
  try {
    user.value = await api.me()
  } catch {
    user.value = null
  }
  await loadImages()
  if (user.value) await loadMyImages()
  loading.value = false
}

async function loadImages() {
  try {
    images.value = await api.images(sort.value)
  } catch (error) {
    message.value = error.message
  }
}

async function loadMyImages() {
  if (!user.value) {
    myImages.value = []
    return
  }
  try {
    myImages.value = await api.myImages()
  } catch (error) {
    message.value = error.message
  }
}

async function submitAuth() {
  authLoading.value = true
  message.value = ''
  try {
    const payload = { ...authForm.value }
    user.value = authMode.value === 'login' ? await api.login(payload) : await api.register(payload)
    authForm.value = { username: '', password: '', display_name: '' }
    await loadImages()
    await loadMyImages()
  } catch (error) {
    message.value = error.message
  } finally {
    authLoading.value = false
  }
}

async function logout() {
  closeEvents()
  await api.logout()
  user.value = null
  sort.value = 'latest'
  queueState.value = null
  myImages.value = []
  await loadImages()
}

async function generateImage() {
  if (!user.value) {
    message.value = '请先登录后再生成图片'
    return
  }
  if (prompt.value.trim().length < 2) {
    message.value = '提示词至少需要 2 个字符'
    return
  }
  generating.value = true
  message.value = ''
  try {
    const created = await api.createImage(prompt.value.trim())
    images.value = [created, ...images.value.filter((item) => item.id !== created.id)]
    myImages.value = [created, ...myImages.value.filter((item) => item.id !== created.id)]
    prompt.value = ''
    watchJob(created.id)
  } catch (error) {
    message.value = error.message
    generating.value = false
  }
}

function watchJob(id) {
  closeEvents()
  eventSource = new EventSource(api.jobEventsUrl(id), { withCredentials: true })
  eventSource.onmessage = async (event) => {
    const payload = JSON.parse(event.data)
    queueState.value = payload
    if (payload.image) replaceImage(payload.image)
    if (['ready', 'failed'].includes(payload.status)) {
      generating.value = false
      closeEvents()
      await loadImages()
      await loadMyImages()
    }
  }
  eventSource.onerror = () => {
    message.value = '队列状态连接中断，请稍后刷新查看结果'
    generating.value = false
    closeEvents()
  }
}

function closeEvents() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

async function setSort(nextSort) {
  sort.value = nextSort
  await loadImages()
}

async function toggleLike(image) {
  if (!user.value) {
    message.value = '登录后才能点赞'
    return
  }
  replaceImage(await api.like(image.id))
}

async function toggleFavorite(image) {
  if (!user.value) {
    message.value = '登录后才能收藏'
    return
  }
  replaceImage(await api.favorite(image.id))
}

function replaceImage(updated) {
  images.value = images.value.map((item) => (item.id === updated.id ? updated : item))
  myImages.value = myImages.value.map((item) => (item.id === updated.id ? updated : item))
}

function queueText() {
  if (!queueState.value) return ''
  if (queueState.value.status === 'queued') return `排队中，当前第 ${queueState.value.position} 位`
  if (queueState.value.status === 'running') return '正在生成，请保持页面打开'
  if (queueState.value.status === 'ready') return '生成完成，已加入画廊'
  if (queueState.value.status === 'failed') return queueState.value.image?.error || '生成失败'
  return ''
}

function dateOnly(value) {
  return new Date(value).toLocaleDateString('zh-CN')
}

function timeOnly(value) {
  return new Date(value).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function statusLabel(status) {
  const labels = { queued: '排队中', running: '生成中', ready: '已完成', failed: '失败' }
  return labels[status] || status
}
</script>

<template>
  <main class="page-shell">
    <section class="top-grid">
      <div class="hero-panel">
        <div class="kicker">AI Prompt Gallery</div>
        <h1>创意灵感画廊与<br /><span>生图大厅</span></h1>
        <p>探索社区创作的精彩瞬间。登录后即可提交提示词生成图片，作品会保存到本地画廊，并支持点赞、收藏与按热度筛选。</p>
      </div>

      <aside class="status-panel">
        <div class="status-title">
          <h2>运行状态</h2>
          <span class="pill success"><span></span> API 就绪</span>
        </div>
        <div class="stat-grid">
          <div><small>本地作品</small><strong>{{ images.length }}</strong></div>
          <div><small>成功生成</small><strong>{{ stats.ready }}</strong></div>
          <div><small>累计点赞</small><strong>{{ stats.likes }}</strong></div>
          <div><small>当前身份</small><strong class="identity">{{ currentName }}</strong></div>
        </div>
      </aside>
    </section>

    <section class="workspace-grid">
      <div class="create-panel">
        <div class="section-heading">
          <h2>开启创意之旅</h2>
          <span class="model-pill">排队生成</span>
        </div>
        <textarea v-model="prompt" placeholder="描绘你心中的画面（支持详细的提示词）..." :disabled="generating"></textarea>
        <div v-if="queueState" class="queue-card">
          <strong>{{ queueText() }}</strong>
          <span>队列 {{ queueState.queue.queued }} · 生成中 {{ queueState.queue.running }}</span>
        </div>
        <div class="create-footer">
          <label><input type="checkbox" checked disabled />提交的提示词及成功生成的图像将公开展示在本地画廊。</label>
          <button class="primary-button" :disabled="generating" @click="generateImage">
            <RefreshCw v-if="generating" class="spin" :size="18" />
            <WandSparkles v-else :size="18" />
            {{ generating ? '等待结果' : '立即生成' }}
          </button>
        </div>
        <p v-if="message" class="message">{{ message }}</p>
      </div>

      <aside class="account-panel">
        <template v-if="user">
          <div class="profile-row">
            <div class="avatar" :style="{ background: user.avatar_color }">{{ user.display_name.slice(0, 1).toUpperCase() }}</div>
            <div><h2>{{ user.display_name }}</h2><p>@{{ user.username }}</p></div>
          </div>
          <button class="ghost-button" @click="logout"><LogOut :size="18" /> 退出登录</button>
        </template>

        <form v-else class="auth-form" @submit.prevent="submitAuth">
          <div class="auth-tabs">
            <button type="button" :class="{ active: authMode === 'login' }" @click="authMode = 'login'">登录</button>
            <button type="button" :class="{ active: authMode === 'register' }" @click="authMode = 'register'">注册</button>
          </div>
          <label>用户名<input v-model="authForm.username" autocomplete="username" required minlength="3" /></label>
          <label>密码<input v-model="authForm.password" type="password" autocomplete="current-password" required minlength="6" /></label>
          <label v-if="authMode === 'register'">昵称<input v-model="authForm.display_name" /></label>
          <button class="primary-button wide" :disabled="authLoading">
            <UserRound :size="18" /> {{ authLoading ? '处理中' : authMode === 'login' ? '账号登录' : '创建账号' }}
          </button>
        </form>
      </aside>
    </section>

    <section v-if="user" class="records-section">
      <div class="section-heading">
        <h2>我的生成记录</h2>
        <button class="ghost-button compact" @click="loadMyImages">刷新</button>
      </div>
      <div v-if="myImages.length === 0" class="empty-state small-empty">暂无生成记录</div>
      <div v-else class="records-list">
        <article v-for="item in myImages" :key="item.id" class="record-row">
          <div>
            <strong>#{{ item.id }} · {{ statusLabel(item.status) }}</strong>
            <p>{{ item.prompt }}</p>
            <span>{{ item.completed_at || item.created_at }}</span>
            <em v-if="item.error">{{ item.error }}</em>
          </div>
          <a
            class="tiny-button"
            :class="{ disabled: item.status !== 'ready' }"
            :href="item.status === 'ready' ? mediaUrl(item.image_url) : undefined"
            target="_blank"
            rel="noopener noreferrer"
          >
            打开图片
          </a>
        </article>
      </div>
    </section>

    <section class="gallery-section">
      <div class="section-heading">
        <h2>社区画廊</h2>
        <div class="segmented">
          <button :class="{ active: sort === 'latest' }" @click="setSort('latest')">最新生成</button>
          <button :class="{ active: sort === 'popular' }" @click="setSort('popular')">最多点赞</button>
          <button :class="{ active: sort === 'favorites' }" @click="setSort('favorites')">我的收藏</button>
        </div>
      </div>

      <div v-if="loading" class="empty-state"><Sparkles :size="24" />正在同步画廊</div>
      <div v-else-if="images.length === 0" class="empty-state"><ImagePlus :size="26" />{{ sortedLabel }} 暂无作品</div>
      <div v-else class="gallery-grid">
        <article v-for="image in images" :key="image.id" class="gallery-card">
          <div class="image-frame">
            <img v-if="image.status === 'ready'" :src="mediaUrl(image.image_url)" :alt="image.prompt" />
            <div v-else class="failed-state">
              <strong>{{ image.status === 'failed' ? '生成失败' : image.status === 'running' ? '生成中' : '排队中' }}</strong>
              <span>{{ image.error || image.prompt }}</span>
            </div>
          </div>
          <div class="card-body">
            <div class="author-line">
              <div class="avatar small" :style="{ background: image.author.avatar_color }">{{ image.author.display_name.slice(0, 1).toUpperCase() }}</div>
              <strong>{{ image.author.display_name }}</strong>
              <time>{{ dateOnly(image.created_at) }}</time>
            </div>
            <p class="prompt-text">{{ image.prompt }}</p>
            <div class="card-actions">
              <span>{{ timeOnly(image.created_at) }}</span>
              <div>
                <button :aria-label="image.liked_by_me ? '取消点赞' : '点赞'" :class="{ active: image.liked_by_me }" @click="toggleLike(image)">
                  <Heart :size="18" /> {{ image.likes }}
                </button>
                <button :aria-label="image.favorited_by_me ? '取消收藏' : '收藏'" :class="{ active: image.favorited_by_me }" @click="toggleFavorite(image)">
                  <Bookmark :size="18" /> {{ image.favorites }}
                </button>
              </div>
            </div>
          </div>
        </article>
      </div>
    </section>
  </main>
</template>
