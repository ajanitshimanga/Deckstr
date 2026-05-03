import { useEffect, useState, type ReactNode } from 'react'
import { useAppStore } from '../stores/appStore'
import {
  X,
  Shield,
  Download,
  Lock,
  HelpCircle,
  RefreshCw,
  Eye,
  EyeOff,
  ChevronRight,
  ChevronLeft,
  Zap,
  Clock,
  LogOut,
  ShieldCheck,
  FolderOpen,
  ExternalLink,
  Activity,
  Check,
  Info,
  Upload,
  AlertTriangle,
} from 'lucide-react'
import { cn } from '../lib/utils'
import { IconButton } from './ui/IconButton'
import {
  IsTelemetryEnabled,
  SetTelemetryEnabled,
  OpenUsageLogsFolder,
  OpenReleasePage,
  OpenVaultFolder,
} from '../../wailsjs/go/main/App'

// SettingsModal renders as a full-bleed page that fills the entire app body
// below the custom WindowFrame (h-9 / 36px). It always tracks the current
// app window size — there's no centered modal box to overflow or be cut off
// at min window dimensions (520×760). The single-column grouped-list layout
// mirrors iOS / Discord-style settings: scannable section headers, inline
// toggles, and drill-in rows for sub-flows that need their own form.

type View =
  | 'main'
  | 'password'
  | 'hint'
  | 'pin'
  | 'importVault'
  | 'updateSpeed'
  | 'syncInterval'
  | 'privacyDetails'

interface SettingsModalProps {
  onClose: () => void
}

const POLLING_PROFILES = [
  { id: 'instant', name: 'Instant', desc: 'Real-time updates · 1s', detail: 'Highest responsiveness, more CPU' },
  { id: 'balanced', name: 'Balanced', desc: 'Good responsiveness · 5s', detail: 'Recommended for most setups' },
  { id: 'eco', name: 'Eco', desc: 'Battery saver · 15s', detail: 'Lowest impact, slower updates' },
] as const

const SYNC_INTERVAL_PRESETS = [2, 5, 10, 15, 30] as const

