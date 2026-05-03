import { useState, useEffect, useRef, ClipboardEvent, KeyboardEvent } from 'react'
import { useAppStore } from '../stores/appStore'
import { Lock, Eye, EyeOff, KeyRound, User, HelpCircle, ArrowLeft, AlertTriangle, ArrowRightLeft, FolderOpen, CheckCircle2 } from 'lucide-react'
import { cn } from '../lib/utils'
import { RecoveryPhraseModal } from './RecoveryPhraseModal'
import type { storage } from '../../wailsjs/go/models'
import { OpenVaultFolder } from '../../wailsjs/go/main/App'

// LegacyVaultBanner surfaces an orphaned vault.osm in the pre-rebrand
// OpenSmurfManager directory so the user can adopt it without manually
// shuffling files. Reused on the unlock screen, the create screen, and
// inside the recovery flow's "no recovery phrase" branch.
// AdoptedConfirmationBanner is the green follow-up that appears after a
// successful AdoptLegacyVault. Stays put until the user actually signs in
// (the store clears recentlyAdopted on first successful unlock). Without
// this banner the only cue that adoption committed is the *absence* of
// the blue legacy banner — too subtle.
function AdoptedConfirmationBanner() {
  return (
    <div
      data-testid="adopted-confirmation-banner"
      className="mb-4 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30"
    >
      <div className="flex items-start gap-2.5">
        <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0 mt-0.5" />
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-[var(--color-foreground)]">
            Migrated to Deckstr
          </p>
          <p className="text-xs text-[var(--color-muted-foreground)] mt-0.5">
            OpenSmurfManager has been removed. Sign in below with the same username and password you used before.
          </p>
        </div>
      </div>
    </div>
  )
}

function LegacyVaultBanner({
  info,
  isCreating,
  onAdopt,
  isAdopting,
  errorMessage,
}: {
  info: storage.LegacyVaultInfo
  isCreating: boolean
  onAdopt: () => void | Promise<unknown>
  isAdopting: boolean
  errorMessage?: string | null
}) {
  const username = info.username || '(no username)'
  // Two-step UX: compact summary by default; clicking "Use this vault"
  // expands an inline confirm panel that lists the consequences before
  // the destructive action runs. Avoids surprise from a misclick on what
  // looks like a benign primary action.
  const [confirming, setConfirming] = useState(false)
  const subtitle = isCreating
    ? 'Use it instead of starting over'
    : 'Sign in with your old credentials'
  return (
    <div
      data-testid="legacy-vault-banner"
      className="mb-4 p-3 rounded-lg bg-[var(--color-primary)]/10 border border-[var(--color-primary)]/30"
    >
      <div className="flex items-center gap-2.5">
        <ArrowRightLeft className="w-4 h-4 text-[var(--color-primary)] flex-shrink-0" />
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-[var(--color-foreground)] truncate">
            Existing vault: <span className="text-[var(--color-primary)]">{username}</span>
          </p>
          <p className="text-xs text-[var(--color-muted-foreground)] mt-0.5 truncate">
            From OpenSmurfManager · {subtitle}
          </p>
        </div>
        {!confirming && (
          <button
            type="button"
            onClick={() => setConfirming(true)}
            className="flex-shrink-0 px-2.5 py-1.5 rounded-md text-xs font-medium bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white cursor-pointer transition-colors"
          >
            Use this vault
          </button>
        )}
      </div>

      {confirming && (
        <div className="mt-3 pt-3 border-t border-[var(--color-primary)]/20 space-y-2">
          <p className="text-xs font-medium text-[var(--color-foreground)]">
            When you switch:
          </p>
          <ul className="text-xs text-[var(--color-muted-foreground)] space-y-1 list-disc pl-4">
            <li>Sign in with the username and password from OpenSmurfManager.</li>
            <li>
              Your current vault is archived as{' '}
              <code className="px-1 py-0.5 rounded bg-[var(--color-card)] text-[10px]">vault.osm.replaced-&lt;timestamp&gt;</code>
              {' '}— not deleted.
            </li>
            <li>The OpenSmurfManager folder is removed.</li>
          </ul>
          <div className="flex gap-2 pt-1">
            <button
              type="button"
              onClick={() => { void onAdopt() }}
              disabled={isAdopting}
              className={cn(
                'flex-1 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors',
                isAdopting
                  ? 'bg-[var(--color-muted)] text-[var(--color-muted-foreground)] cursor-not-allowed'
                  : 'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white cursor-pointer'
              )}
            >
              {isAdopting ? 'Switching…' : 'Confirm switch'}
            </button>
            <button
              type="button"
              onClick={() => setConfirming(false)}
              disabled={isAdopting}
              className="flex-1 px-2.5 py-1.5 rounded-md text-xs font-medium bg-transparent border border-[var(--color-border)] text-[var(--color-foreground)] hover:bg-[var(--color-card)] cursor-pointer transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {errorMessage && (
        <div className="mt-2.5 p-2 rounded-md bg-[var(--color-destructive)]/10 border border-[var(--color-destructive)]/20">
          <p className="text-xs text-[var(--color-destructive)]">{errorMessage}</p>
          {/* Escape hatch: when in-app adoption fails (permissions, file
              lock, etc.) the user shouldn't be stuck — let them open
              the vault folder so they can copy the legacy file by hand. */}
          <button
            type="button"
            onClick={() => { void OpenVaultFolder() }}
            className="mt-1.5 inline-flex items-center gap-1.5 text-xs font-medium text-[var(--color-foreground)] hover:underline"
          >
            <FolderOpen className="w-3.5 h-3.5" />
            Show me my vault folder
          </button>
        </div>
      )}
    </div>
  )
}

