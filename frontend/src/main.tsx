import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { configError } from './config'
import './styles.css'

const err = configError()
const rootEl = document.getElementById('root')
if (!rootEl) throw new Error('#root element not found')

ReactDOM.createRoot(rootEl).render(
  <React.StrictMode>
    {err ? (
      <div className="setup-error">
        <h1>配置缺失</h1>
        <p>{err}</p>
        <p>
          请在 <code>frontend/.env</code> 中配置（可从 <code>.env.example</code> 复制）后刷新页面。
        </p>
      </div>
    ) : (
      <App />
    )}
  </React.StrictMode>,
)
