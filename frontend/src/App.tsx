import { useEffect } from 'react'
import { useAppStore } from './stores/appStore'
import { UnlockScreen } from './components/UnlockScreen'
import { AccountList } from './components/AccountList'
import { UpdateModal } from './components/UpdateModal'
import './style.css'

function App() {
  const { appState, initialize, settings, checkForUpdates } = useAppStore()

  useEffect(() => {
    initialize()
  }, [])

  // Check for updates when app is unlocked and autoCheckUpdates is enabled
  useEffect(() => {
    if (appState === 'unlocked' && settings?.autoCheckUpdates !== false) {
      // Small delay to let the UI settle first
      const timer = setTimeout(() => {
        checkForUpdates()
      }, 1000)
      return () => clearTimeout(timer)
    }
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

  // Locked or Create states
  if (appState === 'locked' || appState === 'create') {
    return <UnlockScreen />
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