export function UnlockScreen() {
  const {
    appState,
    storedUsername,
    passwordHint,
    hasRecoveryPhrase,
    legacyVault,
    isAdoptingLegacy,
    recentlyAdopted,
    adoptLegacyVault,
    createVault,
    unlock,
    resetPasswordWithRecoveryPhrase,
    error,
    clearError,
  } = useAppStore()

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [hint, setHint] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [showHint, setShowHint] = useState(false)
  const [loading, setLoading] = useState(false)
  const [failedAttempts, setFailedAttempts] = useState(0)

  // Recovery mode state
  const [isRecoveryMode, setIsRecoveryMode] = useState(false)
  const [recoveryWords, setRecoveryWords] = useState<string[]>(['', '', '', '', '', ''])
  const [newPassword, setNewPassword] = useState('')
  const [newConfirmPassword, setNewConfirmPassword] = useState('')
  const [newHint, setNewHint] = useState('')
  const wordInputRefs = useRef<(HTMLInputElement | null)[]>([])

  const isCreating = appState === 'create'

  // Pre-fill username for returning users
  useEffect(() => {
    if (storedUsername && !isCreating) {
      setUsername(storedUsername)
    }
  }, [storedUsername, isCreating])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    clearError()

    if (isCreating && password !== confirmPassword) {
      return
    }

    if (username.length < 3 || password.length < 6) {
      return
    }

    setLoading(true)
    if (isCreating) {
      await createVault(username, password, hint || undefined)
    } else {
      const success = await unlock(username, password)
      if (!success) {
        setFailedAttempts(prev => prev + 1)
      }
    }
    setLoading(false)
  }

  const handleRecoverySubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    clearError()

    if (newPassword !== newConfirmPassword) {
      return
    }

    const filledWords = recoveryWords.filter(w => w.length > 0)
    if (filledWords.length !== 6 || newPassword.length < 6) {
      return
    }

    setLoading(true)
    // Join only filled words to avoid extra spaces
    const recoveryPhrase = filledWords.join(' ')
    const success = await resetPasswordWithRecoveryPhrase(recoveryPhrase, newPassword, newHint)
    setLoading(false)

    if (success) {
      // Reset recovery mode state
      setIsRecoveryMode(false)
      setRecoveryWords(['', '', '', '', '', ''])
      setNewPassword('')
      setNewConfirmPassword('')
      setNewHint('')
      setFailedAttempts(0)
    }
  }

  const exitRecoveryMode = () => {
    clearError()
    setIsRecoveryMode(false)
    setRecoveryWords(['', '', '', '', '', ''])
    setNewPassword('')
    setNewConfirmPassword('')
    setNewHint('')
  }

  // Handle paste in recovery word inputs - supports pasting full phrase
  const handleWordPaste = (e: ClipboardEvent<HTMLInputElement>, index: number) => {
    const pastedText = e.clipboardData.getData('text').trim()
    const words = pastedText.split(/\s+/).filter(w => w.length > 0)

    // If pasting multiple words, fill the cells
    if (words.length > 1) {
      e.preventDefault()
      const newWords = [...recoveryWords]
      for (let i = 0; i < 6 && i < words.length; i++) {
        newWords[index + i] = words[i] || ''
      }
      setRecoveryWords(newWords)
      // Focus the last filled input or the next empty one
      const nextIndex = Math.min(index + words.length, 5)
      wordInputRefs.current[nextIndex]?.focus()
    }
  }

  // Handle keyboard navigation between word inputs
  const handleWordKeyDown = (e: KeyboardEvent<HTMLInputElement>, index: number) => {
    if (e.key === ' ' || e.key === 'Tab') {
      if (e.key === ' ') e.preventDefault()
      if (index < 5) {
        wordInputRefs.current[index + 1]?.focus()
      }
    } else if (e.key === 'Backspace' && recoveryWords[index] === '' && index > 0) {
      wordInputRefs.current[index - 1]?.focus()
    }
  }

  const updateRecoveryWord = (index: number, value: string) => {
    // Remove spaces from individual word input
    const cleanValue = value.replace(/\s+/g, '')
    const newWords = [...recoveryWords]
    newWords[index] = cleanValue
    setRecoveryWords(newWords)
  }

  const passwordsMatch = !isCreating || password === confirmPassword
  const isValid = username.length >= 3 && password.length >= 6 && passwordsMatch

  const newPasswordsMatch = newPassword === newConfirmPassword
  const filledRecoveryWords = recoveryWords.filter(w => w.length > 0)
  const isRecoveryValid = filledRecoveryWords.length === 6 && newPassword.length >= 6 && newPasswordsMatch

  // Recovery Mode UI
  if (isRecoveryMode) {
    // No recovery phrase on the current vault → the 6-word reset can't
    // succeed against this file. Branch the screen to either guide the
    // user to their orphaned legacy vault, or explain that no recovery is
    // available. This is the "make it not require figuring out failed
    // attempts" path the user asked for.
    if (!hasRecoveryPhrase) {
      return (
        <div className="flex-1 min-h-0 flex items-center justify-center p-4 bg-[var(--color-background)] overflow-y-auto">
          <div className="w-full max-w-md">
            <div className="text-center mb-8">
              <div className="mx-auto w-16 h-16 bg-[var(--color-warning)] rounded-2xl flex items-center justify-center mb-4">
                <KeyRound className="w-8 h-8 text-white" />
              </div>
              <h1 className="text-2xl font-bold text-[var(--color-foreground)]">
                Forgot your password?
              </h1>
            </div>

            {legacyVault ? (
              <div className="space-y-4">
                <p className="text-sm text-[var(--color-muted-foreground)]">
                  This vault doesn't have a recovery PIN saved, but we found
                  your previous OpenSmurfManager vault on this PC. Switch to
                  it to sign in with the username and password you used
                  before — your accounts and data come back with it.
                </p>
                <LegacyVaultBanner
                  info={legacyVault}
                  isCreating={false}
                  onAdopt={async () => {
                    const ok = await adoptLegacyVault()
                    if (ok) setIsRecoveryMode(false)
                  }}
                  isAdopting={isAdoptingLegacy}
                  errorMessage={error}
                />
                <button
                  type="button"
                  onClick={exitRecoveryMode}
                  className="w-full py-2 text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] flex items-center justify-center gap-2"
                >
                  <ArrowLeft className="w-4 h-4" />
                  Back to Sign In
                </button>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="p-4 rounded-lg bg-[var(--color-card)] border border-[var(--color-border)]">
                  <p className="text-sm text-[var(--color-foreground)]">
                    No recovery PIN was saved for this vault.
                  </p>
                  <p className="text-xs text-[var(--color-muted-foreground)] mt-2">
                    Recovery requires the 6-word PIN that was generated when
                    the vault was created. If you have a backup vault file
                    on this PC, you can import it from{' '}
                    <span className="font-medium text-[var(--color-foreground)]">Settings → Security → Import vault from file</span>.
                  </p>
                  <button
                    type="button"
                    onClick={() => { void OpenVaultFolder() }}
                    className="mt-3 inline-flex items-center gap-1.5 text-xs font-medium text-[var(--color-primary)] hover:underline"
                  >
                    <FolderOpen className="w-3.5 h-3.5" />
                    Show me my vault folder
                  </button>
                </div>
                <button
                  type="button"
                  onClick={exitRecoveryMode}
                  className="w-full py-2 text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] flex items-center justify-center gap-2"
                >
                  <ArrowLeft className="w-4 h-4" />
                  Back to Sign In
                </button>
              </div>
            )}
          </div>
          <RecoveryPhraseModal />
        </div>
      )
    }

    return (
      <div className="flex-1 min-h-0 flex items-center justify-center p-4 bg-[var(--color-background)] overflow-y-auto">
        <div className="w-full max-w-md">
          {/* Header */}
          <div className="text-center mb-8">
            <div className="mx-auto w-16 h-16 bg-[var(--color-warning)] rounded-2xl flex items-center justify-center mb-4">
              <KeyRound className="w-8 h-8 text-white" />
            </div>
            <h1 className="text-2xl font-bold text-[var(--color-foreground)]">
              Reset Password
            </h1>
            <p className="text-[var(--color-muted-foreground)] mt-3">
              Enter your 6-word recovery phrase to reset your password
            </p>
          </div>

          {/* Recovery Form */}
          <form onSubmit={handleRecoverySubmit} className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium text-[var(--color-foreground)]">
                Recovery PIN
              </label>
              <p className="text-xs text-[var(--color-muted-foreground)] mb-2">
                Paste your full PIN or type each word
              </p>
              <div className="grid grid-cols-3 gap-2">
                {recoveryWords.map((word, index) => (
                  <div key={index} className="relative">
                    <span className="absolute left-2 top-1/2 -translate-y-1/2 text-xs text-[var(--color-muted-foreground)]">
                      {index + 1}.
                    </span>
                    <input
                      ref={(el) => { wordInputRefs.current[index] = el }}
                      type="text"
                      value={word}
                      onChange={(e) => updateRecoveryWord(index, e.target.value)}
                      onPaste={(e) => handleWordPaste(e, index)}
                      onKeyDown={(e) => handleWordKeyDown(e, index)}
                      placeholder="word"
                      autoFocus={index === 0}
                      className={cn(
                        'w-full pl-7 pr-2 py-2 rounded-lg text-sm font-mono',
                        'bg-[var(--color-card)] border border-[var(--color-border)]',
                        'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                        'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent'
                      )}
                    />
                  </div>
                ))}
              </div>
              <p className="text-xs text-[var(--color-muted-foreground)]">
                Words: {filledRecoveryWords.length} / 6
              </p>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-[var(--color-foreground)]">
                New Password
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-[var(--color-muted-foreground)]" />
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Enter new password"
                  className={cn(
                    'w-full pl-10 pr-12 py-3 rounded-lg',
                    'bg-[var(--color-card)] border border-[var(--color-border)]',
                    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent'
                  )}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]"
                >
                  {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
                </button>
              </div>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-[var(--color-foreground)]">
                Confirm New Password
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-[var(--color-muted-foreground)]" />
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={newConfirmPassword}
                  onChange={(e) => setNewConfirmPassword(e.target.value)}
                  placeholder="Confirm new password"
                  className={cn(
                    'w-full pl-10 pr-4 py-3 rounded-lg',
                    'bg-[var(--color-card)] border',
                    newPassword && newConfirmPassword && !newPasswordsMatch
                      ? 'border-[var(--color-destructive)]'
                      : 'border-[var(--color-border)]',
                    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent'
                  )}
                />
              </div>
              {newPassword && newConfirmPassword && !newPasswordsMatch && (
                <p className="text-sm text-[var(--color-destructive)]">
                  Passwords do not match
                </p>
              )}
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-[var(--color-foreground)]">
                New Password Hint <span className="text-[var(--color-muted-foreground)] font-normal">(optional)</span>
              </label>
              <div className="relative">
                <HelpCircle className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-[var(--color-muted-foreground)]" />
                <input
                  type="text"
                  value={newHint}
                  onChange={(e) => setNewHint(e.target.value)}
                  placeholder="e.g., My favorite pet's name"
                  className={cn(
                    'w-full pl-10 pr-4 py-3 rounded-lg',
                    'bg-[var(--color-card)] border border-[var(--color-border)]',
                    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent'
                  )}
                />
              </div>
            </div>

            {error && (
              <div className="p-3 rounded-lg bg-[var(--color-destructive)]/10 border border-[var(--color-destructive)]/20">
                <p className="text-sm text-[var(--color-destructive)]">{error}</p>
              </div>
            )}

            <button
              type="submit"
              disabled={!isRecoveryValid || loading}
              className={cn(
                'w-full py-3 rounded-lg font-medium transition-all duration-200',
                'flex items-center justify-center gap-2',
                isRecoveryValid && !loading
                  ? 'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white cursor-pointer'
                  : 'bg-[var(--color-muted)] text-[var(--color-muted-foreground)] cursor-not-allowed'
              )}
            >
              {loading ? (
                <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
              ) : (
                <>
                  <KeyRound className="w-5 h-5" />
                  Reset Password
                </>
              )}
            </button>

            <button
              type="button"
              onClick={exitRecoveryMode}
              className="w-full py-2 text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] flex items-center justify-center gap-2"
            >
              <ArrowLeft className="w-4 h-4" />
              Back to Sign In
            </button>
          </form>
        </div>
        <RecoveryPhraseModal />
      </div>
    )
  }

  return (
    <div className="flex-1 min-h-0 flex items-center justify-center p-4 bg-[var(--color-background)] overflow-y-auto">
      <div className="w-full max-w-md py-4">
        {/* Header */}
        <div className="text-center mb-6">
          <div className="mx-auto w-16 h-16 bg-[var(--color-primary)] rounded-2xl flex items-center justify-center mb-4">
            <KeyRound className="w-8 h-8 text-white" />
          </div>
          <h1 className="text-2xl font-bold text-[var(--color-foreground)]">
            Deckstr
          </h1>
          <p className="text-[var(--color-primary)] text-sm font-medium mt-1">
            Centralize Your Accounts
          </p>
          <p className="text-[var(--color-muted-foreground)] mt-3">
            {isCreating
              ? 'Create your account to securely manage all your gaming credentials'
              : 'Sign in to access your accounts'}
          </p>
        </div>

        {/* Legacy vault banner — surfaces an orphaned OpenSmurfManager vault
            so the user can adopt it instead of creating/signing into the
            wrong one. Shown above the form on both create and unlock
            states. */}
        {/* Post-adoption confirmation. Shown until the user signs in
            successfully so they get explicit feedback that the migration
            committed and OpenSmurfManager is gone — without this the only
            cue is the (correct) absence of the legacy banner, which is
            easy to miss. */}
        {recentlyAdopted && !legacyVault && (
          <AdoptedConfirmationBanner />
        )}

        {legacyVault && (
          <LegacyVaultBanner
            info={legacyVault}
            isCreating={isCreating}
            onAdopt={adoptLegacyVault}
            isAdopting={isAdoptingLegacy}
            errorMessage={error}
          />
        )}

        {/* Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium text-[var(--color-foreground)]">
              Username
            </label>
            <div className="relative">
              <User className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-[var(--color-muted-foreground)]" />
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="Enter username"
                className={cn(
                  'w-full pl-10 pr-4 py-3 rounded-lg',
                  'bg-[var(--color-card)] border border-[var(--color-border)]',
                  'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                  'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent',
                  'transition-all duration-200'
                )}
                autoFocus={!storedUsername}
              />
            </div>
            {username.length > 0 && username.length < 3 && (
              <p className="text-sm text-[var(--color-warning)]">
                Username must be at least 3 characters
              </p>
            )}
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium text-[var(--color-foreground)]">
              Password
            </label>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-[var(--color-muted-foreground)]" />
              <input
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter password"
                className={cn(
                  'w-full pl-10 pr-12 py-3 rounded-lg',
                  'bg-[var(--color-card)] border border-[var(--color-border)]',
                  'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                  'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent',
                  'transition-all duration-200'
                )}
                autoFocus={!!storedUsername}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]"
              >
                {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
              </button>
            </div>
          </div>

          {isCreating && (
            <>
              <div className="space-y-2">
                <label className="text-sm font-medium text-[var(--color-foreground)]">
                  Confirm Password
                </label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-[var(--color-muted-foreground)]" />
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Confirm password"
                    className={cn(
                      'w-full pl-10 pr-4 py-3 rounded-lg',
                      'bg-[var(--color-card)] border',
                      password && confirmPassword && !passwordsMatch
                        ? 'border-[var(--color-destructive)]'
                        : 'border-[var(--color-border)]',
                      'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                      'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent',
                      'transition-all duration-200'
                    )}
                  />
                </div>
                {password && confirmPassword && !passwordsMatch && (
                  <p className="text-sm text-[var(--color-destructive)]">
                    Passwords do not match
                  </p>
                )}
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-[var(--color-foreground)]">
                  Password Hint <span className="text-[var(--color-muted-foreground)] font-normal">(optional)</span>
                </label>
                <div className="relative">
                  <HelpCircle className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-[var(--color-muted-foreground)]" />
                  <input
                    type="text"
                    value={hint}
                    onChange={(e) => setHint(e.target.value)}
                    placeholder="e.g., My favorite pet's name"
                    className={cn(
                      'w-full pl-10 pr-4 py-3 rounded-lg',
                      'bg-[var(--color-card)] border border-[var(--color-border)]',
                      'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                      'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent',
                      'transition-all duration-200'
                    )}
                  />
                </div>
                <p className="text-xs text-[var(--color-muted-foreground)]">
                  This hint will be shown on the login screen to help you remember your password
                </p>
              </div>
            </>
          )}

          {/* Show password hint and forgot password for returning users */}
          {!isCreating && (
            <div className="space-y-2">
              {passwordHint && (
                <>
                  <button
                    type="button"
                    onClick={() => setShowHint(!showHint)}
                    className="flex items-center gap-2 text-sm text-[var(--color-primary)] hover:text-[var(--color-primary)]/80 transition-colors"
                  >
                    <HelpCircle className="w-4 h-4" />
                    {showHint ? 'Hide password hint' : 'Show password hint'}
                  </button>
                  {showHint && (
                    <div className="p-3 rounded-lg bg-[var(--color-primary)]/10 border border-[var(--color-primary)]/20">
                      <p className="text-sm text-[var(--color-foreground)]">
                        <span className="font-medium">Hint:</span> {passwordHint}
                      </p>
                    </div>
                  )}
                </>
              )}

              {/* Escalated suggestion after 3 failed attempts. The lightweight
                  link below is always visible — this card just nudges harder. */}
              {failedAttempts >= 3 && (
                <div className="p-3 rounded-lg bg-[var(--color-warning)]/10 border border-[var(--color-warning)]/20">
                  <div className="flex items-start gap-2">
                    <AlertTriangle className="w-5 h-5 text-[var(--color-warning)] flex-shrink-0 mt-0.5" />
                    <div>
                      <p className="text-sm font-medium text-[var(--color-foreground)]">
                        Multiple failed attempts
                      </p>
                      <p className="text-xs text-[var(--color-muted-foreground)] mt-1">
                        {hasRecoveryPhrase
                          ? 'Forgot your password? Use your recovery PIN to reset it.'
                          : legacyVault
                            ? 'Looks like the wrong vault. You can switch to your OpenSmurfManager vault above.'
                            : 'Forgot your password? Recovery requires the 6-word PIN you saved when creating this vault.'}
                      </p>
                      <button
                        type="button"
                        onClick={() => setIsRecoveryMode(true)}
                        className="mt-2 text-sm font-medium text-[var(--color-warning)] hover:text-[var(--color-warning)]/80 transition-colors"
                      >
                        Use Recovery PIN
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {/* Always-visible lightweight link. Recovery mode itself
                  branches on whether the vault has a phrase + whether a
                  legacy vault is detected, so it stays useful even when
                  hasRecoveryPhrase is false. */}
              {failedAttempts < 3 && (
                <button
                  type="button"
                  onClick={() => setIsRecoveryMode(true)}
                  className="flex items-center gap-2 text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] transition-colors"
                >
                  <KeyRound className="w-4 h-4" />
                  Forgot your password?
                </button>
              )}
            </div>
          )}

          {password.length > 0 && password.length < 6 && (
            <p className="text-sm text-[var(--color-warning)]">
              Password must be at least 6 characters
            </p>
          )}

          {error && (
            <div className="p-3 rounded-lg bg-[var(--color-destructive)]/10 border border-[var(--color-destructive)]/20">
              <p className="text-sm text-[var(--color-destructive)]">{error}</p>
            </div>
          )}

          <button
            type="submit"
            disabled={!isValid || loading}
            className={cn(
              'w-full py-3 rounded-lg font-medium transition-all duration-200',
              'flex items-center justify-center gap-2',
              isValid && !loading
                ? 'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90 text-white cursor-pointer'
                : 'bg-[var(--color-muted)] text-[var(--color-muted-foreground)] cursor-not-allowed'
            )}
          >
            {loading ? (
              <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
            ) : (
              <>
                <KeyRound className="w-5 h-5" />
                {isCreating ? 'Create Account' : 'Sign In'}
              </>
            )}
          </button>
        </form>

        {/* Info */}
        <div className="mt-8 text-center text-sm text-[var(--color-muted-foreground)]">
          <p>Your credentials are encrypted locally with AES-256-GCM</p>
          <p className="mt-1">Zero-knowledge encryption — only you can access your data</p>
        </div>
      </div>
      <RecoveryPhraseModal />
    </div>
  )
}
