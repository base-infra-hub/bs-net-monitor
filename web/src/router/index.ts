import { createRouter, createWebHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Manage from '../views/Manage.vue'
import Analysis from '../views/Analysis.vue'
import Login from '../views/Login.vue'
import { authApi } from '../api'

const router = createRouter({
  history: createWebHistory('/web/'),
  routes: [
    { path: '/login', name: 'Login', component: Login, meta: { public: true } },
    { path: '/', name: 'Home', component: Home },
    { path: '/manage', name: 'Manage', component: Manage },
    { path: '/analysis', name: 'Analysis', component: Analysis },
  ],
})

router.beforeEach(async (to, from, next) => {
  if (to.meta.public) {
    next()
    return
  }

  try {
    const res = await authApi.check()
    if (res.code === 0) {
      next()
      return
    }
  } catch {
    // 鉴权失败，跳登录页
  }
  next('/login')
})

export default router
