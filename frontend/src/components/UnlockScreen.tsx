import { useState, useEffect } from 'react'
import { useAppStore } from '../stores/appStore'
import { Lock, Eye, EyeOff, KeyRound, User } from 'lucide-react'
import { cn } from '../lib/utils'

export function UnlockScreen() {
  const { appState, storedUsername, createVault, unlock, error, clearError } = useAppStore()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)

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
      await createVault(username, password)
    } else {
      await unlock(username, password)
    }
    setLoading(false)
  }

  const passwordsMatch = !isCreating || password === confirmPassword
  const isValid = username.length >= 3 && password.length >= 6 && passwordsMatch

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
    </div>
  )
}
