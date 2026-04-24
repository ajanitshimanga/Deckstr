import { useEffect, useState } from 'react'
import { useAppStore } from '../stores/appStore'
import {
  X,
  User,
  Shield,
  Trophy,
  Download,
  Lock,
  HelpCircle,
  RefreshCw,
  Eye,
  EyeOff,
  ChevronRight,
  Zap,
  Clock,
  LogOut,
} from 'lucide-react'
import { cn } from '../lib/utils'

type SettingsTab = 'general' | 'security' | 'ranked' | 'updates'

interface SettingsModalProps {
  onClose: () => void
}

export function SettingsModal({ onClose }: SettingsModalProps) {
  const { lock } = useAppStore()
  const [activeTab, setActiveTab] = useState<SettingsTab>('general')

  // Esc to dismiss + body scroll lock, matching the Modal primitive's behaviour
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = prev
      window.removeEventListener('keydown', onKey)
    }
  }, [onClose])

  const tabs: { id: SettingsTab; label: string; icon: React.ReactNode }[] = [
    { id: 'general', label: 'General', icon: <User className="w-4 h-4" /> },
    { id: 'security', label: 'Security', icon: <Shield className="w-4 h-4" /> },
    { id: 'ranked', label: 'Ranked', icon: <Trophy className="w-4 h-4" /> },
    { id: 'updates', label: 'Updates', icon: <Download className="w-4 h-4" /> },
  ]

  return (
    <div
      className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-50 animate-fade-in"
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-2xl h-[500px] bg-[var(--color-card)] rounded-xl border border-[var(--color-border)] overflow-hidden shadow-2xl shadow-black/40 flex animate-scale-in relative before:absolute before:inset-x-0 before:top-0 before:h-px before:bg-white/[0.06] before:pointer-events-none"
      >
        {/* Sidebar */}
        <div className="w-48 bg-[var(--color-background)] border-r border-[var(--color-border)] flex flex-col">
          <div className="p-4 border-b border-[var(--color-border)]">
            <h2 className="text-lg font-bold text-[var(--color-foreground)]">Settings</h2>
          </div>
          <nav className="flex-1 p-2 space-y-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={cn(
                  'w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-150 motion-reduce:transition-none active:scale-[0.98] motion-reduce:active:scale-100',
                  activeTab === tab.id
                    ? 'bg-[var(--color-primary)] text-white shadow-sm shadow-[var(--color-primary)]/25'
                    : 'text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] hover:bg-[var(--color-muted)]/50'
                )}
              >
                {tab.icon}
                {tab.label}
              </button>
            ))}
          </nav>
          {/* Sign Out button at bottom of sidebar */}
          <div className="p-2 border-t border-[var(--color-border)]">
            <button
              onClick={() => {
                lock()
                onClose()
              }}
              className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-red-400 hover:bg-red-500/10 transition-colors"
            >
              <LogOut className="w-4 h-4" />
              Sign Out
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 flex flex-col">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
            <h3 className="text-base font-semibold text-[var(--color-foreground)]">
              {tabs.find(t => t.id === activeTab)?.label}
            </h3>
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] hover:bg-[var(--color-muted)]/30 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Tab Content */}
          <div className="flex-1 overflow-y-auto p-6">
            {activeTab === 'general' && <GeneralTab />}
            {activeTab === 'security' && <SecurityTab onClose={onClose} />}
            {activeTab === 'ranked' && <RankedTab />}
            {activeTab === 'updates' && <UpdatesTab />}
          </div>
        </div>
      </div>
    </div>
  )
}

