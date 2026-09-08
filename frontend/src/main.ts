import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { setUnauthorizedHandler } from './api/http'
import { useAuthStore } from './stores/auth'
import './styles/global.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

setUnauthorizedHandler(() => {
  const auth = useAuthStore()
  if (!auth.signedIn) return
  auth.clear()
  if (router.currentRoute.value.name !== 'login') {
    router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
  }
})

app.mount('#app')
