import { createApp } from 'vue'
import 'element-plus/dist/index.css'
import './styles/tokens.css'
import './styles/element-plus.css'
import './styles/base.css'
import App from './app/App.vue'
import { i18n } from './app/i18n'
import { router } from './app/router'

createApp(App).use(i18n).use(router).mount('#app')
