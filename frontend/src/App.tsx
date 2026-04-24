import { useEffect, useRef } from 'react'
import { useAppStore } from './stores/appStore'
import { UnlockScreen } from './components/UnlockScreen'
import { AccountList } from './components/AccountList'
import { UpdateModal } from './components/UpdateModal'
import { WindowFrame } from './components/WindowFrame'
import './style.css'

function App() {
  const { appState, initialize, settings, checkForUpdates } = useAppStore()
  const hasCheckedForUpdates = useRef(false)

  useEffect(() => {
    initialize()
  }, [])

  // Check for updates on app startup (before login for safety)
  // This ensures users can update even if they forgot their password
  useEffect(() => {
    if (appState !== 'loading' && !hasCheckedForUpdates.current) {
      hasCheckedForUpdates.current = true
      // Small delay to let the UI settle first
      const timer = setTimeout(() => {
        checkForUpdates()
      }, 1500)
      return () => clearTimeout(timer)
    }
  }, [appState])

  // Periodic update check every 30 minutes
  useEffect(() => {
    if (appState === 'loading') return

    const interval = setInterval(() => {
      // Only auto-check if setting allows (when unlocked) or always on lock screen
      if (appState !== 'unlocked' || settings?.autoCheckUpdates !== false) {
        checkForUpdates()
      }
    }, 30 * 60 * 1000) // 30 minutes

    return () => clearInterval(interval)
  }, [appState, settings?.autoCheckUpdates])

  // Every screen sits under the custom window frame so chrome feels
  // continuous with the app instead of hanging off a native title bar.
  const screen = (() => {
    if (appState === 'loading') {
      return (
        <div className="flex-1 flex items-center justify-center bg-[var(--color-background)]">
          <div className="text-center">
            <div className="w-12 h-12 border-4 border-[var(--color-primary)]/30 border-t-[var(--color-primary)] rounded-full animate-spin mx-auto"></div>
            <p className="mt-4 text-[var(--color-muted-foreground)]">Loading...</p>
          </div>
        </div>
      )
    }
    if (appState === 'locked' || appState === 'create') {
      return <UnlockScreen />
    }
    return <AccountList />
  })()

  return (
    <div className="h-screen flex flex-col bg-[var(--color-background)] overflow-hidden">
      <WindowFrame />
      <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
        {screen}
      </div>
      <UpdateModal />
    </div>
  )
}

export default App
