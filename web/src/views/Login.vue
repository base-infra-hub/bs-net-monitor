<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NConfigProvider,
  NForm,
  NFormItem,
  NInput,
  NButton,
  NAlert,
  darkTheme,
} from 'naive-ui'
import { authApi } from '../api'

const router = useRouter()

const username = ref('')
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')

const handleLogin = async () => {
  if (!username.value || !password.value) {
    errorMsg.value = '请输入账号和密码'
    return
  }

  loading.value = true
  errorMsg.value = ''

  try {
    const loginRes = await authApi.login({
      username: username.value,
      password: password.value,
    })
    if (loginRes.code !== 0 || !loginRes.data?.sessionId) {
      errorMsg.value = loginRes.msg || '登录失败'
      return
    }

    // 保存 Session ID 到 localStorage，让请求拦截器可以通过 Authorization 头携带
    localStorage.setItem('web_session_id', loginRes.data.sessionId)

    // 确认 session 在后端已正确关联并生效
    const checkRes = await authApi.check()
    if (checkRes.code === 0) {
      router.push('/')
    } else {
      errorMsg.value = checkRes.msg || '登录状态确认失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.msg || '登录失败，请检查网络'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <!-- 强制使用 Naive UI 的暗黑主题配置，彻底隔离全局白天模式的样式穿透 -->
  <n-config-provider :theme="darkTheme">
    <div class="login-page">
      <!-- 极客网格背景纹理 -->
      <div class="grid-overlay"></div>

      <!-- 始终保持的高颜值深色流光光晕 -->
      <div class="glow-orb orb-indigo"></div>
      <div class="glow-orb orb-purple"></div>
      <div class="glow-orb orb-cyan"></div>
      
      <div class="login-card-container">
        <div class="login-card">
          <!-- 顶部渐变条装饰 -->
          <div class="card-accent-line"></div>

          <div class="login-header">
            <h1 class="login-title">BS NET MONITOR</h1>
            <p class="login-subtitle">网络监控后台管理系统</p>
          </div>

          <n-form @submit.prevent="handleLogin" class="login-form" :show-label="false">
            <!-- show-feedback="false" 压缩输入框间距，使其极度紧凑 -->
            <n-form-item :show-feedback="false">
              <div class="input-wrapper">
                <label class="custom-label">用户名</label>
                <n-input
                  v-model:value="username"
                  size="large"
                  placeholder="请输入用户名"
                  :input-props="{ autocomplete: 'username' }"
                  @keyup.enter="handleLogin"
                  class="custom-input"
                />
              </div>
            </n-form-item>

            <n-form-item :show-feedback="false">
              <div class="input-wrapper">
                <label class="custom-label">密码</label>
                <n-input
                  v-model:value="password"
                  size="large"
                  type="password"
                  placeholder="请输入密码"
                  show-password-on="click"
                  :input-props="{ autocomplete: 'current-password' }"
                  @keyup.enter="handleLogin"
                  class="custom-input"
                />
              </div>
            </n-form-item>

            <transition name="fade">
              <n-alert
                v-if="errorMsg"
                type="error"
                :show-icon="false"
                class="login-error"
                closable
                @close="errorMsg = ''"
              >
                {{ errorMsg }}
              </n-alert>
            </transition>

            <n-button
              type="primary"
              size="large"
              block
              :loading="loading"
              @click="handleLogin"
              class="submit-btn"
            >
              {{ loading ? '正在建立安全连接...' : '登 录' }}
            </n-button>
          </n-form>

          <div class="login-footer">
            <span>SECURE PROTOCOL • BS NET MONITOR</span>
          </div>
        </div>
      </div>
    </div>
  </n-config-provider>
</template>

<style scoped>
/* 登录页统一采用高颜值的深色/暗黑底色，作为默认首选设计 */
.login-page {
  position: relative;
  height: 100vh;
  width: 100vw;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #0a0f1d;
  color: #f8fafc;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  overflow: hidden;
}

/* 极客网格背景 */
.grid-overlay {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgba(255, 255, 255, 0.025) 1px, transparent 1px);
  background-size: 24px 24px;
  z-index: 1;
  pointer-events: none;
}

/* 青蓝微光背景球 */
.glow-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(120px);
  opacity: 0.25;
  pointer-events: none;
  z-index: 0;
}

.orb-indigo {
  width: 500px;
  height: 500px;
  background: radial-gradient(circle, rgba(6, 182, 212, 0.4) 0%, rgba(8, 145, 178, 0.25) 70%, transparent 100%);
  top: -10%;
  left: 20%;
  animation: float-orb-1 25s infinite alternate ease-in-out;
}

.orb-purple {
  width: 600px;
  height: 600px;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.4) 0%, rgba(37, 99, 235, 0.25) 70%, transparent 100%);
  bottom: -15%;
  right: 15%;
  animation: float-orb-2 30s infinite alternate ease-in-out;
}

