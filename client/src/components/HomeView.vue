<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Bookmark,
  Check,
  ChevronDown,
  Copy,
  EyeOff,
  ExternalLink,
  Heart,
  ImagePlus,
  LogIn,
  LogOut,
  RefreshCw,
  Sparkles,
  Trash2,
  UploadCloud,
  UserPlus,
  UserRound,
  WandSparkles,
  X,
} from 'lucide-vue-next'
import { api, mediaUrl } from '../services/api'

const GALLERY_PAGE_SIZE = 24
const RECORD_PAGE_SIZE = 8
const user = ref(null)
const adminAccess = ref(false)
const authMode = ref('login')
const authForm = ref({ username: '', password: '', display_name: '', captcha_code: '' })
const authModalOpen = ref(false)
const captcha = ref(null)
const prompt = ref('')
const createMode = ref('generate')
const sourceFile = ref(null)
const sourcePreview = ref('')
const images = ref([])
const myImages = ref([])
const activeTab = ref('gallery')
const sort = ref('latest')
const loading = ref(true)
const galleryLoadingMore = ref(false)
const galleryHasMore = ref(true)
const galleryOffset = ref(0)
const recordsLoaded = ref(false)
const recordsLoading = ref(false)
const recordsHasMore = ref(true)
const recordsOffset = ref(0)
const authLoading = ref(false)
const generating = ref(false)
const message = ref('')
const queueState = ref(null)
const previewImage = ref(null)
const copiedPromptId = ref(null)
let eventSource = null
let copyResetTimer = null
let previewObjectUrl = ''

const stats = computed(() => {
  const ready = images.value.filter((item) => item.status === 'ready').length
  const likes = images.value.reduce((sum, item) => sum + item.likes, 0)
  return { ready, likes }
})

const currentName = computed(() => user.value?.display_name || '游客')
const canManageGallery = computed(() => adminAccess.value || Boolean(user.value?.is_admin))
const sortedLabel = computed(() => {
  if (sort.value === 'popular') return '最多点赞'
  if (sort.value === 'favorites') return '我的收藏'
  return '最新生成'
})

onMounted(() => {
  bootstrap()
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('scroll', handleGalleryScroll, { passive: true })
})
onBeforeUnmount(() => {
  closeEvents()
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('scroll', handleGalleryScroll)
  if (copyResetTimer) clearTimeout(copyResetTimer)
  clearSourceImage()
})

async function bootstrap() {
  loading.value = true
  try {
    user.value = await api.me()
  } catch {
    user.value = null
  }
  await refreshAdminAccess()
  await loadImages(true)
  loading.value = false
}

async function refreshAdminAccess() {
  try {
    await api.adminMe()
    adminAccess.value = true
  } catch {
    adminAccess.value = false
  }
}

async function loadImages(reset = false) {
  if (!reset && (galleryLoadingMore.value || !galleryHasMore.value || loading.value)) return
  if (reset) {
    loading.value = true
    galleryOffset.value = 0
    galleryHasMore.value = true
  } else {
    galleryLoadingMore.value = true
  }
  try {
    const offset = reset ? 0 : galleryOffset.value
    const rows = await api.images(sort.value, offset, GALLERY_PAGE_SIZE)
    images.value = reset ? rows : mergeImages(images.value, rows)
    galleryOffset.value = offset + rows.length
    galleryHasMore.value = rows.length === GALLERY_PAGE_SIZE
  } catch (error) {
    message.value = error.message
  } finally {
    if (reset) loading.value = false
    else galleryLoadingMore.value = false
  }
}

async function loadMyImages(reset = false) {
  if (!user.value) {
    myImages.value = []
    return
  }
  if (!reset && (recordsLoading.value || !recordsHasMore.value)) return
  if (reset) {
    recordsOffset.value = 0
    recordsHasMore.value = true
  }
  recordsLoading.value = true
  try {
    const offset = reset ? 0 : recordsOffset.value
    const rows = await api.myImages(offset, RECORD_PAGE_SIZE)
    myImages.value = reset ? rows : mergeImages(myImages.value, rows)
    recordsOffset.value = offset + rows.length
    recordsHasMore.value = rows.length === RECORD_PAGE_SIZE
    recordsLoaded.value = true
  } catch (error) {
    message.value = error.message
  } finally {
    recordsLoading.value = false
  }
}