// General Tab
function GeneralTab() {
  const { username, settings, updateSettings } = useAppStore()
  const currentProfile = settings?.pollingProfile || 'balanced'

  const profiles = [
    { id: 'instant', name: 'Instant', desc: 'Real-time updates (1s), more resources' },
    { id: 'balanced', name: 'Balanced', desc: 'Good responsiveness (5s), minimal impact' },
    { id: 'eco', name: 'Eco', desc: 'Minimal resources (15s), slower updates' },
  ]

  const handleProfileChange = async (profileId: string) => {
    if (settings) {
      await updateSettings({ ...settings, pollingProfile: profileId })
      // Trigger immediate re-poll with new interval
      useAppStore.getState().pollForActiveAccount()
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <label className="text-xs font-medium text-[var(--color-muted-foreground)] uppercase tracking-wider">
          Account
        </label>
        <div className="mt-2 flex items-center gap-3 p-3 rounded-lg bg-[var(--color-muted)]/30">
          <div className="w-10 h-10 rounded-full bg-[var(--color-primary)]/20 flex items-center justify-center">
            <User className="w-5 h-5 text-[var(--color-primary)]" />
          </div>
          <div>
            <p className="font-medium text-[var(--color-foreground)]">{username}</p>
            <p className="text-xs text-[var(--color-muted-foreground)]">Local vault owner</p>
          </div>
        </div>
      </div>

      {/* Polling Profile */}
      <div className="pt-4 border-t border-[var(--color-border)]">
        <label className="text-xs font-medium text-[var(--color-muted-foreground)] uppercase tracking-wider">
          Update Speed
        </label>
        <p className="text-xs text-[var(--color-muted-foreground)] mt-1 mb-3">
          Balance between UI responsiveness and system resources
        </p>
        <div className="space-y-2">
          {profiles.map((profile) => (
            <button
              key={profile.id}
              onClick={() => handleProfileChange(profile.id)}
              className={cn(
                'w-full flex items-start gap-3 p-3 rounded-lg border-2 transition-all text-left',
                currentProfile === profile.id
                  ? 'border-[var(--color-primary)] bg-[var(--color-primary)]/10'
                  : 'border-[var(--color-border)] hover:border-[var(--color-border)]/80'
              )}
            >
              <div className={cn(
                'w-4 h-4 mt-0.5 rounded-full border-2 flex items-center justify-center flex-shrink-0',
                currentProfile === profile.id
                  ? 'border-[var(--color-primary)]'
                  : 'border-[var(--color-muted-foreground)]'
              )}>
                {currentProfile === profile.id && (
                  <div className="w-2 h-2 rounded-full bg-[var(--color-primary)]" />
                )}
              </div>
              <div>
                <p className="font-medium text-[var(--color-foreground)]">{profile.name}</p>
                <p className="text-xs text-[var(--color-muted-foreground)]">{profile.desc}</p>
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}

// Security Tab
function SecurityTab({ onClose }: { onClose: () => void }) {
  const { hasRecoveryPhrase, passwordHint } = useAppStore()
  const [showPasswordChange, setShowPasswordChange] = useState(false)
  const [showHintUpdate, setShowHintUpdate] = useState(false)
  const [showPinRegen, setShowPinRegen] = useState(false)

  if (showPasswordChange) {
    return <PasswordChangeForm onBack={() => setShowPasswordChange(false)} onClose={onClose} />
  }

  if (showHintUpdate) {
    return <HintUpdateForm onBack={() => setShowHintUpdate(false)} />
  }

  if (showPinRegen) {
    return <PinRegenForm onBack={() => setShowPinRegen(false)} />
  }

  return (
    <div className="space-y-2">
      <button
        onClick={() => setShowPasswordChange(true)}
        className="w-full flex items-center justify-between p-3 rounded-lg hover:bg-[var(--color-muted)]/30 transition-colors group"
      >
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-[var(--color-muted)]/50 flex items-center justify-center">
            <Lock className="w-4 h-4 text-[var(--color-muted-foreground)]" />
          </div>
          <div className="text-left">
            <p className="font-medium text-[var(--color-foreground)]">Change Password</p>
            <p className="text-xs text-[var(--color-muted-foreground)]">Update your master password</p>
          </div>
        </div>
        <ChevronRight className="w-4 h-4 text-[var(--color-muted-foreground)] group-hover:text-[var(--color-foreground)]" />
      </button>

      <button
        onClick={() => setShowHintUpdate(true)}
        className="w-full flex items-center justify-between p-3 rounded-lg hover:bg-[var(--color-muted)]/30 transition-colors group"
      >
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-[var(--color-muted)]/50 flex items-center justify-center">
            <HelpCircle className="w-4 h-4 text-[var(--color-muted-foreground)]" />
          </div>
          <div className="text-left">
            <p className="font-medium text-[var(--color-foreground)]">Password Hint</p>
            <p className="text-xs text-[var(--color-muted-foreground)]">
              {passwordHint ? 'Update your hint' : 'Add a password hint'}
            </p>
          </div>
        </div>
        <ChevronRight className="w-4 h-4 text-[var(--color-muted-foreground)] group-hover:text-[var(--color-foreground)]" />
      </button>

      <button
        onClick={() => setShowPinRegen(true)}
        className="w-full flex items-center justify-between p-3 rounded-lg hover:bg-[var(--color-muted)]/30 transition-colors group"
      >
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-[var(--color-muted)]/50 flex items-center justify-center">
            <Shield className="w-4 h-4 text-[var(--color-muted-foreground)]" />
          </div>
          <div className="text-left">
            <p className="font-medium text-[var(--color-foreground)]">
              {hasRecoveryPhrase ? 'Regenerate Recovery PIN' : 'Generate Recovery PIN'}
            </p>
            <p className="text-xs text-[var(--color-muted-foreground)]">
              {hasRecoveryPhrase ? 'Create a new recovery PIN' : 'Set up password recovery'}
            </p>
          </div>
        </div>
        <ChevronRight className="w-4 h-4 text-[var(--color-muted-foreground)] group-hover:text-[var(--color-foreground)]" />
      </button>
    </div>
  )
}

// Password Change Form
function PasswordChangeForm({ onBack, onClose }: { onBack: () => void; onClose: () => void }) {
  const { changePassword, clearError, error } = useAppStore()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPasswords, setShowPasswords] = useState(false)
  const [loading, setLoading] = useState(false)

  const passwordsMatch = newPassword === confirmPassword
  const isValid = currentPassword.length >= 6 && newPassword.length >= 6 && passwordsMatch

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!isValid) return

    clearError()
    setLoading(true)
    const result = await changePassword(currentPassword, newPassword)
    setLoading(false)

    if (result) {
      onClose() // Close settings, recovery phrase modal will show
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <button
        type="button"
        onClick={onBack}
        className="text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] flex items-center gap-1"
      >
        <ChevronRight className="w-4 h-4 rotate-180" />
        Back to Security
      </button>

      <div className="space-y-3">
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-[var(--color-foreground)]">Current Password</label>
          <div className="relative">
            <input
              type={showPasswords ? 'text' : 'password'}
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              placeholder="Enter current password"
              className={cn(
                'w-full px-3 py-2 pr-10 rounded-lg text-sm',
                'bg-[var(--color-muted)] border border-[var(--color-border)]',
                'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
              )}
              autoFocus
            />
            <button
              type="button"
              onClick={() => setShowPasswords(!showPasswords)}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]"
            >
              {showPasswords ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="text-xs font-medium text-[var(--color-foreground)]">New Password</label>
          <input
            type={showPasswords ? 'text' : 'password'}
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            placeholder="Enter new password"
            className={cn(
              'w-full px-3 py-2 rounded-lg text-sm',
              'bg-[var(--color-muted)] border border-[var(--color-border)]',
              'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
              'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
            )}
          />
          {newPassword.length > 0 && newPassword.length < 6 && (
            <p className="text-xs text-amber-400">Password must be at least 6 characters</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="text-xs font-medium text-[var(--color-foreground)]">Confirm Password</label>
          <input
            type={showPasswords ? 'text' : 'password'}
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            placeholder="Confirm new password"
            className={cn(
              'w-full px-3 py-2 rounded-lg text-sm',
              'bg-[var(--color-muted)] border',
              newPassword && confirmPassword && !passwordsMatch
                ? 'border-red-500'
                : 'border-[var(--color-border)]',
              'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
              'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
            )}
          />
          {newPassword && confirmPassword && !passwordsMatch && (
            <p className="text-xs text-red-400">Passwords do not match</p>
          )}
        </div>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/20">
          <p className="text-xs text-red-400">{error}</p>
        </div>
      )}

      <button
        type="submit"
        disabled={!isValid || loading}
        className={cn(
          'w-full py-2.5 rounded-lg font-medium text-sm transition-colors',
          'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white',
          'disabled:opacity-50 disabled:cursor-not-allowed'
        )}
      >
        {loading ? 'Changing...' : 'Change Password'}
      </button>
    </form>
  )
}

// Hint Update Form
function HintUpdateForm({ onBack }: { onBack: () => void }) {
  const { updatePasswordHint, passwordHint, clearError, error } = useAppStore()
  const [hint, setHint] = useState(passwordHint || '')
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    clearError()
    setLoading(true)
    const result = await updatePasswordHint(hint)
    setLoading(false)

    if (result) {
      setSuccess(true)
      setTimeout(onBack, 1500)
    }
  }

  if (success) {
    return (
      <div className="text-center py-8">
        <div className="w-12 h-12 mx-auto mb-3 rounded-full bg-green-500/20 flex items-center justify-center">
          <svg className="w-6 h-6 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
          </svg>
        </div>
        <p className="font-medium text-[var(--color-foreground)]">Hint Updated!</p>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <button
        type="button"
        onClick={onBack}
        className="text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] flex items-center gap-1"
      >
        <ChevronRight className="w-4 h-4 rotate-180" />
        Back to Security
      </button>

      <div className="space-y-1.5">
        <label className="text-xs font-medium text-[var(--color-foreground)]">Password Hint</label>
        <input
          type="text"
          value={hint}
          onChange={(e) => setHint(e.target.value)}
          placeholder="e.g., My favorite pet's name"
          className={cn(
            'w-full px-3 py-2 rounded-lg text-sm',
            'bg-[var(--color-muted)] border border-[var(--color-border)]',
            'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
            'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
          )}
          autoFocus
        />
        <p className="text-xs text-[var(--color-muted-foreground)]">
          Shown on the login screen. Leave empty to remove.
        </p>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/20">
          <p className="text-xs text-red-400">{error}</p>
        </div>
      )}

      <button
        type="submit"
        disabled={loading}
        className={cn(
          'w-full py-2.5 rounded-lg font-medium text-sm transition-colors',
          'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white',
          'disabled:opacity-50 disabled:cursor-not-allowed'
        )}
      >
        {loading ? 'Saving...' : (hint ? 'Save Hint' : 'Remove Hint')}
      </button>
    </form>
  )
}

// PIN Regeneration Form
function PinRegenForm({ onBack }: { onBack: () => void }) {
  const { regenerateRecoveryPhrase, clearError, error, hasRecoveryPhrase } = useAppStore()
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (password.length < 6) return

    clearError()
    setLoading(true)
    const result = await regenerateRecoveryPhrase(password)
    setLoading(false)

    if (result) {
      onBack() // Recovery phrase modal will show
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <button
        type="button"
        onClick={onBack}
        className="text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] flex items-center gap-1"
      >
        <ChevronRight className="w-4 h-4 rotate-180" />
        Back to Security
      </button>

      <div className="flex items-start gap-3 p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
        <Shield className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
        <div className="text-sm">
          <p className="font-medium text-[var(--color-foreground)]">Security Notice</p>
          <p className="text-[var(--color-muted-foreground)] mt-1">
            {hasRecoveryPhrase
              ? 'This will invalidate your current recovery PIN.'
              : 'Your recovery PIN can reset your password if forgotten.'}
          </p>
        </div>
      </div>

      <div className="space-y-1.5">
        <label className="text-xs font-medium text-[var(--color-foreground)]">Current Password</label>
        <div className="relative">
          <input
            type={showPassword ? 'text' : 'password'}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Enter your password"
            className={cn(
              'w-full px-3 py-2 pr-10 rounded-lg text-sm',
              'bg-[var(--color-muted)] border border-[var(--color-border)]',
              'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
              'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
            )}
            autoFocus
          />
          <button
            type="button"
            onClick={() => setShowPassword(!showPassword)}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]"
          >
            {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
          </button>
        </div>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/20">
          <p className="text-xs text-red-400">{error}</p>
        </div>
      )}

      <button
        type="submit"
        disabled={password.length < 6 || loading}
        className={cn(
          'w-full py-2.5 rounded-lg font-medium text-sm transition-colors',
          'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white',
          'disabled:opacity-50 disabled:cursor-not-allowed'
        )}
      >
        {loading ? 'Verifying...' : (hasRecoveryPhrase ? 'Regenerate PIN' : 'Generate PIN')}
      </button>
    </form>
  )
}

