import { useState } from 'react'
import { useAppStore } from '../stores/appStore'
import { Copy, Check, Shield, AlertTriangle } from 'lucide-react'
import { cn } from '../lib/utils'
import { Modal } from './ui/Modal'
import { Button } from './ui/Button'

export function RecoveryPhraseModal() {
  const { pendingRecoveryPhrase, showRecoveryPhraseModal, dismissRecoveryPhraseModal, requiresPinSetup } = useAppStore()
  const [showPhrase, setShowPhrase] = useState(false)
  const [isRevealing, setIsRevealing] = useState(false)
  const [copied, setCopied] = useState(false)

  if (!showRecoveryPhraseModal || !pendingRecoveryPhrase) {
    return null
  }

  const handleReveal = async () => {
    if (showPhrase || isRevealing) return
    setIsRevealing(true)
    // 1-second shimmer animation
    await new Promise(resolve => setTimeout(resolve, 1000))
    setShowPhrase(true)
    setIsRevealing(false)
  }

  const handleCopy = async () => {
    if (!showPhrase) return
    try {
      await navigator.clipboard.writeText(pendingRecoveryPhrase)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  const handleDismiss = () => {
    if (!showPhrase) return // Must reveal PIN first
    setShowPhrase(false)
    setCopied(false)
    setIsRevealing(false)
    dismissRecoveryPhraseModal()
  }

  const words = pendingRecoveryPhrase.split(' ')
  const isFirstTime = requiresPinSetup

  return (
    <Modal onClose={handleDismiss} dismissOnBackdrop={false} size="md">
      <div className="p-6 space-y-6">
        {/* Header */}
        <div className="text-center">
          <div className="mx-auto w-12 h-12 bg-[var(--color-warning)]/20 rounded-full flex items-center justify-center mb-4">
            <Shield className="w-6 h-6 text-[var(--color-warning)]" />
          </div>
          <h2 className="text-xl font-bold text-[var(--color-foreground)]">
            {isFirstTime ? 'Generate Master Recovery PIN' : 'Your New Recovery PIN'}
          </h2>
          <p className="text-sm text-[var(--color-muted-foreground)] mt-2">
            This 6-word PIN is your only way to recover your account if you forget your password.
          </p>
        </div>

        {/* Warning */}
        <div className="flex items-start gap-3 p-3 rounded-lg bg-[var(--color-warning)]/10 border border-[var(--color-warning)]/20">
          <AlertTriangle className="w-5 h-5 text-[var(--color-warning)] flex-shrink-0 mt-0.5" />
          <div className="text-sm text-[var(--color-foreground)]">
            <p className="font-medium">Important:</p>
            <ul className="list-disc list-inside mt-1 text-[var(--color-muted-foreground)]">
              <li>Write down this single-use master PIN</li>
              <li>You cannot view this PIN again after closing</li>
              <li>You can regenerate a new PIN anytime with your password</li>
            </ul>
          </div>
        </div>

        {/* Recovery PIN */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-[var(--color-foreground)]">
              Recovery PIN
            </span>
            {showPhrase && (
              <button
                onClick={handleCopy}
                className="flex items-center gap-1 text-sm text-[var(--color-primary)] hover:text-[var(--color-primary)]/80"
              >
                {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                {copied ? 'Copied!' : 'Copy'}
              </button>
            )}
          </div>

          {/* Click to reveal or show PIN */}
          {!showPhrase && !isRevealing ? (
            <button
              onClick={handleReveal}
              className="w-full p-4 rounded-lg bg-[var(--color-background)] border-2 border-dashed border-[var(--color-border)] hover:border-[var(--color-primary)] hover:bg-[var(--color-primary)]/5 transition-colors cursor-pointer"
            >
              <span className="text-[var(--color-primary)] font-medium">
                Click to Reveal PIN
              </span>
            </button>
          ) : (
            <div className="grid grid-cols-3 gap-2">
              {words.map((word, index) => (
                <div
                  key={index}
                  className={cn(
                    'flex items-center gap-2 p-2 rounded-lg border border-[var(--color-border)]',
                    isRevealing ? 'shimmer' : 'bg-[var(--color-background)]'
                  )}
                >
                  <span className="text-xs text-[var(--color-muted-foreground)] w-4">
                    {index + 1}.
                  </span>
                  <span
                    className={cn(
                      'font-mono text-sm',
                      isRevealing
                        ? 'text-transparent'
                        : 'text-[var(--color-foreground)]'
                    )}
                  >
                    {isRevealing ? '......' : word}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Continue button - only shown after reveal */}
        {showPhrase && (
          <Button fullWidth size="lg" onClick={handleDismiss}>
            I've Saved My Recovery PIN
          </Button>
        )}

        {/* Shimmer animation for reveal */}
        <style>{`
          .shimmer {
            background: linear-gradient(
              90deg,
              var(--color-muted) 25%,
              var(--color-border) 50%,
              var(--color-muted) 75%
            );
            background-size: 200% 100%;
            animation: shimmer 1s ease-in-out;
          }
          @keyframes shimmer {
            0% { background-position: 200% 0; }
            100% { background-position: -200% 0; }
          }
        `}</style>
      </div>
    </Modal>
  )
}
