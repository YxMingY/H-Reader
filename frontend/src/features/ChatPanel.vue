<template>
  <!-- 
    ChatPanel 组件 - AI 对话面板
    
    模块职责：
    1. 会话管理：展示、创建、加载、删除会话列表
    2. 消息展示：渲染用户和 AI 的对话消息（支持 Markdown）
    3. 消息输入：文本输入框 + 图片附件上传
    4. 流式响应：接收后端推送的实时回复并动态更新 UI
    5. 作用域隔离：支持书架级和书籍级两种会话作用域
    
    数据流向：
    - 用户输入 → sendMessage() → 后端 API → 事件监听 → UI 更新
    - 后端推送 → chat_chunk/chat_done 事件 → onStreamChunk/onStreamDone → 实时更新消息
  -->
  <div class="chat-panel">
    <!-- 头部区域：显示当前作用域信息和操作按钮 -->
    <div class="chat-header">
      <!-- 作用域副标题：显示当前是书架级还是书籍级对话 -->
      <div class="chat-header-copy">
        <p class="chat-subtitle">{{ scopeSubtitle }}</p>
      </div>

      <!-- 操作按钮组：根据当前状态显示不同按钮 -->
      <div class="chat-header-actions">
        <!-- 在会话详情页时显示"返回会话列表"按钮 -->
        <button
          v-if="activeSessionId"
          class="chat-header-btn"
          type="button"
          @click="backToSessions"
        >
          会话列表
        </button>
        <!-- 在会话列表页时显示"刷新"按钮 -->
        <button
          v-else
          class="chat-header-btn"
          type="button"
          @click="refreshSessions"
          :disabled="loadingSessions"
        >
          刷新
        </button>
        <!-- 仅在会话列表页时显示"新会话"按钮 -->
        <button v-if="!activeSessionId" class="chat-header-btn primary" type="button" @click="createSessionAndEnter" @mousedown="clearErrorBeforeAction">
          新会话
        </button>
        <!-- 关闭聊天面板按钮 -->
        <button class="chat-close" type="button" @click="emit('close')" aria-label="关闭聊天框">×</button>
      </div>
    </div>

    <!-- 会话列表视图：当没有活跃会话时显示 -->
    <div v-if="!activeSessionId" class="chat-sessions">
      <!-- 会话列表标题和计数 -->
      <div class="chat-section-head">
        <h4>会话</h4>
        <span class="chat-section-count">{{ sessions.length }}</span>
      </div>

      <!-- 加载状态提示 -->
      <div v-if="loadingSessions" class="chat-empty compact">正在加载会话...</div>
      <!-- 空状态提示 -->
      <div v-else-if="sessions.length === 0" class="chat-empty compact">
        当前作用域还没有会话
      </div>
      <!-- 会话列表项 -->
      <div v-else class="chat-session-list">
        <div
          v-for="session in sessions"
          :key="session.session_id"
          class="chat-session-item"
          :class="{ active: session.session_id === activeSessionId }"
          role="button"
          tabindex="0"
          @click="loadSession(session.session_id)"
          @keydown.enter.prevent="loadSession(session.session_id)"
          @keydown.space.prevent="loadSession(session.session_id)"
        >
          <!-- 会话主要信息区 -->
          <div class="chat-session-main">
            <div class="chat-session-title">{{ session.title }}</div>
            <!-- 会话元数据：消息数量和更新时间 -->
            <div class="chat-session-meta">
              <span>{{ session.message_count }} 条消息</span>
              <span>{{ session.updated_at }}</span>
            </div>
          </div>
          <!-- 删除会话按钮（阻止事件冒泡，避免触发加载会话） -->
          <button
            class="chat-session-delete"
            type="button"
            title="删除会话"
            @click.stop="deleteSession(session.session_id)"
          >
            ×
          </button>
        </div>
      </div>

      <!-- 错误消息显示 -->
      <p v-if="errorMessage" class="chat-error selector">{{ errorMessage }}</p>
    </div>

    <!-- 会话详情视图：当有活跃会话时显示 -->
    <div v-else class="chat-body">
      <!-- 消息滚动容器：包含所有对话消息 -->
      <div ref="messageViewportRef" class="chat-messages">
        <!-- 加载状态 -->
        <div v-if="loadingMessages" class="chat-empty">正在加载对话...</div>
        <!-- 空状态 -->
        <div v-else-if="messages.length === 0" class="chat-empty">
          当前会话还没有消息，输入内容开始对话
        </div>
        <!-- 消息列表 -->
        <div v-else class="chat-message-list">
          <div
            v-for="(message, index) in messages"
            :key="`${message.created_at}-${index}`"
            class="chat-message"
            :class="message.role"
          >
            <!-- AI 助手消息：使用 Markdown 渲染 -->
            <div
              v-if="message.role === 'assistant'"
              class="chat-message-content markdown-body"
              v-html="renderMarkdown(message.content)"
            ></div>
            <!-- 用户消息：纯文本显示 -->
            <div v-else class="chat-message-content">{{ message.content }}</div>
          </div>
        </div>
      </div>
    
      <!-- 消息输入区域：包含附件预览、输入框和操作按钮 -->
      <div class="chat-composer">
        <!-- 附件预览区：显示已选择的图片缩略图 -->
        <div v-if="attachments.length" class="attachment-strip">
          <div v-for="item in attachments" :key="item.id" class="attachment-thumb">
            <img :src="item.dataUrl" :alt="item.name" />
            <!-- 移除附件按钮 -->
            <button type="button" class="attachment-remove" @click="removeAttachment(item.id)">×</button>
          </div>
        </div>
        
    
        <!-- 文本输入框：支持多行输入、快捷键发送和粘贴图片 -->
        <textarea
          v-model="draft"
          class="chat-input"
          rows="3"
          placeholder="输入文本，或直接粘贴截图（Ctrl+V）"
          @keydown.meta.enter.prevent="sendMessage"
          @keydown.ctrl.enter.prevent="sendMessage"
          @keydown.alt.enter.prevent="sendMessage"
          @input="clearErrorOnInput"
          @paste="handlePaste"
        ></textarea>
    
        <!-- 操作按钮区：图片选择和发送按钮 -->
        <div class="composer-actions">
          <!-- 隐藏的文件选择器 -->
          <input
            ref="fileInputRef"
            class="file-input"
            type="file"
            accept="image/*"
            multiple
            @change="handleFileInput"
          />
          <!-- 触发文件选择器 -->
          <button class="chat-header-btn" type="button" @click="pickImages">添加图片</button>
          <!-- 发送按钮：发送中时禁用 -->
          <button class="chat-header-btn primary" type="button" @click="sendMessage" :disabled="sending">
            {{ sending ? '发送中…' : '发送' }}
          </button>
        </div>
    
        <!-- 错误消息显示 -->
        <p v-if="errorMessage" class="chat-error">{{ errorMessage }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
/**
 * ChatPanel 组件 - AI 对话面板
 * 
 * 核心功能模块：
 * 1. 会话管理模块：负责会话的 CRUD 操作
 * 2. 消息渲染模块：负责消息的展示和 Markdown 渲染
 * 3. 输入处理模块：负责文本输入和图片附件处理
 * 4. 流式通信模块：负责与后端的实时通信和状态同步
 * 5. 作用域管理模块：负责书架级/书籍级会话的隔离
 * 
 * 技术栈：
 * - Vue 3 Composition API
 * - MarkdownIt（Markdown 渲染）
 * - Highlight.js（代码高亮）
 * - Wails Events（前后端通信）
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import MarkdownIt from 'markdown-it';
import hljs from 'highlight.js';
import 'highlight.js/styles/github.css';
import { Events } from '@wailsio/runtime';
import { ChatService } from '../../bindings/hreader';

/**
 * ========================================
 * 1. Props 定义 - 组件输入参数
 * ========================================
 * 
 * 作用域类型说明：
 * - 'library': 书架级会话，跨所有书籍共享
 * - 'book': 书籍级会话，仅针对当前书籍
 * 
 * bookKey 用于唯一标识一本书（基于文件内容哈希），
 * 避免因文件路径变化导致会话丢失。
 */
const props = defineProps({
  scopeType: {
    type: String,
    default: 'library',
  },
  bookPath: {
    type: String,
    default: '',
  },
  bookTitle: {
    type: String,
    default: '',
  },
  bookKey: {
    type: String,
    default: '',
  },
});

// 事件发射器：向父组件通知关闭请求
const emit = defineEmits(['close']);

/**
 * ========================================
 * 2. 响应式状态定义
 * ========================================
 * 
 * 状态分类：
 * - 数据状态：sessions, messages, attachments
 * - UI 状态：loadingSessions, loadingMessages, sending, errorMessage
 * - 导航状态：activeSessionId
 * - 输入状态：draft
 * - 流式追踪：streamSessionId, streamAssistantIndex
 * - 引用状态：messageViewportRef, fileInputRef
 * - 资源清理：eventUnsubscribers
 */

// 会话列表：存储当前作用域下的所有会话摘要
const sessions = ref([]);
// 当前活跃的会话 ID：空字符串表示在会话列表页
const activeSessionId = ref('');
// 当前会话的消息列表：按时间顺序排列
const messages = ref([]);
// 输入框草稿文本
const draft = ref('');
// 附件列表：待发送的图片文件
const attachments = ref([]);
// 加载会话列表的状态标志
const loadingSessions = ref(false);
// 加载会话详情的状态标志
const loadingMessages = ref(false);
// 发送消息的状态标志
const sending = ref(false);
// 错误消息：统一在此处存储和显示
const errorMessage = ref('');
// 消息视口 DOM 引用：用于滚动控制
const messageViewportRef = ref(null);
// 文件选择器 DOM 引用：用于触发文件选择
const fileInputRef = ref(null);
// 当前流式响应的会话 ID：用于验证事件来源
const streamSessionId = ref('');
// 当前流式响应的 assistant 消息索引：用于追加内容
const streamAssistantIndex = ref(-1);
// 事件取消订阅函数列表：用于组件卸载时清理
const eventUnsubscribers = [];

/**
 * ========================================
 * 3. Markdown 渲染器配置
 * ========================================
 * 
 * 功能：
 * - 自动换行（breaks: true）
 * - 链接识别（linkify: true）
 * - 排版优化（typographer: true）
 * - 代码高亮（集成 highlight.js）
 * 
 * 高亮策略：
 * 1. 优先使用指定的语言进行高亮
 * 2. 如果指定语言失败，回退到自动检测
 * 3. 如果都失败，返回空字符串（不影响消息渲染）
 */
const markdownRenderer = new MarkdownIt({
  breaks: true,
  linkify: true,
  typographer: true,
  highlight(code, language) {
    // 尝试使用指定语言高亮
    if (language && hljs.getLanguage(language)) {
      try {
        const highlighted = hljs.highlight(code, { language, ignoreIllegals: true }).value;
        return `<pre class="hljs"><code class="language-${language}">${highlighted}</code></pre>`;
      } catch (err) {
        // 回退到纯文本输出，避免高亮失败影响消息渲染。
      }
    }

    // 回退到自动检测语言
    try {
      const highlighted = hljs.highlightAuto(code).value;
      return `<pre class="hljs"><code>${highlighted}</code></pre>`;
    } catch (err) {
      return '';
    }
  },
});

/**
 * ========================================
 * 4. 计算属性 - 作用域相关
 * ========================================
 * 
 * 这些计算属性根据当前作用域类型动态生成显示文本和标识符，
 * 确保 UI 正确反映当前的上下文环境。
 */

// 作用域标题：书籍名称或"全局书架"
const scopeTitle = computed(() => (props.scopeType === 'book' ? props.bookTitle || props.bookPath || '当前书籍' : '全局书架'));
// 作用域副标题：显示在头部，帮助用户理解当前对话范围
const scopeSubtitle = computed(() => (props.scopeType === 'book' ? `当前书籍：${scopeTitle.value}` : '当前作用域：书架'));
// 作用域书籍路径：仅在书籍级作用域时有值
const scopeBookPath = computed(() => (props.scopeType === 'book' ? props.bookPath : ''));
// 作用域唯一键：用于 watch 检测作用域切换
// 格式："library:library" 或 "book:<bookKey>"
const scopeKey = computed(() => `${props.scopeType}:${props.bookKey || scopeBookPath.value || 'library'}`);

/**
 * ========================================
 * 5. 工具函数
 * ========================================
 */

/**
 * 将消息视口滚动到底部
 * 
 * 实现细节：
 * - 使用 nextTick 等待 DOM 更新
 * - 使用多层 requestAnimationFrame 确保异步渲染完成后仍能正确滚动
 * - 适用于：新增消息、切换会话、流式响应等场景
 * 
 * 为什么需要多次滚动？
 * Markdown 渲染和图片加载是异步的，单次滚动可能发生在内容完全渲染之前，
 * 导致滚动位置不准确。通过多帧滚动可以覆盖这种延迟。
 */
const scrollToBottom = async () => {
  // 在切换会话/渲染 Markdown 后再滚动，避免只执行一次 nextTick 仍停在中部。
  await nextTick();
  const container = messageViewportRef.value;
  if (container) {
    container.scrollTop = container.scrollHeight;
    requestAnimationFrame(() => {
      container.scrollTop = container.scrollHeight;
      requestAnimationFrame(() => {
        container.scrollTop = container.scrollHeight;
      });
    });
  }
};

/**
 * 渲染 Markdown 内容为 HTML
 * @param {string} content - 原始文本内容
 * @returns {string} 渲染后的 HTML 字符串
 */
const renderMarkdown = (content) => markdownRenderer.render(String(content || ''));

/**
 * 规范化会话列表结果
 * 确保返回值始终是数组，避免 null/undefined 导致的错误
 * @param {*} result - 后端返回的会话列表
 * @returns {Array} 会话数组
 */
const normalizeSessions = (result) => (Array.isArray(result) ? result : []);

/**
 * ========================================
 * 6. 会话管理模块
 * ========================================
 * 
 * 这一组函数负责会话的 CRUD 操作，包括：
 * - refreshSessions: 拉取会话列表
 * - loadSession: 加载会话详情
 * - createSession: 创建新会话
 * - deleteSession: 删除会话
 * - upsertSessionSummary: 更新会话列表中的单个会话
 */

/**
 * 刷新会话列表
 * 
 * 流程：
 * 1. 调用后端 API 获取当前作用域的所有会话
 * 2. 如果有活跃会话，检查它是否仍在列表中
 * 3. 如果存在，重新加载该会话的消息
 * 4. 如果不存在，清空活跃会话状态
 * 
 * 用途：
 * - 组件初始化时
 * - 用户点击"刷新"按钮
 * - 发送消息后同步最新状态
 */
const refreshSessions = async () => {
  loadingSessions.value = true;
  errorMessage.value = '';
  try {
    sessions.value = normalizeSessions(await ChatService.ListSessions(props.scopeType, scopeBookPath.value));
    if (activeSessionId.value) {
      const existing = sessions.value.find((item) => item.session_id === activeSessionId.value);
      if (existing) {
        await loadSession(existing.session_id, { keepSelection: true });
      } else {
        activeSessionId.value = '';
        messages.value = [];
      }
    }
  } catch (err) {
    errorMessage.value = `加载会话失败：${err?.message || err}`;
  } finally {
    loadingSessions.value = false;
  }
};

/**
 * 插入或更新会话摘要到列表顶部
 * 
 * 策略：
 * - 先移除旧的相同 ID 的会话（避免重复）
 * - 将新的会话添加到列表开头（最新优先）
 * 
 * @param {Object} summary - 会话摘要对象
 */
const upsertSessionSummary = (summary) => {
  const next = sessions.value.filter((item) => item.session_id !== summary.session_id);
  next.unshift(summary);
  sessions.value = next;
};

/**
 * 加载指定会话的详细信息
 * 
 * 流程：
 * 1. 调用后端 API 获取会话详情（包括消息历史和 LLM 状态）
 * 2. 设置活跃会话 ID
 * 3. 填充消息列表
 * 4. 更新会话列表中的摘要（除非是保持选中状态）
 * 5. 滚动到底部显示最新消息
 * 
 * @param {string} sessionId - 会话 ID
 * @param {Object} options - 可选参数
 * @param {boolean} options.keepSelection - 是否保持当前选中状态（不更新列表）
 */
const loadSession = async (sessionId, options = {}) => {
  if (!sessionId) return;
  loadingMessages.value = true;
  errorMessage.value = '';
  try {
    const detail = await ChatService.LoadSession(props.scopeType, scopeBookPath.value, sessionId);
    activeSessionId.value = detail.summary.session_id;
    messages.value = detail.messages || [];
    if (!options.keepSelection) {
      upsertSessionSummary(detail.summary);
    }
    await scrollToBottom();
  } catch (err) {
    errorMessage.value = `加载对话失败：${err?.message || err}`;
  } finally {
    loadingMessages.value = false;
  }
};

/**
 * 创建新会话
 * 
 * 命名规则：
 * - 书籍级："{书名} 对话"
 * - 书架级："新会话"
 * 
 * @returns {Object|null} 创建的会话摘要，失败返回 null
 */
const createSession = async () => {
  errorMessage.value = '';
  try {
    const title = props.scopeType === 'book' ? `${scopeTitle.value} 对话` : '新会话';
    const summary = await ChatService.CreateSession(props.scopeType, scopeBookPath.value, title);
    upsertSessionSummary(summary);
    return summary;
  } catch (err) {
    errorMessage.value = `创建会话失败：${err?.message || err}`;
    return null;
  }
};

/**
 * 创建新会话并立即进入
 * 
 * 流程：
 * 1. 调用 createSession 创建会话
 * 2. 切换到会话详情页
 * 3. 清空输入状态
 * 4. 滚动到底部
 */
const createSessionAndEnter = async () => {
  const summary = await createSession();
  if (!summary) return;
  activeSessionId.value = summary.session_id;
  messages.value = [];
  draft.value = '';
  attachments.value = [];
  await scrollToBottom();
};

/**
 * 返回会话列表页
 * 
 * 清理状态：
 * - 清除活跃会话 ID
 * - 清空消息列表
 * - 清空输入草稿和附件
 * - 清除错误消息
 */
const backToSessions = () => {
  activeSessionId.value = '';
  messages.value = [];
  draft.value = '';
  attachments.value = [];
  errorMessage.value = '';
};

/**
 * 删除指定会话
 * 
 * 流程：
 * 1. 用户确认删除
 * 2. 调用后端 API 删除会话文件
 * 3. 从本地列表中移除
 * 4. 如果删除的是当前会话，切换到第一个会话或返回列表页
 * 
 * @param {string} sessionId - 要删除的会话 ID
 */
const deleteSession = async (sessionId) => {
  if (!window.confirm('确定删除这个会话吗？')) return;
  errorMessage.value = '';
  try {
    await ChatService.DeleteSession(props.scopeType, scopeBookPath.value, sessionId);
    sessions.value = sessions.value.filter((item) => item.session_id !== sessionId);
    if (activeSessionId.value === sessionId) {
      activeSessionId.value = '';
      messages.value = [];
      if (sessions.value.length > 0) {
        await loadSession(sessions.value[0].session_id, { keepSelection: true });
      }
    }
  } catch (err) {
    errorMessage.value = `删除会话失败：${err?.message || err}`;
  }
};

/**
 * ========================================
 * 7. 附件处理模块
 * ========================================
 * 
 * 功能：
 * - 读取本地图片文件并转换为 data URL
 * - 管理附件列表（添加、删除）
 * - 提供文件选择器触发接口
 * 
 * 为什么使用 data URL？
 * - 便于直接嵌入到消息中发送给后端
 * - 避免临时文件管理的复杂性
 * - 适合小图片（大图片应考虑上传到服务器）
 */

/**
 * 将文件读取为 data URL
 * 
 * 生成的 ID 包含：
 * - 文件名、大小、修改时间：确保唯一性
 * - 随机字符串：避免同名同大小文件的冲突
 * 
 * @param {File} file - 文件对象
 * @returns {Promise<Object>} 包含 id, name, dataUrl 的对象
 */
const readFileAsDataUrl = (file) => new Promise((resolve, reject) => {
  const reader = new FileReader();
  reader.onload = () => resolve({
    id: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2)}`,
    name: file.name,
    dataUrl: reader.result,
  });
  reader.onerror = () => reject(reader.error || new Error('读取图片失败'));
  reader.readAsDataURL(file);
});

/**
 * 触发文件选择器
 * 通过编程方式点击隐藏的 input 元素
 */
const pickImages = () => {
  fileInputRef.value?.click();
};

/**
 * 处理文件选择事件
 * 
 * 流程：
 * 1. 获取选中的文件列表
 * 2. 清空 input value（允许重复选择同一文件）
 * 3. 并行读取所有文件为 data URL
 * 4. 追加到附件列表
 * 
 * @param {Event} event - 文件选择事件
 */
const handleFileInput = async (event) => {
  const files = Array.from(event.target.files || []);
  event.target.value = '';
  if (!files.length) return;

  try {
    const nextItems = await Promise.all(files.map((file) => readFileAsDataUrl(file)));
    attachments.value = attachments.value.concat(nextItems);
  } catch (err) {
    errorMessage.value = `添加图片失败：${err?.message || err}`;
  }
};

/**
 * 移除指定附件
 * @param {string} id - 附件 ID
 */
const removeAttachment = (id) => {
  attachments.value = attachments.value.filter((item) => item.id !== id);
};

/**
 * 处理粘贴事件：从剪贴板中提取图片并添加到附件列表
 * 
 * 支持的粘贴来源：
 * - 截图软件（微信、QQ、系统截图等）
 * - 复制的图片文件
 * - 网页上复制的图片
 * 
 * @param {ClipboardEvent} event - 粘贴事件对象
 */
const handlePaste = async (event) => {
  console.log('Paste event triggered');
  const items = event.clipboardData?.items;
  if (!items) {
    console.log('No clipboard data');
    return;
  }

  const imageFiles = [];
  
  // 遍历剪贴板中的所有项目
  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    console.log(`Clipboard item ${i}: type=${item.type}, kind=${item.kind}`);
    
    // 检查是否是图片类型
    if (item.type.indexOf('image') !== -1) {
      const blob = item.getAsFile();
      if (blob) {
        console.log(`Found image: size=${blob.size}, type=${blob.type}`);
        // 生成一个文件名，使用时间戳
        const fileName = `pasted-image-${Date.now()}-${i}.png`;
        const file = new File([blob], fileName, { type: blob.type });
        imageFiles.push(file);
      }
    }
  }

  // 如果找到了图片，读取并添加到附件列表
  if (imageFiles.length > 0) {
    event.preventDefault(); // 阻止默认粘贴行为（避免粘贴图片的 base64 到文本框）
    console.log(`Processing ${imageFiles.length} pasted image(s)`);
    try {
      const nextItems = await Promise.all(imageFiles.map((file) => readFileAsDataUrl(file)));
      attachments.value = attachments.value.concat(nextItems);
      console.log(`Successfully pasted ${nextItems.length} image(s)`);
      
      // 显示成功提示（可选）
      if (nextItems.length === 1) {
        errorMessage.value = '✅ 已添加 1 张图片';
        // 2秒后自动清除提示
        setTimeout(() => {
          if (errorMessage.value === '✅ 已添加 1 张图片') {
            errorMessage.value = '';
          }
        }, 2000);
      } else {
        errorMessage.value = `✅ 已添加 ${nextItems.length} 张图片`;
        setTimeout(() => {
          if (errorMessage.value.startsWith('✅ 已添加')) {
            errorMessage.value = '';
          }
        }, 2000);
      }
    } catch (err) {
      console.error('Failed to process pasted images:', err);
      errorMessage.value = `粘贴图片失败：${err?.message || err}`;
    }
  } else {
    console.log('No images found in clipboard');
  }
};

/**
 * 当用户开始输入时清除错误消息
 * 提供更好的用户体验，避免错误消息一直显示
 * 
 * 注意：
 * - 只在发送成功后，用户开始输入新消息时清除
 * - 如果正在发送或刚刚失败，不要立即清除
 * - 不清除成功提示（以 ✅ 开头的消息）
 */
const clearErrorOnInput = () => {
  // 只有在没有正在发送的消息时才清除错误
  // 并且不清除成功提示
  if (errorMessage.value && !sending.value && !errorMessage.value.startsWith('✅')) {
    console.log('Clearing error message on input');
    errorMessage.value = '';
  }
};

/**
 * 当用户点击新会话按钮时清除错误消息
 */
const clearErrorBeforeAction = () => {
  errorMessage.value = '';
};

/**
 * ========================================
 * 8. 流式通信模块
 * ========================================
 * 
 * 这是 ChatPanel 的核心模块，负责与后端的实时通信。
 * 
 * 工作流程：
 * 1. 用户发送消息 → sendMessage()
 * 2. 前端创建 user 和 assistant 占位消息
 * 3. 调用后端 SendMessageStreamInSession API
 * 4. 后端在后台 goroutine 中处理，通过事件推送分片
 * 5. 前端监听 chat_chunk 事件，逐块更新 assistant 消息
 * 6. 后端完成后发送 chat_done 事件
 * 7. 前端刷新会话数据，确保与后端同步
 * 
 * 事件类型：
 * - chat_chunk: 流式分片（字符串）
 * - chat_done: 完整回复（字符串）
 * - chat_stream_error: 错误消息（字符串）
 */

/**
 * 重置流式发送的前端追踪状态
 * 
 * 在以下情况调用：
 * - 发送完成（成功或失败）
 * - 发生错误
 * - 组件卸载
 */
const resetStreamingState = () => {
  streamSessionId.value = '';
  streamAssistantIndex.value = -1;
  sending.value = false;
};

/**
 * 处理流式分片事件
 * 
 * 安全检查：
 * 1. 验证 streamSessionId 是否存在
 * 2. 验证事件来自当前活跃会话
 * 3. 验证 assistant 消息索引有效
 * 4. 验证目标消息角色为 assistant
 * 
 * 追加策略：
 * - 直接将分片拼接到现有内容后面
 * - 每次追加后滚动到底部
 * 
 * @param {Object} event - Wails 事件对象，data 字段包含文本分片
 */
const onStreamChunk = async (event) => {
  if (!streamSessionId.value) return;
  if (activeSessionId.value !== streamSessionId.value) return;

  const chunk = String(event?.data || '');
  if (!chunk) return;

  const index = streamAssistantIndex.value;
  if (index < 0 || index >= messages.value.length) return;

  const target = messages.value[index];
  if (!target || target.role !== 'assistant') return;

  target.content = (target.content || '') + chunk;
  await scrollToBottom();
};

/**
 * 处理流式完成事件
 * 
 * 流程：
 * 1. 更新 assistant 消息的最终内容（以防分片拼接不完整）
 * 2. 记录完成的会话 ID
 * 3. 重置流式状态
 * 4. 如果完成的是当前活跃会话，重新加载以确保数据一致
 * 5. 刷新会话列表（更新消息计数和时间戳）
 * 
 * @param {Object} event - Wails 事件对象，data 字段包含完整回复
 */
const onStreamDone = async (event) => {
  if (!streamSessionId.value) return;

  const full = String(event?.data || '');
  const index = streamAssistantIndex.value;
  if (
    activeSessionId.value === streamSessionId.value &&
    index >= 0 &&
    index < messages.value.length &&
    messages.value[index]?.role === 'assistant'
  ) {
    messages.value[index].content = full || messages.value[index].content || '';
  }

  const doneSessionId = streamSessionId.value;
  const shouldReloadActive = activeSessionId.value === doneSessionId;
  resetStreamingState();
  if (shouldReloadActive) {
    await loadSession(doneSessionId, { keepSelection: true });
  }
  await refreshSessions();
};

/**
 * 处理流式错误事件
 * 
 * 清理策略：
 * 1. 显示错误消息
 * 2. 如果 assistant 消息为空，删除占位消息（避免空白气泡）
 * 3. 重置流式状态
 * 4. 刷新会话列表
 * 
 * @param {Object} event - Wails 事件对象，data 字段包含错误信息
 */
const onStreamError = async (event) => {
  const msg = String(event?.data || '流式回复失败');
  console.error('Stream error:', msg);
  console.log('Setting errorMessage to:', msg);
  console.log('Current activeSessionId:', activeSessionId.value);
  console.log('Current streamSessionId:', streamSessionId.value);
  
  // 首先设置错误消息，确保它不会被后续操作清除
  errorMessage.value = msg;
  console.log('errorMessage.value after set:', errorMessage.value);

  const index = streamAssistantIndex.value;
  if (
    activeSessionId.value === streamSessionId.value &&
    index >= 0 &&
    index < messages.value.length &&
    messages.value[index]?.role === 'assistant'
  ) {
    if (!messages.value[index].content) {
      messages.value.splice(index, 1);
    }
  }

  resetStreamingState();
  console.log('After resetStreamingState, errorMessage.value:', errorMessage.value);
  // 注意：不要立即调用 refreshSessions，因为它可能会清除 errorMessage
  // 让错误消息保持显示，用户可以手动刷新或重新发送
};

/**
 * 发送消息入口函数
 * 
 * 完整流程：
 * 1. 验证输入（文本或附件至少有一个）
 * 2. 如果没有活跃会话，自动创建新会话
 * 3. 在本地创建 user 和 assistant 占位消息
 * 4. 设置流式追踪状态
 * 5. 清空输入框和附件
 * 6. 滚动到底部
 * 7. 调用后端流式 API
 * 8. 如果失败，清理占位消息并显示错误
 * 
 * 设计考虑：
 * - 提前创建占位消息，让用户立即看到反馈
 * - 失败时回滚，保持 UI 一致性
 * - 支持纯文本、纯图片或图文混合发送
 */
const sendMessage = async () => {
  const text = draft.value.trim();
  if (!text && attachments.value.length === 0) return;

  errorMessage.value = '';
  sending.value = true;
  try {
    let sessionId = activeSessionId.value;
    if (!sessionId) {
      const summary = await createSession();
      if (!summary) {
        throw new Error('创建会话失败');
      }
      sessionId = summary.session_id;
      activeSessionId.value = sessionId;
    }

    const localNow = new Date().toISOString();
    messages.value.push({ role: 'user', content: text, created_at: localNow });
    messages.value.push({ role: 'assistant', content: '', created_at: localNow });
    streamSessionId.value = sessionId;
    streamAssistantIndex.value = messages.value.length - 1;

    const pendingImages = attachments.value.map((item) => item.dataUrl);
    draft.value = '';
    attachments.value = [];
    await scrollToBottom();

    // 调用后端流式 API
    // 注意：这个调用可能会立即失败（如 API key 未设置），也可能在后台异步失败
    await ChatService.SendMessageStreamInSession(
      props.scopeType,
      scopeBookPath.value,
      sessionId,
      text,
      pendingImages
    );
  } catch (err) {
    console.error('sendMessage catch error:', err);
    console.log('Error message:', err?.message);
    // 失败回滚：移除占位消息
    if (streamAssistantIndex.value >= 0 && streamAssistantIndex.value < messages.value.length) {
      messages.value.splice(streamAssistantIndex.value, 1);
      streamAssistantIndex.value = -1;
    }
    if (messages.value.length && messages.value[messages.value.length - 1]?.role === 'user' && messages.value[messages.value.length - 1]?.content === text) {
      messages.value.splice(messages.value.length - 1, 1);
    }
    resetStreamingState();
    // 确保错误消息被设置并显示
    const errorMsg = err?.message || String(err) || '发送失败';
    errorMessage.value = `发送失败：${errorMsg}`;
    console.log('errorMessage.value set to:', errorMessage.value);
    console.error('sendMessage error:', err);
  } finally {
    // 确保 sending 状态被重置
    sending.value = false;
    console.log('Finally block: errorMessage.value =', errorMessage.value);
  }
};

/**
 * ========================================
 * 9. 响应式监听器（Watchers）
 * ========================================
 * 
 * Vue 的 watch API 用于在特定状态变化时执行副作用。
 * 这里主要用于：
 * - 作用域切换时重置状态
 * - 消息变化时自动滚动
 * - 会话切换时自动滚动
 */

/**
 * 监听作用域变化
 * 
 * 触发条件：
 * - 用户从书架切换到书籍阅读
 * - 用户从书籍阅读返回书架
 * - 用户切换不同的书籍
 * 
 * 清理动作：
 * - 清除活跃会话
 * - 清空消息列表
 * - 清空输入状态
 * - 重新加载新作用域的会话列表
 * 
 * immediate: true 确保组件挂载时立即执行一次
 */
watch(
  () => scopeKey.value,
  async () => {
    activeSessionId.value = '';
    messages.value = [];
    draft.value = '';
    attachments.value = [];
    await refreshSessions();
  },
  { immediate: true }
);

/**
 * 监听消息数量变化
 * 
 * 触发条件：
 * - 新增用户消息
 * - 新增 assistant 消息
 * - 加载历史消息
 * 
 * 目的：确保新消息出现时自动滚动到底部
 */
watch(
  () => messages.value.length,
  async () => {
    await scrollToBottom();
  }
);

/**
 * 监听消息内容变化
 * 
 * 触发条件：
 * - 流式响应中每个分片的追加
 * 
 * 实现技巧：
 * - 将所有消息内容用特殊字符连接成字符串
 * - 任何内容变化都会改变这个字符串，从而触发 watch
 * 
 * 目的：流式响应时持续滚动，确保用户能看到最新的回复内容
 */
watch(
  () => messages.value.map((item) => item.content || '').join('\u0001'),
  async () => {
    await scrollToBottom();
  }
);

/**
 * 监听活跃会话变化
 * 
 * 触发条件：
 * - 用户点击会话列表项
 * - 创建新会话后自动进入
 * 
 * 目的：进入会话后立即滚动到底部，显示最新消息
 */
watch(
  () => activeSessionId.value,
  async (sessionId) => {
    if (!sessionId) return;
    await scrollToBottom();
  }
);

/**
 * 监听错误消息变化（用于调试）
 */
watch(
  () => errorMessage.value,
  (newVal, oldVal) => {
    console.log('errorMessage changed:', { old: oldVal, new: newVal });
  }
);

/**
 * ========================================
 * 10. 生命周期钩子
 * ========================================
 * 
 * 管理组件的初始化和清理逻辑。
 */

/**
 * 组件挂载时注册事件监听器
 * 
 * 注册的三个事件：
 * 1. chat_chunk: 接收流式分片
 * 2. chat_done: 接收完整回复
 * 3. chat_stream_error: 接收错误信息
 * 
 * 每个监听器都会返回一个取消订阅函数，
 * 存储在 eventUnsubscribers 数组中供后续清理。
 */
onMounted(() => {
  eventUnsubscribers.push(Events.On('chat_chunk', onStreamChunk));
  eventUnsubscribers.push(Events.On('chat_done', onStreamDone));
  eventUnsubscribers.push(Events.On('chat_stream_error', onStreamError));
});

/**
 * 组件卸载前清理事件监听器
 * 
 * 重要性：
 * - 防止内存泄漏（未清理的事件监听器会阻止 GC）
 * - 避免重复回调（组件重新挂载时会注册新的监听器）
 * - 确保组件生命周期的一致性
 * 
 * 清理策略：
 * - 遍历所有取消订阅函数并执行
 * - 从数组中移除已执行的函数
 */
onBeforeUnmount(() => {
  while (eventUnsubscribers.length) {
    const off = eventUnsubscribers.pop();
    if (typeof off === 'function') {
      off();
    }
  }
});

/**
 * 暴露给父组件的方法
 * 
 * 这些方法可以通过 ref 访问，用于外部调用
 */
defineExpose({
  /**
   * 从 data URL 添加图片到附件列表
   * @param {string} dataUrl - 图片的 data URL
   * @returns {Promise<void>}
   */
  addAttachmentFromDataUrl: async (dataUrl) => {
    try {
      // 将 data URL 转换为 File 对象
      const response = await fetch(dataUrl);
      const blob = await response.blob();
      const fileName = `screenshot-${Date.now()}.png`;
      const file = new File([blob], fileName, { type: blob.type });
      
      // 读取为 data URL 并添加到附件列表
      const item = await readFileAsDataUrl(file);
      attachments.value.push(item);
      console.log('已添加截图到附件列表');
    } catch (err) {
      console.error('添加截图失败:', err);
      errorMessage.value = `添加截图失败：${err?.message || err}`;
    }
  },
});

</script>

<style scoped>
.chat-panel {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(246, 248, 252, 0.98));
  border-left: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow: inset 1px 0 0 rgba(255, 255, 255, 0.7);
}

.chat-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 18px 14px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.chat-header-copy {
  min-width: 0;
}

.chat-kicker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 22px;
  padding: 0 9px;
  border-radius: 999px;
  background: rgba(0, 122, 204, 0.1);
  color: var(--accent-color);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.chat-title {
  margin: 8px 0 0;
  font-size: 18px;
  line-height: 1.2;
}

.chat-subtitle {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.chat-header-btn,
.chat-close {
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(255, 255, 255, 0.9);
  color: var(--text-primary);
  border-radius: 10px;
  height: 30px;
  padding: 0 10px;
  font-size: 12px;
  cursor: pointer;
}

.chat-header-btn.primary {
  background: var(--accent-color);
  color: #fff;
  border-color: transparent;
}

.chat-close {
  width: 30px;
  padding: 0;
  font-size: 20px;
  line-height: 1;
}

.chat-sessions {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
}

.chat-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.chat-section-head h4 {
  margin: 0;
  font-size: 13px;
}

.chat-section-count {
  font-size: 12px;
  color: var(--text-secondary);
}

.chat-session-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: grid;
  align-content: start;
  grid-auto-rows: max-content;
  gap: 10px;
  padding-right: 6px;
}

.chat-session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  box-sizing: border-box;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.9);
  padding: 12px;
  cursor: pointer;
  text-align: left;
  min-width: 0;
}

.chat-session-item.active {
  border-color: rgba(0, 122, 204, 0.4);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.08);
}

.chat-session-main {
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.chat-session-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-session-meta {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 11px;
  color: var(--text-secondary);
}

.chat-session-delete {
  flex: 0 0 24px;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 20px;
  line-height: 1;
  padding: 0;
}

.chat-body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
  gap: 14px;
}

.chat-messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
  user-select: text;
  -webkit-user-select: text;
}

.chat-message-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.chat-message {
  box-sizing: border-box;
  max-width: 100%;
  min-width: 0;
  width: auto;
  align-self: flex-start;
  padding: 12px 13px;
  border-radius: 16px;
  border: 1px solid rgba(0, 0, 0, 0.06);
  background: rgba(255, 255, 255, 0.88);
}

.chat-message.user {
  align-self: flex-end;
  background: linear-gradient(180deg, rgba(0, 122, 204, 0.08), rgba(0, 122, 204, 0.04));
  border-color: rgba(0, 122, 204, 0.14);
}

.chat-message.assistant {
  background: rgba(255, 255, 255, 0.96);
}

.chat-message-content {
  min-width: 0;
  max-width: 100%;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: anywhere;
  font-size: 13px;
  line-height: 1.65;
  color: var(--text-primary);
  user-select: text;
  -webkit-user-select: text;
}

.chat-message-content.markdown-body {
  min-width: 0;
  max-width: 100%;
  white-space: normal;
}

.chat-message-content.markdown-body :deep(*) {
  max-width: 100%;
}

.chat-message-content.markdown-body :deep(p) {
  margin: 0 0 0.75em;
}

.chat-message-content.markdown-body :deep(p:last-child) {
  margin-bottom: 0;
}

.chat-message-content.markdown-body :deep(ul),
.chat-message-content.markdown-body :deep(ol) {
  margin: 0.5em 0 0.75em;
  padding-left: 1.4em;
}

.chat-message-content.markdown-body :deep(li + li) {
  margin-top: 0.25em;
}

.chat-message-content.markdown-body :deep(blockquote) {
  margin: 0.5em 0;
  padding: 0.2em 0 0.2em 0.9em;
  border-left: 3px solid rgba(0, 122, 204, 0.3);
  color: var(--text-secondary);
}

.chat-message-content.markdown-body :deep(pre) {
  margin: 0.75em 0;
  width: auto;
  padding: 12px 14px;
  overflow: auto;
  max-width: 100%;
  box-sizing: border-box;
  border-radius: 12px;
  border: 1px solid #d0d7de;
  background: #f6f8fa;
  color: #24292f;
  white-space: pre;
}

.chat-message-content.markdown-body :deep(pre code) {
  display: block;
  min-width: max-content;
  padding: 0;
  background: transparent;
  color: inherit;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
  overflow-wrap: normal;
  word-break: normal;
}

.chat-message-content.markdown-body :deep(code) {
  padding: 0.12em 0.35em;
  border-radius: 6px;
  background: rgba(175, 184, 193, 0.2);
  color: #24292f;
  font-size: 0.95em;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.chat-message-content.markdown-body :deep(.hljs) {
  color: #24292f;
  background: transparent;
}

.chat-message-content.markdown-body :deep(a) {
  color: var(--accent-color);
  text-decoration: none;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.chat-message-content.markdown-body :deep(a:hover) {
  text-decoration: underline;
}

.chat-message-content.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  margin: 0.9em 0;
}

.chat-composer {
  flex-shrink: 0;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 10px;
}

.attachment-strip {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}

.paste-hint {
  margin-bottom: 8px;
  padding: 6px 10px;
  font-size: 12px;
  color: var(--text-secondary);
  background: rgba(0, 122, 204, 0.05);
  border: 1px dashed rgba(0, 122, 204, 0.2);
  border-radius: 6px;
  text-align: center;
  animation: fadeIn 0.3s ease-in;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.attachment-thumb {
  position: relative;
  aspect-ratio: 1 / 1;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: #fff;
}

.attachment-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.attachment-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.58);
  color: #fff;
  cursor: pointer;
}

.chat-input {
  width: 100%;
  box-sizing: border-box;
  resize: vertical;
  min-height: 76px;
  max-height: 140px;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  padding: 9px 10px;
  font: inherit;
  font-size: 13px;
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.96);
  outline: none;
}

.chat-input:focus {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.12);
}

.composer-actions {
  margin-top: 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.file-input {
  display: none;
}

.chat-empty {
  min-height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-secondary);
  padding: 20px;
  font-size: 13px;
  line-height: 1.6;
}

.chat-empty.compact {
  min-height: 56px;
  padding: 14px 10px;
  justify-content: flex-start;
  border: 1px dashed rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.55);
}

.chat-error {
  margin: 10px 0 0;
  padding: 8px 12px;
  font-size: 13px;
  line-height: 1.5;
  color: #b42318;
  background: rgba(180, 35, 24, 0.08);
  border: 1px solid rgba(180, 35, 24, 0.2);
  border-radius: 8px;
  word-break: break-word;
  animation: errorFadeIn 0.3s ease-in;
}

@keyframes errorFadeIn {
  from {
    opacity: 0;
    transform: translateY(-5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.chat-error.selector {
  margin-top: 12px;
}
</style>