export function SettingsModal({ onClose }: SettingsModalProps) {
  const [view, setView] = useState<View>('main')

  // Esc dismisses one level: drill-in → main → close. Body scroll is locked
  // while settings is open so the page underneath doesn't compete for focus.
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (view !== 'main') setView('main')
      else onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = prev
      window.removeEventListener('keydown', onKey)
    }
  }, [view, onClose])

  const headerTitle: Record<View, string> = {
    main: 'Settings',
    password: 'Change password',
    hint: 'Password hint',
    pin: 'Recovery PIN',
    importVault: 'Import vault',
    updateSpeed: 'Update speed',
    syncInterval: 'Sync interval',
    privacyDetails: 'Data & privacy',
  }

  return (
    <div
      className={cn(
        // Fill the app body below the 36px WindowFrame so the title bar
        // stays draggable and the window controls keep working from inside
        // settings.
        'fixed top-9 left-0 right-0 bottom-0 z-40',
        'bg-[var(--color-background)] flex flex-col animate-fade-in',
      )}
      role="dialog"
      aria-label="Settings"
    >
      {/* Header — paddings + IconButton mirror AccountList's header so the
          close X lands exactly where the settings gear was. Toggling
          settings on/off doesn't shift the right-edge button by even a pixel. */}
      <header className="shrink-0 flex items-center justify-between gap-2 px-4 sm:px-5 lg:px-6 py-2.5 sm:py-3 border-b border-[var(--color-border)]">
        <div className="flex items-center gap-2 min-w-0 flex-1">
          {view !== 'main' && (
            <IconButton
              ariaLabel="Back"
              tone="neutral"
              icon={<ChevronLeft className="w-4 h-4" />}
              onClick={() => setView('main')}
            />
          )}
          <h2 className="text-sm sm:text-base font-semibold text-[var(--color-foreground)] truncate">
            {headerTitle[view]}
          </h2>
        </div>
        <IconButton
          ariaLabel="Close settings"
          tone="neutral"
          icon={<X className="w-4 h-4" />}
          onClick={onClose}
        />
      </header>

      {/* Body */}
      <div key={view} className="flex-1 min-h-0 overflow-y-auto">
        {view === 'main' && <MainView setView={setView} onClose={onClose} />}
        {view === 'password' && <PasswordChangeView onBack={() => setView('main')} onClose={onClose} />}
        {view === 'hint' && <HintUpdateView onBack={() => setView('main')} />}
        {view === 'pin' && <PinRegenView onBack={() => setView('main')} />}
        {view === 'importVault' && <ImportVaultView onBack={() => setView('main')} onClose={onClose} />}
        {view === 'updateSpeed' && <UpdateSpeedView />}
        {view === 'syncInterval' && <SyncIntervalView />}
        {view === 'privacyDetails' && <PrivacyDetailsView />}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main grouped-list view
// ---------------------------------------------------------------------------

function MainView({ setView, onClose }: { setView: (v: View) => void; onClose: () => void }) {
  const {
    username,
    settings,
    hasRecoveryPhrase,
    passwordHint,
    detectAndUpdateRanks,
    isDetecting,
    lastRankSyncTime,
    appVersion,
    isCheckingForUpdates,
    checkForUpdates,
    updateSettings,
    lock,
  } = useAppStore()

  const pollingProfileId = settings?.pollingProfile || 'balanced'
  const pollingProfile = POLLING_PROFILES.find(p => p.id === pollingProfileId) || POLLING_PROFILES[1]

  const autoRankSync = settings?.autoRankSync !== false
  const syncIntervalMin = Math.round((settings?.rankSyncIntervalMs || 600000) / 60000)
  const autoCheckUpdates = settings?.autoCheckUpdates !== false

  const initials = (username || '?').slice(0, 2).toUpperCase()

  return (
    <div className="px-3 sm:px-4 py-4 space-y-5 pb-8">
      {/* Profile card */}
      <div className="flex items-center gap-3 p-3 rounded-xl bg-[var(--color-card)] border border-[var(--color-border)]">
        <div className="w-11 h-11 rounded-full bg-gradient-to-br from-[var(--color-primary)]/30 to-[var(--color-primary)]/10 border border-[var(--color-primary)]/30 flex items-center justify-center text-sm font-semibold text-[var(--color-foreground)]">
          {initials}
        </div>
        <div className="min-w-0 flex-1">
          <p className="font-semibold text-[var(--color-foreground)] truncate">{username || 'Vault'}</p>
          <p className="text-xs text-[var(--color-muted-foreground)]">Local vault · encrypted on this device</p>
        </div>
      </div>

      {/* General */}
      <Section label="General">
        <DrillRow
          icon={<Zap className="w-4 h-4" />}
          title="Update speed"
          subtitle={pollingProfile.desc}
          value={pollingProfile.name}
          onClick={() => setView('updateSpeed')}
        />
      </Section>

      {/* Live tracking */}
      <Section label="Live tracking" hint="Detect your active League account and refresh ranks">
        <ToggleRow
          icon={<Activity className="w-4 h-4" />}
          title="Auto-sync ranks"
          subtitle="Refresh ranks for the detected account"
          value={autoRankSync}
          onToggle={() => settings && updateSettings({ ...settings, autoRankSync: !autoRankSync })}
        />
        <DrillRow
          icon={<Clock className="w-4 h-4" />}
          title="Sync interval"
          subtitle="Minimum time between rank checks"
          value={`${syncIntervalMin} min`}
          onClick={() => setView('syncInterval')}
          disabled={!autoRankSync}
        />
        <ActionRow
          icon={isDetecting ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
          title={isDetecting ? 'Detecting…' : 'Detect ranks now'}
          subtitle={`Last synced ${formatLastSync(lastRankSyncTime)}`}
          onClick={() => detectAndUpdateRanks()}
          disabled={isDetecting}
          tone="brand"
        />
      </Section>

      {/* Security */}
      <Section label="Security">
        <DrillRow
          icon={<Lock className="w-4 h-4" />}
          title="Change password"
          subtitle="Update your master password"
          onClick={() => setView('password')}
        />
        <DrillRow
          icon={<HelpCircle className="w-4 h-4" />}
          title="Password hint"
          subtitle={passwordHint ? 'Shown on the lock screen' : 'Add a hint shown on the lock screen'}
          value={passwordHint ? 'Set' : undefined}
          onClick={() => setView('hint')}
        />
        <DrillRow
          icon={<Shield className="w-4 h-4" />}
          title={hasRecoveryPhrase ? 'Regenerate recovery PIN' : 'Generate recovery PIN'}
          subtitle={hasRecoveryPhrase ? 'Replace your current recovery PIN' : 'Set up password recovery'}
          value={hasRecoveryPhrase ? 'Active' : undefined}
          onClick={() => setView('pin')}
        />
        <DrillRow
          icon={<Upload className="w-4 h-4" />}
          title="Import vault from file"
          subtitle="Restore from a backup or migrate from another PC"
          onClick={() => setView('importVault')}
        />
      </Section>

      {/* Updates */}
      <Section label="App updates">
        <ToggleRow
          icon={<Download className="w-4 h-4" />}
          title="Auto-check on launch"
          subtitle="Check for updates when Deckstr starts"
          value={autoCheckUpdates}
          onToggle={() => settings && updateSettings({ ...settings, autoCheckUpdates: !autoCheckUpdates })}
        />
        <ActionRow
          icon={isCheckingForUpdates ? <RefreshCw className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
          title={isCheckingForUpdates ? 'Checking…' : 'Check for updates'}
          subtitle={`Version ${appVersion || 'dev'}`}
          onClick={() => checkForUpdates()}
          disabled={isCheckingForUpdates}
          tone="neutral"
        />
      </Section>

      {/* Privacy */}
      <PrivacyGroup onLearnMore={() => setView('privacyDetails')} />

      {/* Sign out */}
      <button
        onClick={() => {
          lock()
          onClose()
        }}
        className="w-full flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-medium text-red-400 bg-red-500/10 hover:bg-red-500/15 border border-red-500/20 transition-colors"
      >
        <LogOut className="w-4 h-4" />
        Sign out
      </button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Privacy group (toggle is hot-loaded async; "Learn more" drills into the
// disclosure page rather than crowding the main list).
// ---------------------------------------------------------------------------

function PrivacyGroup({ onLearnMore }: { onLearnMore: () => void }) {
  const [enabled, setEnabled] = useState<boolean | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    IsTelemetryEnabled().then(setEnabled).catch(() => setEnabled(false))
  }, [])

  const handleToggle = async () => {
    if (enabled === null || saving) return
    const next = !enabled
    setSaving(true)
    try {
      await SetTelemetryEnabled(next)
      setEnabled(next)
    } catch (e) {
      console.error('telemetry toggle failed', e)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Section label="Privacy">
      <ToggleRow
        icon={<ShieldCheck className="w-4 h-4" />}
        title="Anonymous usage data"
        subtitle="Help improve Deckstr — vault contents are never sent"
        value={!!enabled}
        onToggle={handleToggle}
        disabled={enabled === null || saving}
      />
      <DrillRow
        icon={<Info className="w-4 h-4" />}
        title="What's collected"
        subtitle="See exactly what gets logged and what never does"
        onClick={onLearnMore}
      />
    </Section>
  )
}

// ---------------------------------------------------------------------------
// Section + Row primitives
// ---------------------------------------------------------------------------

function Section({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <section className="space-y-1.5">
      <div className="px-1">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-muted-foreground)]">
          {label}
        </p>
        {hint && (
          <p className="text-[11px] text-[var(--color-muted-foreground)]/80 mt-0.5">{hint}</p>
        )}
      </div>
      <div className="rounded-xl bg-[var(--color-card)] border border-[var(--color-border)] divide-y divide-[var(--color-border)]/60 overflow-hidden">
        {children}
      </div>
    </section>
  )
}

function RowFrame({
  icon,
  title,
  subtitle,
  trailing,
  onClick,
  disabled,
  tone = 'neutral',
}: {
  icon: ReactNode
  title: string
  subtitle?: string
  trailing?: ReactNode
  onClick?: () => void
  disabled?: boolean
  tone?: 'neutral' | 'brand'
}) {
  const baseClass = cn(
    'w-full flex items-center gap-3 px-3 py-2.5 text-left transition-colors',
    onClick && !disabled && 'hover:bg-[var(--color-muted)]/30 active:bg-[var(--color-muted)]/40',
    disabled && 'opacity-50 cursor-not-allowed',
  )
  const inner = (
    <>
      <div
        className={cn(
          'w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0',
          tone === 'brand'
            ? 'bg-[var(--color-primary)]/15 text-[var(--color-primary)]'
            : 'bg-[var(--color-muted)]/60 text-[var(--color-muted-foreground)]',
        )}
      >
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium text-[var(--color-foreground)] truncate">{title}</p>
        {subtitle && (
          <p className="text-xs text-[var(--color-muted-foreground)] truncate">{subtitle}</p>
        )}
      </div>
      {trailing && <div className="flex-shrink-0">{trailing}</div>}
    </>
  )

  if (onClick) {
    return (
      <button type="button" onClick={onClick} disabled={disabled} className={baseClass}>
        {inner}
      </button>
    )
  }
  return <div className={baseClass}>{inner}</div>
}

function DrillRow(props: {
  icon: ReactNode
  title: string
  subtitle?: string
  value?: string
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <RowFrame
      {...props}
      trailing={
        <div className="flex items-center gap-1.5 text-[var(--color-muted-foreground)]">
          {props.value && <span className="text-xs font-medium">{props.value}</span>}
          <ChevronRight className="w-4 h-4" />
        </div>
      }
    />
  )
}

function ToggleRow({
  icon,
  title,
  subtitle,
  value,
  onToggle,
  disabled,
}: {
  icon: ReactNode
  title: string
  subtitle?: string
  value: boolean
  onToggle: () => void
  disabled?: boolean
}) {
  return (
    <RowFrame
      icon={icon}
      title={title}
      subtitle={subtitle}
      onClick={disabled ? undefined : onToggle}
      disabled={disabled}
      trailing={
        <div
          aria-hidden
          className={cn(
            'w-10 h-6 rounded-full relative transition-colors',
            value ? 'bg-green-500' : 'bg-zinc-600',
            disabled && 'opacity-60',
          )}
        >
          <div
            className={cn(
              'absolute top-1 w-4 h-4 rounded-full bg-white transition-transform',
              value ? 'translate-x-5' : 'translate-x-1',
            )}
          />
        </div>
      }
    />
  )
}

function ActionRow({
  icon,
  title,
  subtitle,
  onClick,
  disabled,
  tone = 'brand',
}: {
  icon: ReactNode
  title: string
  subtitle?: string
  onClick: () => void
  disabled?: boolean
  tone?: 'neutral' | 'brand'
}) {
  return <RowFrame icon={icon} title={title} subtitle={subtitle} onClick={onClick} disabled={disabled} tone={tone} />
}

// ---------------------------------------------------------------------------
// Drill-in: Update speed picker
// ---------------------------------------------------------------------------

function UpdateSpeedView() {
  const { settings, updateSettings, pollForActiveAccount } = useAppStore()
  const current = settings?.pollingProfile || 'balanced'

  const choose = async (id: string) => {
    if (!settings || id === current) return
    await updateSettings({ ...settings, pollingProfile: id })
    pollForActiveAccount()
  }

  return (
    <div className="px-3 sm:px-4 py-4 space-y-3">
      <p className="text-xs text-[var(--color-muted-foreground)] px-1 leading-relaxed">
        How often Deckstr checks the League client to detect which account is signed in.
        Faster polling reacts to account swaps quickly but uses slightly more CPU.
      </p>
      <div className="space-y-2">
        {POLLING_PROFILES.map(p => {
          const selected = current === p.id
          return (
            <button
              key={p.id}
              onClick={() => choose(p.id)}
              className={cn(
                'w-full flex items-start gap-3 p-3 rounded-xl border-2 text-left transition-all',
                selected
                  ? 'border-[var(--color-primary)] bg-[var(--color-primary)]/10'
                  : 'border-[var(--color-border)] bg-[var(--color-card)] hover:border-[var(--color-border)]/80',
              )}
            >
              <div
                className={cn(
                  'w-5 h-5 mt-0.5 rounded-full border-2 flex items-center justify-center flex-shrink-0',
                  selected ? 'border-[var(--color-primary)] bg-[var(--color-primary)]' : 'border-[var(--color-muted-foreground)]',
                )}
              >
                {selected && <Check className="w-3 h-3 text-white" />}
              </div>
              <div className="min-w-0">
                <p className="font-medium text-[var(--color-foreground)]">{p.name}</p>
                <p className="text-xs text-[var(--color-muted-foreground)]">{p.desc}</p>
                <p className="text-[11px] text-[var(--color-muted-foreground)]/80 mt-1">{p.detail}</p>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Drill-in: Sync interval picker
// ---------------------------------------------------------------------------

function SyncIntervalView() {
  const { settings, updateSettings } = useAppStore()
  const minutes = Math.round((settings?.rankSyncIntervalMs || 600000) / 60000)

  const setMinutes = (m: number) => {
    if (!settings) return
    updateSettings({ ...settings, rankSyncIntervalMs: m * 60000 })
  }

  return (
    <div className="px-3 sm:px-4 py-4 space-y-5">
      <p className="text-xs text-[var(--color-muted-foreground)] px-1 leading-relaxed">
        Minimum time between automatic rank refreshes for your active account.
        Shorter intervals show fresher data but make more requests to Riot.
      </p>

      <div className="rounded-xl bg-[var(--color-card)] border border-[var(--color-border)] p-4 space-y-4">
        <div className="text-center">
          <p className="text-3xl font-bold text-[var(--color-foreground)] tabular-nums">{minutes}</p>
          <p className="text-xs text-[var(--color-muted-foreground)] uppercase tracking-wider">minutes</p>
        </div>
        <input
          type="range"
          min={2}
          max={30}
          step={1}
          value={minutes}
          onChange={e => setMinutes(parseInt(e.target.value))}
          className="w-full accent-[var(--color-primary)]"
        />
        <div className="flex justify-between text-[11px] text-[var(--color-muted-foreground)]">
          <span>2 min</span>
          <span>30 min</span>
        </div>
      </div>

      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-muted-foreground)] px-1">
          Quick presets
        </p>
        <div className="grid grid-cols-5 gap-2">
          {SYNC_INTERVAL_PRESETS.map(m => (
            <button
              key={m}
              onClick={() => setMinutes(m)}
              className={cn(
                'py-2 rounded-lg text-xs font-medium transition-colors border',
                minutes === m
                  ? 'border-[var(--color-primary)] bg-[var(--color-primary)]/15 text-[var(--color-foreground)]'
                  : 'border-[var(--color-border)] bg-[var(--color-card)] text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]',
              )}
            >
              {m}m
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Drill-in: Privacy details
// ---------------------------------------------------------------------------

function PrivacyDetailsView() {
  return (
    <div className="px-3 sm:px-4 py-4 space-y-5">
      <p className="text-xs text-[var(--color-muted-foreground)] leading-relaxed px-1">
        Your vault — accounts, passwords, notes — is encrypted on your machine and is{' '}
        <span className="text-[var(--color-foreground)] font-medium">never transmitted</span>.
        Telemetry is opt-in and limited to coarse usage signals that help us iterate on
        features that actually get used.
      </p>

      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-muted-foreground)] px-1">
          Collected when on
        </p>
        <div className="rounded-xl bg-[var(--color-card)] border border-[var(--color-border)] p-3 space-y-1.5 text-xs text-[var(--color-muted-foreground)] leading-relaxed">
          <p>• App launches and cold-boot latency</p>
          <p>• Feature usage counts (account add/edit/delete, filters, wizard steps)</p>
          <p>• Error codes from the UI — never error message bodies</p>
          <p>• A salted, daily-rotating hash for anonymous DAU/MAU counts</p>
        </div>
      </div>

      <div className="space-y-2">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-muted-foreground)] px-1">
          Never collected
        </p>
        <div className="rounded-xl bg-[var(--color-card)] border border-[var(--color-border)] p-3 space-y-1.5 text-xs text-[var(--color-muted-foreground)] leading-relaxed">
          <p>• Anything inside your vault (accounts, passwords, Riot IDs, notes, tags)</p>
          <p>• IP addresses, hostnames, or device identifiers</p>
        </div>
      </div>

      <div className="rounded-xl bg-[var(--color-card)] border border-[var(--color-border)] divide-y divide-[var(--color-border)]/60 overflow-hidden">
        <RowFrame
          icon={<FolderOpen className="w-4 h-4" />}
          title="Open logs folder"
          subtitle="Inspect raw JSON event records on disk"
          onClick={() => OpenUsageLogsFolder().catch(console.error)}
          trailing={<ChevronRight className="w-4 h-4 text-[var(--color-muted-foreground)]" />}
        />
        <RowFrame
          icon={<ExternalLink className="w-4 h-4" />}
          title="Read the telemetry policy"
          subtitle="Full list of events, guarantees, and controls on GitHub"
          onClick={() =>
            OpenReleasePage('https://github.com/ajanitshimanga/Deckstr/blob/master/TELEMETRY.md').catch(console.error)
          }
          trailing={<ChevronRight className="w-4 h-4 text-[var(--color-muted-foreground)]" />}
        />
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Drill-in: Change password
// ---------------------------------------------------------------------------

function PasswordChangeView({ onBack, onClose }: { onBack: () => void; onClose: () => void }) {
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
      onBack()
      onClose() // Close settings so the recovery phrase modal can take over
    }
  }

  return (
    <form onSubmit={handleSubmit} className="px-3 sm:px-4 py-4 space-y-4">
      <FormField label="Current password">
        <PasswordInput
          value={currentPassword}
          onChange={setCurrentPassword}
          placeholder="Enter current password"
          show={showPasswords}
          onToggleShow={() => setShowPasswords(!showPasswords)}
          autoFocus
        />
      </FormField>

      <FormField label="New password">
        <PasswordInput
          value={newPassword}
          onChange={setNewPassword}
          placeholder="Enter new password"
          show={showPasswords}
          onToggleShow={() => setShowPasswords(!showPasswords)}
        />
        {newPassword.length > 0 && newPassword.length < 6 && (
          <p className="text-xs text-amber-400">Password must be at least 6 characters</p>
        )}
      </FormField>

      <FormField label="Confirm password">
        <PasswordInput
          value={confirmPassword}
          onChange={setConfirmPassword}
          placeholder="Confirm new password"
          show={showPasswords}
          onToggleShow={() => setShowPasswords(!showPasswords)}
          invalid={!!newPassword && !!confirmPassword && !passwordsMatch}
        />
        {newPassword && confirmPassword && !passwordsMatch && (
          <p className="text-xs text-red-400">Passwords do not match</p>
        )}
      </FormField>

      {error && <ErrorBanner message={error} />}

      <SubmitButton loading={loading} disabled={!isValid}>
        {loading ? 'Changing…' : 'Change password'}
      </SubmitButton>
    </form>
  )
}

// ---------------------------------------------------------------------------
// Drill-in: Password hint
// ---------------------------------------------------------------------------

function HintUpdateView({ onBack }: { onBack: () => void }) {
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
      setTimeout(onBack, 1200)
    }
  }

  if (success) {
    return (
      <div className="px-4 py-12 text-center">
        <div className="w-12 h-12 mx-auto mb-3 rounded-full bg-green-500/20 flex items-center justify-center">
          <Check className="w-6 h-6 text-green-400" />
        </div>
        <p className="font-medium text-[var(--color-foreground)]">Hint updated</p>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="px-3 sm:px-4 py-4 space-y-4">
      <FormField label="Password hint" hint="Shown on the lock screen. Leave empty to remove.">
        <input
          type="text"
          value={hint}
          onChange={e => setHint(e.target.value)}
          placeholder="e.g., My favorite pet's name"
          className={cn(
            'w-full px-3 py-2 rounded-lg text-sm',
            'bg-[var(--color-muted)] border border-[var(--color-border)]',
            'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
            'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]',
          )}
          autoFocus
        />
      </FormField>

      {error && <ErrorBanner message={error} />}

      <SubmitButton loading={loading}>
        {loading ? 'Saving…' : hint ? 'Save hint' : 'Remove hint'}
      </SubmitButton>
    </form>
  )
}

// ---------------------------------------------------------------------------
// Drill-in: Import vault from file
// ---------------------------------------------------------------------------
//
// Catch-all recovery for any scenario the in-banner adoption can't reach:
// backup files, vaults copied off another machine, files at non-standard
// paths. Picking a file signs the user out, archives the existing vault to
// vault.osm.replaced-<unix>, and routes them to the unlock screen with the
// imported vault's username pre-filled.

function ImportVaultView({ onBack, onClose }: { onBack: () => void; onClose: () => void }) {
  const { importVaultFromFile, error, clearError } = useAppStore()
  const [loading, setLoading] = useState(false)

  const handlePick = async () => {
    clearError()
    setLoading(true)
    try {
      const result = await importVaultFromFile()
      if (result.ok) {
        // Re-init has already happened in the store; close settings so the
        // unlock screen surfaces with the imported vault's username
        // pre-filled.
        onClose()
      }
      // result.cancelled → user closed the file picker; stay on this view.
      // !result.ok && !result.cancelled → real error; the store already set
      //   `error`, surfaced by the alert below.
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="px-3 sm:px-4 py-4 space-y-4">
      <button
        onClick={onBack}
        className="flex items-center gap-1.5 text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] transition-colors"
      >
        <ChevronLeft className="w-4 h-4" />
        Back
      </button>

      <div className="space-y-3">
        <h2 className="text-base font-semibold text-[var(--color-foreground)]">
          Import vault from file
        </h2>
        <p className="text-sm text-[var(--color-muted-foreground)] leading-relaxed">
          Restore from a backup, migrate from another PC, or recover an
          orphaned vault. Pick a <code className="px-1 py-0.5 rounded bg-[var(--color-card)] text-xs">.osm</code> file
          and we'll switch to it.
        </p>
      </div>

      <div className="p-3 rounded-lg bg-[var(--color-warning)]/10 border border-[var(--color-warning)]/20">
        <div className="flex items-start gap-2">
          <AlertTriangle className="w-4 h-4 text-[var(--color-warning)] flex-shrink-0 mt-0.5" />
          <div className="space-y-1.5 text-xs text-[var(--color-foreground)]">
            <p className="font-medium">What happens next</p>
            <ul className="space-y-1 text-[var(--color-muted-foreground)] list-disc pl-4">
              <li>You'll be signed out.</li>
              <li>
                Your current vault is archived as{' '}
                <code className="px-1 py-0.5 rounded bg-[var(--color-card)] text-[10px]">vault.osm.replaced-&lt;timestamp&gt;</code>{' '}
                — it's not deleted.
              </li>
              <li>The unlock screen pre-fills the imported vault's username.</li>
              <li>You'll need that vault's password and recovery PIN.</li>
            </ul>
          </div>
        </div>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-[var(--color-destructive)]/10 border border-[var(--color-destructive)]/20">
          <p className="text-xs text-[var(--color-destructive)]">{error}</p>
        </div>
      )}

      <button
        type="button"
        onClick={handlePick}
        disabled={loading}
        className={cn(
          'w-full flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-medium transition-colors',
          loading
            ? 'bg-[var(--color-muted)] text-[var(--color-muted-foreground)] cursor-not-allowed'
            : 'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white cursor-pointer'
        )}
      >
        <Upload className="w-4 h-4" />
        {loading ? 'Importing…' : 'Pick vault file…'}
      </button>

      <button
        type="button"
        onClick={() => { void OpenVaultFolder() }}
        className="w-full flex items-center justify-center gap-2 py-2 text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] transition-colors"
      >
        <FolderOpen className="w-4 h-4" />
        Show my vault folder
      </button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Drill-in: Recovery PIN
// ---------------------------------------------------------------------------

function PinRegenView({ onBack }: { onBack: () => void }) {
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

    if (result) onBack()
  }

  return (
    <form onSubmit={handleSubmit} className="px-3 sm:px-4 py-4 space-y-4">
      <div className="flex items-start gap-3 p-3 rounded-xl bg-amber-500/10 border border-amber-500/20">
        <Shield className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" />
        <div className="text-sm">
          <p className="font-medium text-[var(--color-foreground)]">Security notice</p>
          <p className="text-[var(--color-muted-foreground)] mt-1 text-xs leading-relaxed">
            {hasRecoveryPhrase
              ? 'Generating a new PIN invalidates your current one. Save the new PIN somewhere safe.'
              : 'Your recovery PIN can reset your password if forgotten. Save it somewhere safe.'}
          </p>
        </div>
      </div>

      <FormField label="Current password">
        <PasswordInput
          value={password}
          onChange={setPassword}
          placeholder="Enter your password"
          show={showPassword}
          onToggleShow={() => setShowPassword(!showPassword)}
          autoFocus
        />
      </FormField>

      {error && <ErrorBanner message={error} />}

      <SubmitButton loading={loading} disabled={password.length < 6}>
        {loading ? 'Verifying…' : hasRecoveryPhrase ? 'Regenerate PIN' : 'Generate PIN'}
      </SubmitButton>
    </form>
  )
}

// ---------------------------------------------------------------------------
// Form primitives
// ---------------------------------------------------------------------------

function FormField({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label className="text-xs font-medium text-[var(--color-foreground)]">{label}</label>
      {children}
      {hint && <p className="text-xs text-[var(--color-muted-foreground)]">{hint}</p>}
    </div>
  )
}

function PasswordInput({
  value,
  onChange,
  placeholder,
  show,
  onToggleShow,
  invalid,
  autoFocus,
}: {
  value: string
  onChange: (v: string) => void
  placeholder: string
  show: boolean
  onToggleShow: () => void
  invalid?: boolean
  autoFocus?: boolean
}) {
  return (
    <div className="relative">
      <input
        type={show ? 'text' : 'password'}
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        autoFocus={autoFocus}
        className={cn(
          'w-full px-3 py-2 pr-10 rounded-lg text-sm',
          'bg-[var(--color-muted)] border',
          invalid ? 'border-red-500' : 'border-[var(--color-border)]',
          'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
          'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]',
        )}
      />
      <button
        type="button"
        onClick={onToggleShow}
        className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]"
      >
        {show ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
      </button>
    </div>
  )
}

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/20">
      <p className="text-xs text-red-400">{message}</p>
    </div>
  )
}

function SubmitButton({
  loading,
  disabled,
  children,
}: {
  loading: boolean
  disabled?: boolean
  children: ReactNode
}) {
  return (
    <button
      type="submit"
      disabled={disabled || loading}
      className={cn(
        'w-full py-2.5 rounded-lg font-medium text-sm transition-colors',
        'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white',
        'disabled:opacity-50 disabled:cursor-not-allowed',
      )}
    >
      {children}
    </button>
  )
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatLastSync(timestamp: number | null): string {
  if (!timestamp) return 'never'
  const diff = Date.now() - timestamp
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return 'over a day ago'
}

