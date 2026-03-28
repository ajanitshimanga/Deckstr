import { useState, useEffect, useRef, ClipboardEvent, KeyboardEvent } from 'react'
import { useAppStore } from '../stores/appStore'
import { Lock, Eye, EyeOff, KeyRound, User, HelpCircle, ArrowLeft, AlertTriangle } from 'lucide-react'
import { cn } from '../lib/utils'
import { RecoveryPhraseModal } from './RecoveryPhraseModal'

export function UnlockScreen() {
  const {
    appState,
    storedUsername,
    passwordHint,
    hasRecoveryPhrase,
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
    return (
      <div className="min-h-screen flex items-center justify-center p-4 bg-[var(--color-background)]">
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
    <div className="min-h-screen flex items-center justify-center p-4 bg-[var(--color-background)]">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="text-center mb-8">
          <div className="mx-auto w-16 h-16 bg-[var(--color-primary)] rounded-2xl flex items-center justify-center mb-4">
            <KeyRound className="w-8 h-8 text-white" />
          </div>
          <h1 className="text-2xl font-bold text-[var(--color-foreground)]">
            OpenSmurfManager
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

              {/* Show recovery suggestion after 3 failed attempts */}
              {failedAttempts >= 3 && hasRecoveryPhrase && (
                <div className="p-3 rounded-lg bg-[var(--color-warning)]/10 border border-[var(--color-warning)]/20">
                  <div className="flex items-start gap-2">
                    <AlertTriangle className="w-5 h-5 text-[var(--color-warning)] flex-shrink-0 mt-0.5" />
                    <div>
                      <p className="text-sm font-medium text-[var(--color-foreground)]">
                        Multiple failed attempts
                      </p>
                      <p className="text-xs text-[var(--color-muted-foreground)] mt-1">
                        Forgot your password? You can use your recovery PIN to reset it.
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

              {hasRecoveryPhrase && failedAttempts < 3 && (
                <button
                  type="button"
                  onClick={() => setIsRecoveryMode(true)}
                  className="flex items-center gap-2 text-sm text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)] transition-colors"
                >
                  <KeyRound className="w-4 h-4" />
                  Forgot password? Use recovery PIN
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