async function submitAuth() {
  authLoading.value = true
  message.value = ''
  try {
    const payload = {
      ...authForm.value,
      captcha_token: captcha.value?.token || '',
    }
    user.value = authMode.value === 'login' ? await api.login(payload) : await api.register(payload)
    authForm.value = { username: '', password: '', display_name: '', captcha_code: '' }
    authModalOpen.value = false
    recordsLoaded.value = false
    myImages.value = []
    await refreshAdminAccess()
    await loadImages(true)
    if (activeTab.value === 'records') await loadMyImages(true)
  } catch (error) {
    message.value = error.message
    authForm.value.captcha_code = ''
    await loadCaptcha()
  } finally {
    authLoading.value = false
  }
}

async function logout() {
  closeEvents()
  await api.logout()
  user.value = null
  adminAccess.value = false
  activeTab.value = 'gallery'
  sort.value = 'latest'
  queueState.value = null
  myImages.value = []
  recordsLoaded.value = false
  recordsOffset.value = 0
  recordsHasMore.value = true
  await loadImages(true)
}

async function submitImageJob() {
  if (!user.value) {
    message.value = '请先登录后再提交提示词'
    openAuthModal('login')
    return
  }
  if (prompt.value.trim().length < 2) {
    message.value = '提示词至少需要 2 个字符'
    return
  }
  if (createMode.value === 'edit' && !sourceFile.value) {
    message.value = '请先上传需要编辑的图片'
    return
  }
  generating.value = true
  message.value = ''
  try {
    const cleanPrompt = prompt.value.trim()
    const created =
      createMode.value === 'edit'
        ? await api.editImage(cleanPrompt, sourceFile.value)
        : await api.createImage(cleanPrompt)
    images.value = [created, ...images.value.filter((item) => item.id !== created.id)]
    myImages.value = [created, ...myImages.value.filter((item) => item.id !== created.id)]
    prompt.value = ''
    if (createMode.value === 'edit') clearSourceImage()
    watchJob(created.id)
  } catch (error) {
    message.value = error.message
    generating.value = false
  }
}

function setCreateMode(mode) {
  if (generating.value) return
  createMode.value = mode
  message.value = ''
}

function handleSourceFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    message.value = '仅支持 PNG、JPG、WEBP 图片'
    return
  }
  if (file.size > 10 * 1024 * 1024) {
    message.value = '上传图片不能超过 10MB'
    return
  }
  clearSourceImage()
  sourceFile.value = file
  previewObjectUrl = URL.createObjectURL(file)
  sourcePreview.value = previewObjectUrl
  message.value = ''
}

