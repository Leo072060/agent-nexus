import React from 'react'
import ReactDOM from 'react-dom/client'
import './styles.css'

const root = ReactDOM.createRoot(document.getElementById('root')!)

async function render() {
  const Component = window.location.pathname === '/deploy' ? (await import('./DeployPage')).default : (await import('./App')).default
  root.render(
    <React.StrictMode>
      <Component />
    </React.StrictMode>,
  )
}

void render()
