<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Bookmark,
  Check,
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
const MY_IMAGES_PAGE_SIZE = 8
const user = ref(null)
const adminAccess = ref(false)
const authMode = ref('login')
const authForm = ref({ username: '', password: '', display_name: '', captcha_code: '' })
const authModalOpen = ref(false)
const captcha = ref(null)
const prompt = ref('')
const createMode = ref('generate')
const generationParams = ref({
  size: 'auto',
  quality: 'auto',
  output_format: 'png',
  output_compression: null,
  moderation: 'auto',
})
const sizePickerOpen = ref(false)
const sizeMode = ref('auto')
const sizeTier = ref('1K')
const sizeRatio = ref('1:1')
const customRatio = ref('16:9')
const customWidth = ref('1024')
const customHeight = ref('1024')
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
const myImagesLoaded = ref(false)
const myImagesLoading = ref(false)
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
const compressionDisabled = computed(() => generationParams.value.output_format === 'png')
const ratioOptions = [
  '1:1',
  '3:2',
  '2:3',
  '16:9',
  '9:16',
  '4:3',
  '3:4',
  '21:9',
]
const displayedSize = computed(() => normalizeImageSize(generationParams.value.size || 'auto') || 'auto')
const previewSize = computed(() => {
  if (sizeMode.value === 'auto') return 'auto'
  if (sizeMode.value === 'ratio') {
    const ratio = sizeRatio.value === 'custom' ? customRatio.value : sizeRatio.value
    return calculateImageSize(sizeTier.value, ratio)
  }
  const width = Number(customWidth.value)
  const height = Number(customHeight.value)
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return ''
  return normalizeImageSize(`${width}x${height}`)
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

async function loadMyImages() {
  if (!user.value) {
    myImages.value = []
    return
  }
  if (myImagesLoading.value) return
  myImagesLoading.value = true
  try {
    myImages.value = await api.myImages(0, MY_IMAGES_PAGE_SIZE)
    myImagesLoaded.value = true
  } catch (error) {
    message.value = error.message
  } finally {
    myImagesLoading.value = false
  }
}

async function submitAuth() {
  authLoading.value = true
  message.value = ''
  try {
    const payload = {
      username: authForm.value.username,
      password: authForm.value.password,
      captcha_code: authForm.value.captcha_code,
      captcha_token: captcha.value?.token || '',
    }
    if (authMode.value === 'register') payload.display_name = authForm.value.display_name
    user.value = authMode.value === 'login' ? await api.login(payload) : await api.register(payload)
    authForm.value = { username: '', password: '', display_name: '', captcha_code: '' }
    authModalOpen.value = false
    myImagesLoaded.value = false
    myImages.value = []
    await refreshAdminAccess()
    await loadImages(true)
    if (activeTab.value === 'create') await loadMyImages()
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
  myImagesLoaded.value = false
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
    const params = normalizedGenerationParams()
    const created =
      createMode.value === 'edit'
        ? await api.editImage(cleanPrompt, sourceFile.value, params)
        : await api.createImage(cleanPrompt, params)
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

function openSizePicker() {
  if (generating.value) return
  const size = generationParams.value.size || 'auto'
  const preset = findSizePreset(size)
  const parsed = parseSize(size)
  sizePickerOpen.value = true
  if (size === 'auto') {
    sizeMode.value = 'auto'
  } else if (preset) {
    sizeMode.value = 'ratio'
    sizeTier.value = preset.tier
    sizeRatio.value = preset.ratio
  } else if (parsed) {
    sizeMode.value = 'resolution'
    customWidth.value = String(parsed.width)
    customHeight.value = String(parsed.height)
  } else {
    sizeMode.value = 'auto'
  }
}

function applySize() {
  if (!previewSize.value) return
  generationParams.value.size = previewSize.value
  sizePickerOpen.value = false
}

function parseSize(size) {
  const match = String(size).match(/^\s*(\d+)\s*[xX×]\s*(\d+)\s*$/)
  if (!match) return null
  return { width: Number(match[1]), height: Number(match[2]) }
}

function parseRatio(value) {
  const match = String(value).match(/^\s*(\d+(?:\.\d+)?)\s*[:xX×]\s*(\d+(?:\.\d+)?)\s*$/)
  if (!match) return null
  const width = Number(match[1])
  const height = Number(match[2])
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null
  return { width, height }
}

function calculateImageSize(tier, ratio) {
  const parsed = parseRatio(ratio)
  if (!parsed) return ''
  const base = tier === '4K' ? 3840 : tier === '2K' ? 2048 : 1024
  const { width: ratioWidth, height: ratioHeight } = parsed
  if (ratioWidth === ratioHeight) return normalizeImageSize(`${base}x${base}`)
  if (tier === '1K') {
    const shortSide = 1024
    const width = ratioWidth > ratioHeight ? roundToMultiple(shortSide * ratioWidth / ratioHeight, 16) : shortSide
    const height = ratioWidth > ratioHeight ? shortSide : roundToMultiple(shortSide * ratioHeight / ratioWidth, 16)
    return normalizeImageSize(`${width}x${height}`)
  }
  const width = ratioWidth > ratioHeight ? base : roundToMultiple(base * ratioWidth / ratioHeight, 16)
  const height = ratioWidth > ratioHeight ? roundToMultiple(base * ratioHeight / ratioWidth, 16) : base
  return normalizeImageSize(`${width}x${height}`)
}

function normalizeImageSize(size) {
  if (!size || size === 'auto') return 'auto'
  const parsed = parseSize(size)
  if (!parsed) return ''
  let width = roundToMultiple(parsed.width, 16)
  let height = roundToMultiple(parsed.height, 16)
  for (let i = 0; i < 4; i += 1) {
    const maxEdge = Math.max(width, height)
    if (maxEdge > 3840) {
      const scale = 3840 / maxEdge
      width = floorToMultiple(width * scale, 16)
      height = floorToMultiple(height * scale, 16)
    }
    if (width / height > 3) width = floorToMultiple(height * 3, 16)
    if (height / width > 3) height = floorToMultiple(width * 3, 16)
    const pixels = width * height
    if (pixels > 8294400) {
      const scale = Math.sqrt(8294400 / pixels)
      width = floorToMultiple(width * scale, 16)
      height = floorToMultiple(height * scale, 16)
    }
    if (pixels < 655360) {
      const scale = Math.sqrt(655360 / pixels)
      width = ceilToMultiple(width * scale, 16)
      height = ceilToMultiple(height * scale, 16)
    }
  }
  return `${width}x${height}`
}

function findSizePreset(size) {
  const normalized = normalizeImageSize(size)
  for (const tier of ['1K', '2K', '4K']) {
    for (const ratio of ratioOptions) {
      if (calculateImageSize(tier, ratio) === normalized) return { tier, ratio }
    }
  }
  return null
}

function roundToMultiple(value, multiple) {
  return Math.max(multiple, Math.round(value / multiple) * multiple)
}

function floorToMultiple(value, multiple) {
  return Math.max(multiple, Math.floor(value / multiple) * multiple)
}

function ceilToMultiple(value, multiple) {
  return Math.max(multiple, Math.ceil(value / multiple) * multiple)
}

function normalizedGenerationParams() {
  const params = { ...generationParams.value }
  params.size = normalizeImageSize(params.size) || 'auto'
  if (params.output_format === 'png') params.output_compression = null
  if (params.output_compression !== null) {
    const value = Number(params.output_compression)
    params.output_compression = Number.isFinite(value) ? Math.min(100, Math.max(0, Math.round(value))) : null
  }
  return params
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
      if (myImagesLoaded.value || activeTab.value === 'create') await loadMyImages()
      if (payload.status === 'ready') message.value = `${payload.image?.task_type === 'edit' ? '编辑' : '生成'}完成，已上传到画廊`
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
  if (tab === 'create' && user.value && !myImagesLoaded.value) await loadMyImages()
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
        <button type="button" :class="{ active: activeTab === 'gallery' }" @click="selectTab('gallery')">画廊</button>
        <button type="button" :class="{ active: activeTab === 'create' }" @click="selectTab('create')">我的</button>
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
        登录后进入我的生成工作台
      </div>
      <button class="primary-button wide" type="button" @click="openAuthModal('login')">
        <LogIn :size="18" />
        登录账号
      </button>
    </section>

    <section v-else-if="activeTab === 'create'" id="create" class="playground-shell">
      <div class="playground-header">
        <div>
          <div class="kicker">GPT Image Playground</div>
          <h1>我的工作台</h1>
          <p class="section-subtitle">在这里生成或编辑图片，完成后会自动进入公共画廊。</p>
        </div>
        <div class="playground-header-actions">
          <span class="pill success"><span></span>{{ queueState ? queueText() : '准备就绪' }}</span>
          <button class="ghost-button compact" type="button" @click="selectTab('gallery')">
            <ExternalLink :size="16" />
            查看画廊
          </button>
        </div>
      </div>

      <div class="playground-grid">
        <article class="playground-stat">
          <small>作品</small>
          <strong>{{ images.length }}</strong>
        </article>
        <article class="playground-stat">
          <small>完成</small>
          <strong>{{ stats.ready }}</strong>
        </article>
        <article class="playground-stat">
          <small>点赞</small>
          <strong>{{ stats.likes }}</strong>
        </article>
        <article class="playground-stat">
          <small>身份</small>
          <strong>{{ currentName }}</strong>
        </article>
      </div>

      <div v-if="queueState" class="playground-progress">
        <strong>{{ queueText() }}</strong>
        <span>队列 {{ queueState.queue.queued }} · 生成中 {{ queueState.queue.running }}</span>
      </div>

      <div v-if="myImagesLoading && myImages.length === 0" class="playground-empty">
        <RefreshCw class="spin" :size="28" />
        正在同步你的生成记录
      </div>
      <div v-else-if="myImages.length === 0" class="playground-empty">
        <ImagePlus :size="34" />
        <strong>输入提示词开始生成图片</strong>
        <span>生成成功后会自动保存，并展示在画廊顶部。</span>
      </div>
      <div v-else class="playground-task-grid">
        <article v-for="item in myImages.slice(0, 9)" :key="item.id" class="playground-task-card">
          <button v-if="item.status === 'ready'" class="playground-task-image" type="button" @click="openPreview(item)">
            <img :src="mediaUrl(item.image_url)" :alt="item.prompt" />
          </button>
          <div v-else class="playground-task-image muted">
            <RefreshCw v-if="item.status === 'running' || item.status === 'queued'" class="spin" :size="22" />
            <X v-else :size="22" />
            <span>{{ statusLabel(item.status) }}</span>
          </div>
          <div class="playground-task-body">
            <div>
              <strong>#{{ item.id }} · {{ taskLabel(item.task_type) }}</strong>
              <span>{{ item.completed_at || item.created_at }}</span>
            </div>
            <p>{{ item.prompt }}</p>
            <em v-if="item.error">{{ item.error }}</em>
          </div>
        </article>
      </div>

      <div class="playground-composer-wrap">
        <div class="playground-composer">
          <label v-if="createMode === 'edit'" class="playground-upload" :class="{ filled: sourcePreview }">
            <input type="file" accept="image/png,image/jpeg,image/webp" :disabled="generating" @change="handleSourceFile" />
            <template v-if="sourcePreview">
              <img :src="sourcePreview" alt="待编辑原图预览" />
              <span>更换参考图</span>
            </template>
            <template v-else>
              <UploadCloud :size="22" />
              <span>上传需要编辑的图片</span>
              <small>PNG / JPG / WEBP，最大 10MB</small>
            </template>
          </label>

          <div class="playground-composer-main">
            <textarea
              v-model="prompt"
              :placeholder="createMode === 'edit' ? '描述你希望如何修改这张图片...' : '描述你想生成的图片...'"
              :disabled="generating"
              @keydown.meta.enter.prevent="submitImageJob"
              @keydown.ctrl.enter.prevent="submitImageJob"
            ></textarea>

            <div class="playground-toolbar">
              <div class="playground-controls">
                <div class="mode-switch compact" role="tablist" aria-label="创作模式">
                  <button type="button" :class="{ active: createMode === 'generate' }" :disabled="generating" @click="setCreateMode('generate')">
                    <WandSparkles :size="15" /> 生成
                  </button>
                  <button type="button" :class="{ active: createMode === 'edit' }" :disabled="generating" @click="setCreateMode('edit')">
                    <ImagePlus :size="15" /> 编辑
                  </button>
                </div>
                <label class="playground-param">
                  <span>尺寸</span>
                  <button class="size-picker-button" type="button" :disabled="generating" @click="openSizePicker">
                    {{ displayedSize }}
                  </button>
                </label>
                <label class="playground-param">
                  <span>质量</span>
                  <select v-model="generationParams.quality" :disabled="generating">
                    <option value="auto">auto</option>
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                  </select>
                </label>
                <label class="playground-param">
                  <span>格式</span>
                  <select v-model="generationParams.output_format" :disabled="generating">
                    <option value="png">PNG</option>
                    <option value="jpeg">JPEG</option>
                    <option value="webp">WebP</option>
                  </select>
                </label>
                <label class="playground-param compression">
                  <span>压缩率</span>
                  <input
                    v-model.number="generationParams.output_compression"
                    type="number"
                    min="0"
                    max="100"
                    placeholder="0-100"
                    :disabled="generating || compressionDisabled"
                  />
                </label>
                <label class="playground-param">
                  <span>审核</span>
                  <select v-model="generationParams.moderation" :disabled="generating">
                    <option value="auto">auto</option>
                    <option value="low">low</option>
                  </select>
                </label>
              </div>

              <div class="playground-actions">
                <button v-if="sourcePreview && !generating" class="ghost-button compact" type="button" @click="clearSourceImage">
                  <Trash2 :size="16" />
                  移除
                </button>
                <button class="primary-button compact" :disabled="generating" type="button" @click="submitImageJob">
                  <RefreshCw v-if="generating" class="spin" :size="17" />
                  <WandSparkles v-else :size="17" />
                  {{ generating ? '等待结果' : createMode === 'edit' ? '提交编辑' : '生成图像' }}
                </button>
              </div>
            </div>
          </div>

          <p v-if="message" class="message playground-message">{{ message }}</p>
        </div>
      </div>

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
            {{ user ? '进入我的' : '登录后生成' }}
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

    <div v-if="sizePickerOpen" class="size-picker-modal" role="dialog" aria-modal="true" @click.self="sizePickerOpen = false">
      <section class="size-picker-dialog">
        <div class="size-picker-head">
          <div>
            <h2>设置图像尺寸</h2>
            <p>当前：{{ displayedSize }}</p>
          </div>
          <button class="modal-close inline" type="button" aria-label="关闭尺寸选择" @click="sizePickerOpen = false">
            <X :size="20" />
          </button>
        </div>

        <div class="size-mode-tabs">
          <button type="button" :class="{ active: sizeMode === 'auto' }" @click="sizeMode = 'auto'">自动</button>
          <button type="button" :class="{ active: sizeMode === 'ratio' }" @click="sizeMode = 'ratio'">按比例</button>
          <button type="button" :class="{ active: sizeMode === 'resolution' }" @click="sizeMode = 'resolution'">自定义宽高</button>
        </div>

        <div class="size-picker-body">
          <div v-if="sizeMode === 'auto'" class="size-auto-panel">
            <Sparkles :size="30" />
            <strong>自动尺寸</strong>
            <span>不传递具体分辨率，由模型决定生成尺寸。</span>
          </div>

          <template v-else-if="sizeMode === 'ratio'">
            <label class="size-picker-label">基准分辨率</label>
            <div class="size-choice-grid three">
              <button v-for="tier in ['1K', '2K', '4K']" :key="tier" type="button" :class="{ active: sizeTier === tier }" @click="sizeTier = tier">
                {{ tier }}
              </button>
            </div>

            <label class="size-picker-label">图像比例</label>
            <div class="size-choice-grid four">
              <button v-for="ratio in ratioOptions" :key="ratio" type="button" :class="{ active: sizeRatio === ratio }" @click="sizeRatio = ratio">
                {{ ratio }}
              </button>
              <button class="wide" type="button" :class="{ active: sizeRatio === 'custom' }" @click="sizeRatio = 'custom'">
                自定义比例
              </button>
            </div>

            <input
              v-if="sizeRatio === 'custom'"
              v-model="customRatio"
              class="size-custom-input"
              placeholder="例如 5:4 / 2.39:1"
            />
          </template>

          <template v-else>
            <label class="size-picker-label">输入具体像素值</label>
            <div class="size-resolution-row">
              <label>
                <span>宽度</span>
                <input v-model="customWidth" type="number" min="1" placeholder="1024" />
              </label>
              <span class="size-times">×</span>
              <label>
                <span>高度</span>
                <input v-model="customHeight" type="number" min="1" placeholder="1024" />
              </label>
            </div>
            <p class="size-limit-note">最终尺寸会自动规整为合法值：16 的倍数，最大边长 3840px，宽高比不超过 3:1。</p>
          </template>
        </div>

        <div class="size-preview">
          <span>将使用</span>
          <strong>{{ previewSize || '尺寸无效' }}</strong>
        </div>

        <div class="size-picker-actions">
          <button class="ghost-button" type="button" @click="sizePickerOpen = false">取消</button>
          <button class="primary-button" type="button" :disabled="!previewSize" @click="applySize">确定</button>
        </div>
      </section>
    </div>
  </main>
</template>
