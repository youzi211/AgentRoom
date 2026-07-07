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
  defaultRadius: 'sm',
  fontFamily: appFontFamily,
  headings: {
    fontFamily: appFontFamily,
    fontWeight: '800',
  },
  colors: {
    teal: [
      '#eef9f5',
      '#d9eee8',
      '#b6ded2',
      '#8bcabb',
      '#63b3a3',
      '#439e8d',
      '#2f8777',
      '#256b60',
      '#1f564e',
      '#193f3a',
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
