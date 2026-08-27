import { createApp } from 'vue'
import { install as VueMonacoEditorPlugin, loader } from '@guolao/vue-monaco-editor'
import './assets/index.css'
import 'vue-sonner/style.css'
import App from './App.vue'
import router from './router'

window.addEventListener('vite:preloadError', () => {
  console.log('Detected vite preload error. Reloading page...')
  window.location.reload()
})

const BASE_URL = (window as any).__BASE_URL__ || ''
const origin = window.location.origin

loader.config({
  paths: {
    vs: __MONACO_CDN__ || `${origin}${BASE_URL}/assets/vs`
  }
})

loader.init().then((monaco) => {
  const ts = monaco?.languages?.typescript
  if (ts) {
    const compilerOptions = {
      target: ts.ScriptTarget.ESNext,
      module: ts.ModuleKind.ESNext,
      moduleResolution: ts.ModuleResolutionKind.NodeJs,
      allowNonTsExtensions: true,
      allowSyntheticDefaultImports: true,
      esModuleInterop: true,
      allowJs: true,
      checkJs: false
    }
    ts.typescriptDefaults.setCompilerOptions(compilerOptions)
    ts.javascriptDefaults.setCompilerOptions(compilerOptions)
  }
}).catch(() => {})

createApp(App)
  .use(router)
  .use(VueMonacoEditorPlugin, {
    paths: {
      vs: __MONACO_CDN__ || `${origin}${BASE_URL}/assets/vs`
    }
  })
  .mount('#app')
