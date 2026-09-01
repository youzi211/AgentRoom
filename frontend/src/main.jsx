import React from 'react'
import ReactDOM from 'react-dom/client'
import { MantineProvider, createTheme } from '@mantine/core'
import '@mantine/core/styles.css'
import App from './App'
import './styles.css'

const appFontFamily =
  '"PingFang SC", "Microsoft YaHei", "Hiragino Sans GB", "Noto Sans SC", Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif'

const theme = createTheme({
  primaryColor: 'teal',
  primaryShade: 6,
  defaultRadius: 'sm',
  fontFamily: appFontFamily,
  fontSizes: {
    xs: '0.75rem',
    sm: '0.85rem',
    md: '0.9rem',
    lg: '1.1rem',
    xl: '1.25rem',
  },
  headings: {
    fontFamily: appFontFamily,
    sizes: {
      h1: { fontSize: '1.5rem', fontWeight: '800', lineHeight: '1.3' },
      h2: { fontSize: '1.25rem', fontWeight: '700', lineHeight: '1.35' },
      h3: { fontSize: '1.1rem', fontWeight: '600', lineHeight: '1.4' },
    },
  },
  colors: {
    teal: [
      '#F0FDFA', // 0
      '#CCFBF1', // 1
      '#99F6E4', // 2
      '#5EEAD4', // 3
      '#2DD4BF', // 4
      '#14B8A6', // 5
      '#0D9488', // 6 — PRIMARY
      '#0F766E', // 7 — hover
      '#115E59', // 8
      '#134E4A', // 9
    ],
  },
})

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <MantineProvider theme={theme}>
      <App />
    </MantineProvider>
  </React.StrictMode>,
)