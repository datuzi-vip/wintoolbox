import { createApp } from 'vue'
import App from './App.vue'
import './styles.css'
// Programmatic ElMessage / ElMessageBox need explicit styles (not covered by SFC auto-import).
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'

createApp(App).mount('#app')

