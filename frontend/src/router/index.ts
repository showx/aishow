import { createRouter, createWebHistory } from 'vue-router'
import AppShell from '../layouts/AppShell.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: AppShell,
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
        { path: 'studio', name: 'studio', component: () => import('../views/StudioView.vue') },
        { path: 'queue', name: 'queue', component: () => import('../views/QueueView.vue') },
        { path: 'gallery', name: 'gallery', component: () => import('../views/GalleryView.vue') },
        { path: 'nodes', name: 'nodes', component: () => import('../views/NodesView.vue') },
      ],
    },
  ],
})
