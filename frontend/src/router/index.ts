import { createRouter, createWebHistory } from 'vue-router'
import AppShell from '../layouts/AppShell.vue'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: AppShell,
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
        { path: 'studio', name: 'studio', component: () => import('../views/StudioView.vue') },
        { path: 'queue', name: 'queue', component: () => import('../views/QueueView.vue') },
        { path: 'gallery', name: 'gallery', component: () => import('../views/GalleryView.vue') },
        { path: 'nodes', name: 'nodes', component: () => import('../views/NodesView.vue') },
        { path: 'users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { admin: true } },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) await auth.hydrate()
  if (to.meta.public) {
    if (auth.signedIn && to.name === 'login') return { path: '/' }
    return true
  }
  if (!auth.signedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.admin && !auth.isAdmin) {
    return { path: '/' }
  }
  return true
})

export default router