function clearSourceImage() {
  sourceFile.value = null
  sourcePreview.value = ''
  if (previewObjectUrl) {
    URL.revokeObjectURL(previewObjectUrl)
    previewObjectUrl = ''
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
      await loadImages(true)
      if (recordsLoaded.value) await loadMyImages(true)
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
  await loadImages(true)
}

function handleGalleryScroll() {
  if (activeTab.value !== 'gallery') return
  const documentHeight = document.documentElement.scrollHeight
  const currentBottom = window.scrollY + window.innerHeight
  if (documentHeight - currentBottom < 700) loadImages(false)
}

async function selectTab(tab) {
  activeTab.value = tab
  message.value = ''
  if (tab === 'records' && user.value && !recordsLoaded.value) await loadMyImages(true)
}

function openCreateEntry() {
  selectTab('create')
}

function mergeImages(current, nextRows) {
  const seen = new Set(current.map((item) => item.id))
  return [...current, ...nextRows.filter((item) => !seen.has(item.id))]
}

async function openAuthModal(mode = 'login') {
  authMode.value = mode
  authModalOpen.value = true
  message.value = ''
  authForm.value.captcha_code = ''
  await loadCaptcha()
}

function closeAuthModal() {
  if (authLoading.value) return
  authModalOpen.value = false
}

async function switchAuthMode(mode) {
  authMode.value = mode
  authForm.value.captcha_code = ''
  message.value = ''
  await loadCaptcha()
}

async function loadCaptcha() {
  try {
    captcha.value = await api.captcha()
  } catch (error) {
    message.value = error.message
  }
}

function openPreview(image) {
  if (image.status !== 'ready' || !image.image_url) return
  previewImage.value = image
}

function closePreview() {
  previewImage.value = null
}

function handleKeydown(event) {
  if (event.key !== 'Escape') return
  if (previewImage.value) closePreview()
  else if (authModalOpen.value) closeAuthModal()
}

async function copyPrompt(image) {
  if (!image?.prompt) return
  try {
    await writeClipboard(image.prompt)
    copiedPromptId.value = image.id
    if (copyResetTimer) clearTimeout(copyResetTimer)
    copyResetTimer = setTimeout(() => {
      copiedPromptId.value = null
      copyResetTimer = null
    }, 1600)
  } catch {
    message.value = '复制提示词失败'
  }
}

async function writeClipboard(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
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

async function deleteGalleryImage(image) {
  const ok = window.confirm(`确定删除作品 #${image.id} 吗？这会从画廊移除该记录和本地图片文件。`)
  if (!ok) return
  try {
    await api.adminDeleteGeneration(image.id)
    images.value = images.value.filter((item) => item.id !== image.id)
    myImages.value = myImages.value.filter((item) => item.id !== image.id)
    if (previewImage.value?.id === image.id) closePreview()
    message.value = `已删除作品 #${image.id}`
  } catch (error) {
    message.value = error.message
  }
}

async function hideGalleryImage(image) {
  try {
    await api.adminSetGenerationHidden(image.id, true)
    images.value = images.value.filter((item) => item.id !== image.id)
    myImages.value = myImages.value.map((item) =>
      item.id === image.id ? { ...item, is_hidden: true } : item,
    )
    if (previewImage.value?.id === image.id) closePreview()
    message.value = `已隐藏作品 #${image.id}`
  } catch (error) {
    message.value = error.message
  }
}

function replaceImage(updated) {
  images.value = images.value.map((item) => (item.id === updated.id ? updated : item))
  myImages.value = myImages.value.map((item) => (item.id === updated.id ? updated : item))
}

function queueText() {
  if (!queueState.value) return ''
  const action = queueState.value.image?.task_type === 'edit' ? '编辑' : '生成'
  if (queueState.value.status === 'queued') return `排队中，当前第 ${queueState.value.position} 位`
  if (queueState.value.status === 'running') return `正在${action}，请保持页面打开`
  if (queueState.value.status === 'ready') return `${action}完成，已加入画廊`
  if (queueState.value.status === 'failed') return queueState.value.image?.error || `${action}失败`
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

function taskLabel(taskType) {
  return taskType === 'edit' ? '编辑' : '生成'
}
</script>

<template>
  <main class="page-shell home-workbench">
    <header class="app-topbar">
      <div class="brand-lockup">
        <span class="brand-mark"><Sparkles :size="18" /></span>
        <div>
          <strong>Img2Gallery</strong>
          <span>AI Prompt Gallery</span>
        </div>
      </div>
      <nav class="topbar-nav" aria-label="首页导航">
        <button type="button" :class="{ active: activeTab === 'create' }" @click="selectTab('create')">生成</button>
        <button type="button" :class="{ active: activeTab === 'gallery' }" @click="selectTab('gallery')">画廊</button>
        <button type="button" :class="{ active: activeTab === 'records' }" @click="selectTab('records')">记录</button>
      </nav>
      <div class="studio-actions topbar-actions">
        <template v-if="user">
          <div class="user-chip" :title="user.username">
            <span class="avatar tiny" :style="{ background: user.avatar_color }">{{ user.display_name.slice(0, 1).toUpperCase() }}</span>
            <strong>{{ user.display_name }}</strong>
          </div>
          <button class="header-icon-button" type="button" title="退出登录" aria-label="退出登录" @click="logout">
            <LogOut :size="18" />
          </button>
        </template>
        <template v-else>
          <button class="header-icon-button" type="button" title="登录" aria-label="登录" @click="openAuthModal('login')">
            <LogIn :size="18" />
          </button>
          <button class="header-icon-button accent" type="button" title="注册" aria-label="注册" @click="openAuthModal('register')">
            <UserPlus :size="18" />
          </button>
        </template>
      </div>
    </header>

    <section v-if="activeTab === 'create' && !user" class="create-panel tab-empty-panel">
      <div class="empty-state">
        <UserRound :size="26" />
        登录后才能使用生成图片功能
      </div>
      <button class="primary-button wide" type="button" @click="openAuthModal('login')">
        <LogIn :size="18" />
        登录账号
      </button>
    </section>

    <section v-else-if="activeTab === 'create'" id="create" class="workbench-console">
        <div class="console-main">
          <div class="studio-header">
            <div>
              <div class="kicker">Creative Console</div>
              <h1>输入提示词，开始生成</h1>
              <p class="section-subtitle">生成和编辑都在这里完成，结果自动进入本地画廊。</p>
            </div>
            <div class="console-status-strip" aria-label="运行摘要">
              <span><strong>{{ images.length }}</strong>作品</span>
              <span><strong>{{ stats.ready }}</strong>完成</span>
              <span><strong>{{ stats.likes }}</strong>点赞</span>
              <span><strong>{{ currentName }}</strong>身份</span>
            </div>
          </div>

          <div class="create-panel command-panel">
            <div class="section-heading">
              <div>
                <h2>{{ createMode === 'edit' ? '编辑图片' : '生成图片' }}</h2>
                <p class="section-subtitle">{{ user ? '提示词越具体，画面越稳定。' : '登录后才能提交任务。' }}</p>
              </div>
              <div class="mode-switch" role="tablist" aria-label="创作模式">
                <button type="button" :class="{ active: createMode === 'generate' }" :disabled="generating" @click="setCreateMode('generate')">
                  <WandSparkles :size="16" /> 生成
                </button>
                <button type="button" :class="{ active: createMode === 'edit' }" :disabled="generating" @click="setCreateMode('edit')">
                  <ImagePlus :size="16" /> 编辑
                </button>
              </div>
            </div>
            <label v-if="createMode === 'edit'" class="upload-box" :class="{ filled: sourcePreview }">
              <input type="file" accept="image/png,image/jpeg,image/webp" :disabled="generating" @change="handleSourceFile" />
              <template v-if="sourcePreview">
                <img :src="sourcePreview" alt="待编辑原图预览" />
                <span>更换图片</span>
              </template>
              <template v-else>
                <UploadCloud :size="28" />
                <strong>上传需要编辑的图片</strong>
                <small>支持 PNG、JPG、WEBP，最大 10MB</small>
              </template>
            </label>
            <button v-if="sourcePreview && !generating" class="ghost-button compact clear-upload" type="button" @click="clearSourceImage">
              <Trash2 :size="16" /> 移除原图
            </button>
            <textarea
              v-model="prompt"
              :placeholder="createMode === 'edit' ? '描述你希望如何修改这张图片...' : '描绘你心中的画面（支持详细的提示词）...'"
              :disabled="generating"
            ></textarea>
            <div v-if="queueState" class="queue-card">
              <strong>{{ queueText() }}</strong>
              <span>队列 {{ queueState.queue.queued }} · 生成中 {{ queueState.queue.running }}</span>
            </div>
            <div class="create-footer">
              <label><input type="checkbox" checked disabled />成功结果会保存到本地，并展示在公共画廊。</label>
              <button class="primary-button" :disabled="generating" @click="submitImageJob">
                <RefreshCw v-if="generating" class="spin" :size="18" />
                <WandSparkles v-else :size="18" />
                {{ generating ? '等待结果' : user ? (createMode === 'edit' ? '提交编辑' : '立即生成') : '登录后提交' }}
              </button>
            </div>
            <p v-if="message" class="message">{{ message }}</p>
          </div>
        </div>

        <aside class="console-rail">
          <div class="rail-card live">
            <span class="pill success"><span></span> API 就绪</span>
            <strong>{{ queueState ? queueText() : '等待新的创作任务' }}</strong>
            <small>{{ queueState ? `队列 ${queueState.queue.queued} · 生成中 ${queueState.queue.running}` : '提交后会实时显示排队位置' }}</small>
          </div>
          <button v-if="user" class="rail-record-button" type="button" @click="selectTab('records')">
            <ChevronDown :size="17" />
            查看我的记录
          </button>
        </aside>
    </section>

    <section v-if="activeTab === 'records'" class="records-section">
      <template v-if="!user">
        <div class="empty-state">
          <UserRound :size="26" />
          登录后查看你的生成记录
        </div>
        <button class="primary-button wide" type="button" @click="openAuthModal('login')">
          <LogIn :size="18" />
          登录账号
        </button>
      </template>
      <template v-else>
        <div class="section-heading">
          <h2>我的生成记录</h2>
          <div class="records-actions">
            <button class="ghost-button compact" :disabled="recordsLoading" @click="loadMyImages(true)">刷新</button>
            <button class="ghost-button compact collapse-toggle active" @click="selectTab('gallery')">
              <ChevronDown :size="17" />
              返回画廊
            </button>
          </div>
        </div>
        <div v-if="recordsLoading && myImages.length === 0" class="empty-state small-empty">正在加载生成记录</div>
        <div v-else-if="recordsLoaded && myImages.length === 0" class="empty-state small-empty">暂无生成记录</div>
        <div v-else class="records-list">
          <article v-for="item in myImages" :key="item.id" class="record-row" :class="{ 'has-source': item.source_image_url }">
            <button v-if="item.source_image_url" class="record-source" type="button" @click="openPreview(item)">
              <img :src="mediaUrl(item.source_image_url)" alt="编辑原图" />
            </button>
            <div>
              <strong>#{{ item.id }} · {{ taskLabel(item.task_type) }} · {{ statusLabel(item.status) }}{{ item.is_hidden ? ' · 已隐藏' : '' }}</strong>
              <p>{{ item.prompt }}</p>
              <span>{{ item.completed_at || item.created_at }}</span>
              <em v-if="item.error">{{ item.error }}</em>
            </div>
            <button
              type="button"
              class="tiny-button"
              :class="{ disabled: item.status !== 'ready' }"
              :disabled="item.status !== 'ready'"
              @click="openPreview(item)"
            >
              打开图片
            </button>
          </article>
          <button v-if="recordsHasMore" class="ghost-button records-more" :disabled="recordsLoading" @click="loadMyImages(false)">
            <RefreshCw v-if="recordsLoading" class="spin" :size="17" />
            {{ recordsLoading ? '加载中' : '加载更多记录' }}
          </button>
        </div>
      </template>
    </section>

    <section v-if="activeTab === 'gallery'" id="gallery" class="gallery-section">
      <div class="section-heading">
        <div>
          <h2>灵感流</h2>
          <p class="section-subtitle">{{ sortedLabel }} · 下滑自动加载更多作品</p>
        </div>
        <div class="gallery-heading-actions">
          <div class="segmented" :class="{ 'two-options': !user }">
            <button :class="{ active: sort === 'latest' }" @click="setSort('latest')">最新生成</button>
            <button :class="{ active: sort === 'popular' }" @click="setSort('popular')">最多点赞</button>
            <button v-if="user" :class="{ active: sort === 'favorites' }" @click="setSort('favorites')">我的收藏</button>
          </div>
          <button class="primary-button compact create-entry-button" type="button" @click="openCreateEntry">
            <WandSparkles :size="17" />
            {{ user ? '生成图片' : '登录后生成' }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="empty-state"><Sparkles :size="24" />正在同步画廊</div>
      <div v-else-if="images.length === 0" class="empty-state"><ImagePlus :size="26" />{{ sortedLabel }} 暂无作品</div>
      <div v-else class="gallery-grid">
        <article v-for="image in images" :key="image.id" class="gallery-card">
          <button v-if="image.status === 'ready'" class="image-frame preview-trigger" type="button" @click="openPreview(image)">
            <img :src="mediaUrl(image.image_url)" :alt="image.prompt" />
            <span v-if="image.task_type === 'edit'" class="task-badge">编辑生成</span>
            <span class="preview-hint">查看大图</span>
          </button>
          <div v-else class="image-frame">
            <div class="failed-state">
              <strong>{{ image.status === 'failed' ? `${taskLabel(image.task_type)}失败` : image.status === 'running' ? `${taskLabel(image.task_type)}中` : '排队中' }}</strong>
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
                <button
                  v-if="canManageGallery"
                  :aria-label="`隐藏作品 #${image.id}`"
                  title="隐藏作品"
                  @click="hideGalleryImage(image)"
                >
                  <EyeOff :size="18" />
                </button>
                <button
                  v-if="canManageGallery"
                  class="danger"
                  :aria-label="`删除作品 #${image.id}`"
                  title="删除作品"
                  @click="deleteGalleryImage(image)"
                >
                  <Trash2 :size="18" />
                </button>
              </div>
            </div>
          </div>
        </article>
      </div>
      <div v-if="!loading" class="load-more-state">
        <button v-if="galleryHasMore" class="ghost-button compact" :disabled="galleryLoadingMore" @click="loadImages(false)">
          <RefreshCw v-if="galleryLoadingMore" class="spin" :size="17" />
          {{ galleryLoadingMore ? '加载中' : '加载更多' }}
        </button>
        <span v-else-if="images.length">已经到底了</span>
      </div>
    </section>

    <div v-if="previewImage" class="preview-modal" role="dialog" aria-modal="true" @click.self="closePreview">
      <button class="preview-close" type="button" aria-label="关闭预览" title="关闭" @click="closePreview">
        <X :size="28" />
      </button>

      <div class="preview-shell">
        <div class="preview-stage">
          <img :src="mediaUrl(previewImage.image_url)" :alt="previewImage.prompt" />
        </div>

        <section class="preview-info">
          <p class="preview-prompt">{{ previewImage.prompt }}</p>
          <div v-if="previewImage.source_image_url" class="source-preview-line">
            <img :src="mediaUrl(previewImage.source_image_url)" alt="编辑原图" />
            <span>由这张原图编辑生成</span>
          </div>
          <div class="preview-meta">
            <div class="preview-author">
              <div class="avatar small" :style="{ background: previewImage.author.avatar_color }">
                {{ previewImage.author.display_name.slice(0, 1).toUpperCase() }}
              </div>
              <div>
                <strong>{{ previewImage.author.display_name }}</strong>
                <span>{{ dateOnly(previewImage.created_at) }} {{ timeOnly(previewImage.created_at) }}</span>
              </div>
            </div>
            <div class="preview-actions">
              <button class="preview-action" type="button" @click="copyPrompt(previewImage)">
                <Check v-if="copiedPromptId === previewImage.id" :size="17" />
                <Copy v-else :size="17" />
                {{ copiedPromptId === previewImage.id ? '已复制提示词' : '复制提示词' }}
              </button>
              <a class="preview-action solid" :href="mediaUrl(previewImage.image_url)" target="_blank" rel="noopener noreferrer">
                <ExternalLink :size="17" />
                打开原图
              </a>
              <button v-if="canManageGallery" class="preview-action" type="button" @click="hideGalleryImage(previewImage)">
                <EyeOff :size="17" />
                隐藏作品
              </button>
              <button v-if="canManageGallery" class="preview-action danger" type="button" @click="deleteGalleryImage(previewImage)">
                <Trash2 :size="17" />
                删除作品
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>

    <div v-if="authModalOpen" class="auth-modal" role="dialog" aria-modal="true" @click.self="closeAuthModal">
      <form class="auth-dialog" @submit.prevent="submitAuth">
        <button class="modal-close" type="button" aria-label="关闭登录" @click="closeAuthModal">
          <X :size="22" />
        </button>
        <div class="auth-dialog-head">
          <div class="avatar guest"><UserRound :size="26" /></div>
          <div>
            <h2>{{ authMode === 'login' ? '账号登录' : '创建账号' }}</h2>
            <p>{{ authMode === 'login' ? '登录后才能提交提示词生成图片' : '注册后即可开始生成和收藏作品' }}</p>
          </div>
        </div>
        <div class="auth-tabs">
          <button type="button" :class="{ active: authMode === 'login' }" @click="switchAuthMode('login')">登录</button>
          <button type="button" :class="{ active: authMode === 'register' }" @click="switchAuthMode('register')">注册</button>
        </div>
        <label>用户名<input v-model="authForm.username" autocomplete="username" required minlength="3" /></label>
        <label>密码<input v-model="authForm.password" type="password" :autocomplete="authMode === 'login' ? 'current-password' : 'new-password'" required minlength="6" /></label>
        <label v-if="authMode === 'register'">昵称<input v-model="authForm.display_name" /></label>
        <label>
          图片验证码
          <div class="captcha-row">
            <input v-model="authForm.captcha_code" autocomplete="off" inputmode="text" required minlength="4" maxlength="8" placeholder="输入验证码" />
            <button class="captcha-image" type="button" title="刷新验证码" @click="loadCaptcha">
              <img v-if="captcha" :src="captcha.image" alt="图片验证码" />
              <span v-else>刷新</span>
            </button>
          </div>
        </label>
        <button class="primary-button wide" :disabled="authLoading || !captcha">
          <UserRound :size="18" /> {{ authLoading ? '处理中' : authMode === 'login' ? '登录' : '注册' }}
        </button>
        <p v-if="message" class="message">{{ message }}</p>
      </form>
    </div>
  </main>
</template>
