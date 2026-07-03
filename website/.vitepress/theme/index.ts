import DefaultTheme from 'vitepress/theme'
import './style/custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    // 可在此注册全局组件
  }
}
