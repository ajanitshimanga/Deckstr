import { useEffect, useRef } from 'react'
import { useAppStore } from './stores/appStore'
import { UnlockScreen } from './components/UnlockScreen'
import { AccountList } from './components/AccountList'
import { UpdateModal } from './components/UpdateModal'
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

  // Loading state
  if (appState === 'loading') {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--color-background)]">
        <div className="text-center">
          <div className="w-12 h-12 border-4 border-[var(--color-primary)]/30 border-t-[var(--color-primary)] rounded-full animate-spin mx-auto"></div>
          <p className="mt-4 text-[var(--color-muted-foreground)]">Loading...</p>
        </div>
      </div>
    )
  }

  // Locked or Create states - also show UpdateModal here
  if (appState === 'locked' || appState === 'create') {
    return (
      <>
        <UnlockScreen />
        <UpdateModal />
      </>
    )
  }

  // Unlocked state
  return (
    <>
      <AccountList />
      <UpdateModal />
    </>
  )
}

export default App