.orb-cyan {
  width: 350px;
  height: 350px;
  background: radial-gradient(circle, rgba(34, 211, 238, 0.35) 0%, transparent 80%);
  top: 40%;
  right: 35%;
  animation: float-orb-3 20s infinite alternate ease-in-out;
}

@keyframes float-orb-1 {
  0% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(80px, 40px) scale(1.08); }
  100% { transform: translate(-30px, 90px) scale(0.95); }
}

@keyframes float-orb-2 {
  0% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(-70px, -30px) scale(1.1); }
  100% { transform: translate(40px, -70px) scale(0.9); }
}

@keyframes float-orb-3 {
  0% { transform: translate(0, 0) scale(0.9); }
  50% { transform: translate(50px, -50px) scale(1.05); }
  100% { transform: translate(-50px, 50px) scale(0.9); }
}

/* 登录卡片（毛玻璃拟态：半透 + 背景模糊 + 微光描边） */
.login-card-container {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 440px;
  padding: 20px;
}

.login-card {
  position: relative;
  background: rgba(15, 23, 42, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
  padding: 56px 40px 48px;
  box-shadow:
    0 10px 40px rgba(0, 0, 0, 0.5),
    0 0 60px rgba(8, 145, 178, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  overflow: hidden;
  transition: all 0.3s ease;
}

.login-card:hover {
  border-color: rgba(255, 255, 255, 0.16);
  box-shadow:
    0 15px 50px rgba(0, 0, 0, 0.6),
    0 0 80px rgba(8, 145, 178, 0.12),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

/* 卡片顶部渐变条 */
.card-accent-line {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: linear-gradient(90deg, #06b6d4 0%, #3b82f6 100%);
}

.login-header {
  text-align: center;
  margin-bottom: 36px;
}

.login-title {
  font-size: 26px;
  font-weight: 800;
  margin: 0 0 12px;
  letter-spacing: 0.1em;
  background: linear-gradient(90deg, #ffffff 0%, #cffafe 50%, #22d3ee 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.login-subtitle {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.45);
  margin: 0;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  font-weight: 500;
}

/* 输入框间距压缩 */
.login-form :deep(.n-form-item) {
  margin-bottom: 12px; /* 极窄间距，让输入框挨在一起 */
}

.input-wrapper {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.custom-label {
  font-size: 11px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.45);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

/* Naive UI Input 样式覆盖 */
.custom-input :deep(.n-input) {
  background-color: rgba(3, 7, 18, 0.6) !important;
  border: 1px solid rgba(255, 255, 255, 0.08) !important;
  border-radius: 12px !important;
  color: #ffffff !important;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1) !important;
  padding: 4px 0;
}

.custom-input :deep(.n-input:hover) {
  border-color: rgba(6, 182, 212, 0.4) !important;
}

.custom-input :deep(.n-input--focus) {
  border-color: rgba(6, 182, 212, 0.7) !important;
  box-shadow: 0 0 0 3px rgba(6, 182, 212, 0.15) !important;
  background-color: rgba(2, 6, 23, 0.9) !important;
}

.custom-input :deep(.n-input__placeholder) {
  color: rgba(255, 255, 255, 0.25) !important;
}

/* 登录按钮 */
.submit-btn {
  height: 48px !important;
  border-radius: 12px !important;
  font-size: 14px !important;
  font-weight: 600 !important;
  letter-spacing: 0.1em !important;
  background: linear-gradient(90deg, #06b6d4, #3b82f6) !important;
  border: none !important;
  color: #ffffff !important;
  box-shadow: 0 4px 15px rgba(6, 182, 212, 0.25) !important;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1) !important;
  margin-top: 20px; /* 与上方的密码输入框拉开合理的启动距离 */
}

.submit-btn:hover {
  transform: translateY(-1.5px);
  box-shadow: 0 6px 20px rgba(6, 182, 212, 0.4) !important;
  opacity: 0.95;
}

.submit-btn:active {
  transform: translateY(0);
}

/* 错误提示 */
.login-error {
  margin-top: 10px;
  margin-bottom: 10px;
  background-color: rgba(239, 68, 68, 0.1) !important;
  border: 1px solid rgba(239, 68, 68, 0.2) !important;
  border-radius: 12px !important;
  color: #fca5a5 !important;
}

.login-error :deep(.n-alert-body) {
  padding: 10px 14px !important;
}

.login-footer {
  margin-top: 44px;
  text-align: center;
  font-size: 10px;
  color: rgba(255, 255, 255, 0.25);
  letter-spacing: 0.08em;
  font-weight: 600;
}

/* 过渡动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