// Ranked Tab
function RankedTab() {
  const {
    settings,
    updateSettings,
    detectAndUpdateRanks,
    isDetecting,
    lastRankSyncTime,
  } = useAppStore()

  const autoRankSync = settings?.autoRankSync !== false
  const syncIntervalMs = settings?.rankSyncIntervalMs || 600000
  const syncIntervalMin = Math.round(syncIntervalMs / 60000)

  const handleToggleAutoSync = () => {
    if (settings) {
      updateSettings({ ...settings, autoRankSync: !autoRankSync })
    }
  }

  const handleIntervalChange = (minutes: number) => {
    if (settings) {
      updateSettings({ ...settings, rankSyncIntervalMs: minutes * 60000 })
    }
  }

  const formatLastSync = (timestamp: number | null) => {
    if (!timestamp) return 'Never'
    const diff = Date.now() - timestamp
    const minutes = Math.floor(diff / 60000)
    if (minutes < 1) return 'Just now'
    if (minutes < 60) return `${minutes}m ago`
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}h ago`
    return 'Over a day ago'
  }

  return (
    <div className="space-y-6">
      {/* Auto-sync toggle */}
      <div className="flex items-center justify-between">
        <div>
          <p className="font-medium text-[var(--color-foreground)]">Auto-sync Ranks</p>
          <p className="text-xs text-[var(--color-muted-foreground)]">
            Automatically fetch ranks when account is detected
          </p>
        </div>
        <button
          onClick={handleToggleAutoSync}
          className={cn(
            "w-11 h-6 rounded-full transition-colors relative",
            autoRankSync ? "bg-green-500" : "bg-zinc-600"
          )}
        >
          <div className={cn(
            "absolute top-1 w-4 h-4 rounded-full bg-white transition-transform",
            autoRankSync ? "translate-x-6" : "translate-x-1"
          )} />
        </button>
      </div>

      {/* Sync interval */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <p className="font-medium text-[var(--color-foreground)]">Sync Interval</p>
            <p className="text-xs text-[var(--color-muted-foreground)]">
              Minimum time between rank checks
            </p>
          </div>
          <span className="text-sm font-medium text-[var(--color-foreground)]">
            {syncIntervalMin} min
          </span>
        </div>
        <input
          type="range"
          min={2}
          max={30}
          step={1}
          value={syncIntervalMin}
          onChange={(e) => handleIntervalChange(parseInt(e.target.value))}
          className="w-full accent-[var(--color-primary)]"
        />
        <div className="flex justify-between text-xs text-[var(--color-muted-foreground)]">
          <span>2 min</span>
          <span>30 min</span>
        </div>
      </div>

      <div className="pt-4 border-t border-[var(--color-border)] space-y-4">
        {/* Last synced */}
        <div className="flex items-center gap-2 text-sm">
          <Clock className="w-4 h-4 text-[var(--color-muted-foreground)]" />
          <span className="text-[var(--color-muted-foreground)]">Last synced:</span>
          <span className="text-[var(--color-foreground)]">{formatLastSync(lastRankSyncTime)}</span>
        </div>

        {/* Manual detect button */}
        <button
          onClick={() => detectAndUpdateRanks()}
          disabled={isDetecting}
          className={cn(
            'w-full flex items-center justify-center gap-2 py-2.5 rounded-lg font-medium text-sm transition-colors',
            'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white',
            'disabled:opacity-50 disabled:cursor-not-allowed'
          )}
        >
          {isDetecting ? (
            <>
              <RefreshCw className="w-4 h-4 animate-spin" />
              Detecting...
            </>
          ) : (
            <>
              <Zap className="w-4 h-4" />
              Detect Ranks Now
            </>
          )}
        </button>
      </div>
    </div>
  )
}

// Updates Tab
function UpdatesTab() {
  const {
    settings,
    updateSettings,
    appVersion,
    isCheckingForUpdates,
    checkForUpdates,
  } = useAppStore()

  const autoCheckUpdates = settings?.autoCheckUpdates !== false

  const handleToggleAutoCheck = () => {
    if (settings) {
      updateSettings({ ...settings, autoCheckUpdates: !autoCheckUpdates })
    }
  }

  return (
    <div className="space-y-6">
      {/* Version info */}
      <div className="flex items-center justify-between p-3 rounded-lg bg-[var(--color-muted)]/30">
        <div>
          <p className="text-xs text-[var(--color-muted-foreground)]">Current Version</p>
          <p className="font-mono font-medium text-[var(--color-foreground)]">
            {appVersion || 'dev'}
          </p>
        </div>
      </div>

      {/* Auto-check toggle */}
      <div className="flex items-center justify-between">
        <div>
          <p className="font-medium text-[var(--color-foreground)]">Auto-check Updates</p>
          <p className="text-xs text-[var(--color-muted-foreground)]">
            Check for updates when app starts
          </p>
        </div>
        <button
          onClick={handleToggleAutoCheck}
          className={cn(
            "w-11 h-6 rounded-full transition-colors relative",
            autoCheckUpdates ? "bg-green-500" : "bg-zinc-600"
          )}
        >
          <div className={cn(
            "absolute top-1 w-4 h-4 rounded-full bg-white transition-transform",
            autoCheckUpdates ? "translate-x-6" : "translate-x-1"
          )} />
        </button>
      </div>

      {/* Check now button */}
      <button
        onClick={() => checkForUpdates()}
        disabled={isCheckingForUpdates}
        className={cn(
          'w-full flex items-center justify-center gap-2 py-2.5 rounded-lg font-medium text-sm transition-colors',
          'bg-[var(--color-muted)]/50 hover:bg-[var(--color-muted)] text-[var(--color-foreground)]',
          'disabled:opacity-50 disabled:cursor-not-allowed'
        )}
      >
        {isCheckingForUpdates ? (
          <>
            <RefreshCw className="w-4 h-4 animate-spin" />
            Checking...
          </>
        ) : (
          <>
            <RefreshCw className="w-4 h-4" />
            Check for Updates
          </>
        )}
      </button>
    </div>
  )
}
