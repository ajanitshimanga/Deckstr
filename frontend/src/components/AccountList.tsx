import { useState, useMemo, useEffect } from 'react'
import { useAppStore } from '../stores/appStore'
import {
  Search,
  Plus,
  Copy,
  Eye,
  EyeOff,
  Pencil,
  Trash2,
  Gamepad2,
  ChevronUp,
  ChevronDown,
  Settings,
  Check as CheckIcon,
  Volume2,
  VolumeX,
} from 'lucide-react'
import { cn } from '../lib/utils'
import { MOTION_BASE, MOTION_FOCUS } from '../lib/motion'
import { getSoundsEnabled, setSoundsEnabled, playTick } from '../lib/sound'
import { models, providers } from '../../wailsjs/go/models'
import { Button } from './ui/Button'
import { IconButton } from './ui/IconButton'
import { Card } from './ui/Card'
import { Modal, ModalBody, ModalFooter, ModalHeader } from './ui/Modal'
import { RecoveryPhraseModal } from './RecoveryPhraseModal'
import { SettingsModal } from './SettingsModal'
import { AddAccountWizard } from './AddAccountWizard'

// Per-game badge styling on account cards. Unknown game IDs fall back to the
// game's name (resolved from the gameNetworks catalog) with a neutral palette,
// so adding a new game in models.go shows up correctly without a code change
// here. The previous hard-coded ternary defaulted to "Valorant" for any
// unrecognized id, which mis-labelled Rocket League cards.
const GAME_BADGE: Record<string, { label: string; classes: string }> = {
  lol: { label: 'League', classes: 'bg-blue-500/15 text-blue-400 border border-blue-500/20' },
  tft: { label: 'TFT', classes: 'bg-purple-500/15 text-purple-400 border border-purple-500/20' },
  valorant: { label: 'Valorant', classes: 'bg-red-500/15 text-red-400 border border-red-500/20' },
  rl: { label: 'Rocket League', classes: 'bg-orange-500/15 text-orange-400 border border-orange-500/20' },
  fortnite: { label: 'Fortnite', classes: 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/20' },
}

const NEUTRAL_BADGE_CLASSES =
  'bg-[var(--color-muted)]/40 text-[var(--color-muted-foreground)] border border-[var(--color-border)]'

function gameBadge(
  gameId: string,
  networks: models.GameNetwork[],
): { label: string; classes: string } {
  const known = GAME_BADGE[gameId]
  if (known) return known
  for (const n of networks) {
    for (const g of n.games) {
      if (g.id === gameId) return { label: g.name, classes: NEUTRAL_BADGE_CLASSES }
    }
  }
  return { label: gameId, classes: NEUTRAL_BADGE_CLASSES }
}

// Rank tier ordering for sorting (higher = better)
const TIER_ORDER: Record<string, number> = {
  'CHALLENGER': 10,
  'GRANDMASTER': 9,
  'MASTER': 8,
  'DIAMOND': 7,
  'EMERALD': 6,
  'PLATINUM': 5,
  'GOLD': 4,
  'SILVER': 3,
  'BRONZE': 2,
  'IRON': 1,
  '': 0,
}

const DIVISION_ORDER: Record<string, number> = {
  'I': 4,
  'II': 3,
  'III': 2,
  'IV': 1,
  '': 0,
}

// Convert rank to numeric value for sorting
function getRankValue(rank: models.CachedRank | undefined): number {
  if (!rank) return 0
  const tierValue = (TIER_ORDER[rank.tier?.toUpperCase() || ''] || 0) * 10000
  const divisionValue = (DIVISION_ORDER[rank.division || ''] || 0) * 100
  const lpValue = rank.lp || 0
  return tierValue + divisionValue + lpValue
}

// Get primary rank for an account (Solo/Duo > Flex > TFT)
function getPrimaryRank(account: models.Account): models.CachedRank | undefined {
  if (!account.cachedRanks || account.cachedRanks.length === 0) return undefined

  // Priority: Solo/Duo > Flex > TFT Ranked > others
  const priority = ['RANKED_SOLO_5x5', 'RANKED_FLEX_SR', 'RANKED_TFT']
  for (const queueType of priority) {
    const rank = account.cachedRanks.find(r => r.queueType === queueType)
    if (rank && rank.tier) return rank
  }
  return account.cachedRanks.find(r => r.tier) || account.cachedRanks[0]
}

type SortField = 'name' | 'rank' | 'updated' | 'created'
type SortDirection = 'asc' | 'desc'

export function AccountList() {
  const {
    filteredAccounts,
    accounts: allAccounts,
    gameNetworks,
    tags,
    searchQuery,
    selectedNetworkId,
    selectedTag,
    username,
    detectedAccount,
    activeAccountId,
    showRecoveryPhraseModal,
    setSearchQuery,
    setSelectedNetwork,
    setSelectedTag,
    removeAccount,
    editAccount,
    loadAccounts,
  } = useAppStore()

  const unfilteredAccounts = filteredAccounts()
  // Surface "Custom" in the network filter only when at least one account uses it —
  // keeps the picker clean for users who haven't added custom networks yet.
  const hasCustomAccounts = useMemo(
    () => allAccounts.some(a => a.networkId === 'custom'),
    [allAccounts],
  )
  // Sound mute toggle — lives in the main header so it's always reachable,
  // not buried inside Settings. Persists via sound.ts's localStorage.
  const [soundsOn, setSoundsOn] = useState(getSoundsEnabled())
  const toggleSounds = () => {
    const next = !soundsOn
    setSoundsEnabled(next)
    setSoundsOn(next)
    if (next) playTick() // confirmation pip on enable only
  }
  const [showAddModal, setShowAddModal] = useState(false)
  // Onboarding: drop first-time / zero-account users straight into the Add
  // Account wizard so the happy-path setup is obvious without any copy
  // telling them what to do. Offer once per session — if they dismiss, they
  // land on the empty state and can add later via the Add Account button.
  const [onboardingOffered, setOnboardingOffered] = useState(false)
  useEffect(() => {
    if (onboardingOffered) return
    if (showRecoveryPhraseModal) return // PIN must be saved first
    if (allAccounts.length > 0) {
      setOnboardingOffered(true)
      return
    }
    setShowAddModal(true)
    setOnboardingOffered(true)
  }, [allAccounts.length, showRecoveryPhraseModal, onboardingOffered])
  const [editingAccount, setEditingAccount] = useState<models.Account | null>(null)
  const [visiblePasswords, setVisiblePasswords] = useState<Set<string>>(new Set())
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [sortField, setSortField] = useState<SortField>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')
  const [selectedGame, setSelectedGame] = useState<string | null>(null)
  const [showLinkModal, setShowLinkModal] = useState(false)
  const [showSettingsModal, setShowSettingsModal] = useState(false)
  const [deletingAccount, setDeletingAccount] = useState<models.Account | null>(null)

  // Filter by game then sort accounts
  const accounts = useMemo(() => {
    let filtered = unfilteredAccounts

    // Filter by selected game
    if (selectedGame) {
      filtered = filtered.filter(acc =>
        acc.games && acc.games.includes(selectedGame)
      )
    }

    return [...filtered].sort((a, b) => {
      let comparison = 0

      switch (sortField) {
        case 'name':
          const nameA = (a.displayName || a.username).toLowerCase()
          const nameB = (b.displayName || b.username).toLowerCase()
          comparison = nameA.localeCompare(nameB)
          break
        case 'rank':
          const rankA = getRankValue(getPrimaryRank(a))
          const rankB = getRankValue(getPrimaryRank(b))
          comparison = rankA - rankB
          break
        case 'updated':
          const updatedA = new Date(a.updatedAt).getTime()
          const updatedB = new Date(b.updatedAt).getTime()
          comparison = updatedA - updatedB
          break
        case 'created':
          const createdA = new Date(a.createdAt).getTime()
          const createdB = new Date(b.createdAt).getTime()
          comparison = createdA - createdB
          break
      }

      return sortDirection === 'asc' ? comparison : -comparison
    })
  }, [unfilteredAccounts, sortField, sortDirection, selectedGame])

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(prev => prev === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDirection(field === 'rank' ? 'desc' : 'asc') // Default descending for rank
    }
  }

  const togglePasswordVisibility = (id: string) => {
    setVisiblePasswords(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const copyToClipboard = async (text: string, id: string) => {
    await navigator.clipboard.writeText(text)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  const confirmDelete = async () => {
    if (!deletingAccount) return
    await removeAccount(deletingAccount.id)
    setDeletingAccount(null)
  }

  const getRankForGame = (account: models.Account, gameId: string) => {
    return account.cachedRanks?.find((r: models.CachedRank) => r.gameId === gameId)
  }

  return (
    <div className="flex-1 min-h-0 flex flex-col bg-gradient-to-b from-[var(--color-background)] to-[var(--color-card)]">
      {/* Header - Instagram-style clean header */}
      <header className="wails-drag flex items-center justify-between px-4 sm:px-5 lg:px-6 py-2.5 sm:py-3 bg-[var(--color-background)] border-b border-[var(--color-border)]/30 shrink-0">
        <div className="flex items-center gap-2.5 sm:gap-3">
          <div className="w-7 h-7 sm:w-8 sm:h-8 bg-[var(--color-primary)] rounded-lg flex items-center justify-center shrink-0">
            <Gamepad2 className="w-3.5 h-3.5 sm:w-4 sm:h-4 text-white" />
          </div>
          <div className="min-w-0">
            <h1 className="text-sm sm:text-base font-semibold text-[var(--color-foreground)] truncate">
              Deckstr
            </h1>
            <p className="text-[10px] sm:text-xs text-[var(--color-muted-foreground)] truncate">
              {username}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
          {/* Manual rank detection lives under Settings → Ranked → "Detect Ranks Now".
              Auto-sync still runs in the background; this header used to expose a
              redundant button that was only useful while wiring up the LCU pipeline. */}
          <IconButton
            ariaLabel={soundsOn ? 'Mute sounds' : 'Enable sounds'}
            title={soundsOn ? 'Mute sounds' : 'Enable sounds'}
            tone={soundsOn ? 'neutral' : 'brand'}
            icon={soundsOn ? <Volume2 className="w-4 h-4" /> : <VolumeX className="w-4 h-4" />}
            onClick={toggleSounds}
          />
          <IconButton
            ariaLabel="Settings"
            title="Settings"
            tone="neutral"
            icon={<Settings className="w-4 h-4" />}
            onClick={() => setShowSettingsModal(true)}
          />
        </div>
      </header>

      {/* Currently Playing Banner - Instagram-style clean banner */}
      {detectedAccount && detectedAccount.RiotID && (
        <div className={cn(
          "mx-3 sm:mx-4 lg:mx-5 mt-2.5 sm:mt-3 px-3 sm:px-4 py-2.5 sm:py-3 rounded-lg border transition-all duration-200",
          activeAccountId
            ? "bg-green-500/5 border-green-500/20"
            : "bg-amber-500/5 border-amber-500/20"
        )}>
          <div className="flex items-center gap-2.5 sm:gap-3">
            <div className="relative flex items-center justify-center shrink-0">
              <span className={cn(
                "absolute w-3 h-3 rounded-full animate-ping opacity-30",
                activeAccountId ? "bg-green-500" : "bg-amber-500"
              )} />
              <span className={cn(
                "relative w-2 h-2 rounded-full",
                activeAccountId ? "bg-green-500" : "bg-amber-500"
              )} />
            </div>
            <div className="flex-1 min-w-0">
              <p className={cn(
                "text-[10px] sm:text-xs font-medium uppercase tracking-wider",
                activeAccountId ? "text-green-400/80" : "text-amber-400/80"
              )}>
                {activeAccountId ? "Now Playing" : "Detected"}
              </p>
              <p className="text-sm sm:text-base font-semibold text-[var(--color-foreground)] truncate">
                {detectedAccount.RiotID}
              </p>
            </div>
            {activeAccountId ? (
              <span className="text-[10px] sm:text-xs font-medium text-green-400 bg-green-500/10 px-2 py-0.5 rounded shrink-0">
                Linked
              </span>
            ) : (
              <Button
                size="sm"
                onClick={() => setShowLinkModal(true)}
                className="bg-amber-500 hover:bg-amber-400 text-black shadow-amber-500/25 hover:shadow-amber-500/40 shrink-0"
              >
                Link
              </Button>
            )}
          </div>

          {/* Show top masteries in banner - Compact */}
          {detectedAccount.TopMasteries && detectedAccount.TopMasteries.length > 0 && (
            <div className="mt-2 pt-2 border-t border-white/5 flex items-center gap-2 overflow-x-auto">
              <span className="text-[10px] text-[var(--color-muted-foreground)] shrink-0">Champions:</span>
              <div className="flex gap-1.5">
                {detectedAccount.TopMasteries.map((m, idx) => (
                  <span
                    key={m.championId}
                    className={cn(
                      "px-2 py-0.5 rounded text-[10px] sm:text-xs font-medium",
                      idx === 0 ? "bg-amber-500/10 text-amber-300" :
                      idx === 1 ? "bg-gray-500/10 text-gray-300" :
                      "bg-orange-500/10 text-orange-300"
                    )}
                    title={`M${m.championLevel} - ${m.championPoints.toLocaleString()} pts`}
                  >
                    {m.championName}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Filters - Instagram-style minimal filters */}
      <div className="px-3 sm:px-4 lg:px-5 py-2 sm:py-2.5 space-y-2 shrink-0">
        {/* Search */}
        <div className="relative">
          <Search className="absolute left-2.5 sm:left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[var(--color-muted-foreground)]" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search"
            className={cn(
              'w-full pl-8 sm:pl-9 pr-3 py-1.5 sm:py-2 rounded-md text-sm',
              'bg-[var(--color-muted)]/50 border-none',
              'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
              'focus:outline-none focus:ring-1 focus:ring-[var(--color-border)]',
              'transition-all duration-150'
            )}
          />
        </div>

        {/* Filters Row - Horizontal scroll on mobile */}
        <div className="flex items-center gap-1.5 sm:gap-2 overflow-x-auto pb-1 -mx-1 px-1 scrollbar-none">
          <select
            value={selectedNetworkId || ''}
            onChange={(e) => setSelectedNetwork(e.target.value || null)}
            className={cn(
              'px-2 sm:px-2.5 py-1 sm:py-1.5 rounded-md text-xs font-medium shrink-0',
              'bg-[var(--color-muted)]/50 border-none',
              'text-[var(--color-foreground)]',
              'focus:outline-none cursor-pointer'
            )}
          >
            <option value="">Network</option>
            {gameNetworks.map(network => (
              <option key={network.id} value={network.id}>{network.name}</option>
            ))}
            {hasCustomAccounts && <option value="custom">Custom</option>}
          </select>

          <select
            value={selectedTag || ''}
            onChange={(e) => setSelectedTag(e.target.value || null)}
            className={cn(
              'px-2 sm:px-2.5 py-1 sm:py-1.5 rounded-md text-xs font-medium shrink-0',
              'bg-[var(--color-muted)]/50 border-none',
              'text-[var(--color-foreground)]',
              'focus:outline-none cursor-pointer'
            )}
          >
            <option value="">Tags</option>
            {tags.map(tag => (
              <option key={tag} value={tag}>{tag}</option>
            ))}
          </select>

          <select
            value={selectedGame || ''}
            onChange={(e) => setSelectedGame(e.target.value || null)}
            className={cn(
              'px-2 sm:px-2.5 py-1 sm:py-1.5 rounded-md text-xs font-medium shrink-0',
              'bg-[var(--color-muted)]/50 border-none',
              'text-[var(--color-foreground)]',
              'focus:outline-none cursor-pointer'
            )}
          >
            <option value="">Game</option>
            <option value="lol">LoL</option>
            <option value="tft">TFT</option>
            <option value="valorant">Valorant</option>
          </select>

          <div className="w-px h-4 bg-[var(--color-border)]/50 shrink-0 mx-0.5" />

          {/* Sort Controls - Compact pills */}
          <div className="flex items-center gap-0.5 shrink-0">
            {[
              { field: 'name' as SortField, label: 'Name' },
              { field: 'rank' as SortField, label: 'Rank' },
              { field: 'updated' as SortField, label: 'Date' },
            ].map(({ field, label }) => (
              <button
                key={field}
                onClick={() => toggleSort(field)}
                className={cn(
                  'px-2 py-1 rounded-md text-xs font-medium flex items-center gap-0.5 transition-colors duration-150',
                  sortField === field
                    ? 'bg-[var(--color-foreground)] text-[var(--color-background)]'
                    : 'text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]'
                )}
              >
                {label}
                {sortField === field && (
                  sortDirection === 'asc' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />
                )}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Account List - Prominent account cards */}
      <div className="flex-1 overflow-y-auto px-3 sm:px-4 lg:px-5 py-3 sm:py-4">
        <div className="max-w-2xl mx-auto">
          {accounts.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-center py-12 sm:py-16">
              <div className="w-14 h-14 sm:w-16 sm:h-16 bg-[var(--color-card)] border border-[var(--color-border)]/50 rounded-2xl flex items-center justify-center mb-4">
                <Gamepad2 className="w-7 h-7 sm:w-8 sm:h-8 text-[var(--color-muted-foreground)]" />
              </div>
              <h3 className="text-base sm:text-lg font-semibold text-[var(--color-foreground)]">
                No accounts yet
              </h3>
              <p className="text-sm text-[var(--color-muted-foreground)] mt-1.5 max-w-[240px]">
                Add your first account to start managing your gaming profiles
              </p>
            </div>
          ) : (
            <div className="space-y-3 sm:space-y-4">
              {accounts.map(account => {
                const network = gameNetworks.find(n => n.id === account.networkId)
                const isCustomNetwork = account.networkId === 'custom'
                // Custom accounts store their label on the account itself since
                // the network isn't in gameNetworks.
                const networkLabel = isCustomNetwork
                  ? (account.customNetwork || 'Custom')
                  : network?.name
                const isPasswordVisible = visiblePasswords.has(account.id)
                // Check if this account is the currently signed-in one
                const isActive = activeAccountId === account.id

                return (
                  <Card
                    key={account.id}
                    interactive
                    active={isActive}
                    className="p-4 sm:p-5"
                  >
                    {/* Active indicator bar */}
                    {isActive && (
                      <div className="absolute top-0 left-0 right-0 h-1 bg-gradient-to-r from-green-500 to-emerald-500" />
                    )}

                    {/* Header - Prominent account name */}
                    <div className="flex items-start justify-between mb-3 sm:mb-4">
                      <div className="flex items-center gap-3 min-w-0 flex-1">
                        {/* Account avatar/icon */}
                        <div className={cn(
                          'w-10 h-10 sm:w-12 sm:h-12 rounded-xl flex items-center justify-center text-lg font-bold shrink-0',
                          isActive
                            ? 'bg-gradient-to-br from-green-500/20 to-emerald-500/20 text-green-400'
                            : 'bg-[var(--color-muted)]/50 text-[var(--color-muted-foreground)]'
                        )}>
                          {(account.displayName || account.username).charAt(0).toUpperCase()}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="font-semibold text-base sm:text-lg text-[var(--color-foreground)] truncate">
                              {account.displayName || account.username}
                            </h3>
                            {isActive && (
                              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-green-500/20 text-green-400 uppercase tracking-wide shrink-0">
                                Live
                              </span>
                            )}
                          </div>
                          <p className="text-xs sm:text-sm text-[var(--color-muted-foreground)] truncate mt-0.5">
                            {account.riotId || networkLabel || 'Gaming Account'}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-1 shrink-0 ml-2">
                        <IconButton
                          ariaLabel="Edit account"
                          icon={<Pencil className="w-4 h-4" />}
                          onClick={() => setEditingAccount(account)}
                        />
                        <IconButton
                          ariaLabel="Delete account"
                          tone="destructive"
                          icon={<Trash2 className="w-4 h-4" />}
                          onClick={() => setDeletingAccount(account)}
                        />
                      </div>
                    </div>

                    {/* Credentials */}
                    <div className="space-y-2 bg-[var(--color-background)]/50 rounded-lg p-3">
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-[var(--color-muted-foreground)] w-16 shrink-0">Username</span>
                        <code className="flex-1 text-sm bg-[var(--color-muted)]/30 px-2.5 py-1.5 rounded-md truncate font-medium">
                          {account.username}
                        </code>
                        <IconButton
                          ariaLabel="Copy username"
                          size="sm"
                          tone={copiedId === `${account.id}-user` ? 'success' : 'neutral'}
                          icon={
                            copiedId === `${account.id}-user`
                              ? <CheckIcon className="w-3.5 h-3.5" />
                              : <Copy className="w-3.5 h-3.5" />
                          }
                          onClick={() => copyToClipboard(account.username, `${account.id}-user`)}
                        />
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-[var(--color-muted-foreground)] w-16 shrink-0">Password</span>
                        <code className="flex-1 text-sm bg-[var(--color-muted)]/30 px-2.5 py-1.5 rounded-md font-mono truncate">
                          {isPasswordVisible ? account.password : '••••••••'}
                        </code>
                        <IconButton
                          ariaLabel={isPasswordVisible ? 'Hide password' : 'Show password'}
                          size="sm"
                          icon={isPasswordVisible ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                          onClick={() => togglePasswordVisibility(account.id)}
                        />
                        <IconButton
                          ariaLabel="Copy password"
                          size="sm"
                          tone={copiedId === `${account.id}-pass` ? 'success' : 'neutral'}
                          icon={
                            copiedId === `${account.id}-pass`
                              ? <CheckIcon className="w-3.5 h-3.5" />
                              : <Copy className="w-3.5 h-3.5" />
                          }
                          onClick={() => copyToClipboard(account.password, `${account.id}-pass`)}
                        />
                      </div>
                    </div>

                    {/* Tags & Games */}
                    {(account.tags.length > 0 || (account.games && account.games.length > 0) || account.customGame) && (
                      <div className="flex items-center gap-2 mt-3 flex-wrap">
                        {account.games && account.games.map(gameId => {
                          const badge = gameBadge(gameId, gameNetworks)
                          return (
                            <span
                              key={gameId}
                              className={cn('px-2.5 py-1 text-xs rounded-md font-semibold', badge.classes)}
                            >
                              {badge.label}
                            </span>
                          )
                        })}
                        {isCustomNetwork && account.customGame && (
                          <span className="px-2.5 py-1 text-xs rounded-md font-semibold bg-indigo-500/15 text-indigo-300 border border-indigo-500/20">
                            {account.customGame}
                          </span>
                        )}
                        {account.tags.map(tag => (
                          <span
                            key={tag}
                            className="px-2.5 py-1 text-xs font-medium rounded-md bg-[var(--color-muted)]/50 text-[var(--color-muted-foreground)] border border-[var(--color-border)]/30"
                          >
                            {tag}
                          </span>
                        ))}
                      </div>
                    )}

                    {/* Ranks - Prominent pills */}
                    {account.cachedRanks && account.cachedRanks.length > 0 && (
                      <div className="mt-4 pt-4 border-t border-[var(--color-border)]/30">
                        <p className="text-xs font-medium text-[var(--color-muted-foreground)] mb-2 uppercase tracking-wide">Ranks</p>
                        <div className="space-y-2">
                          {account.cachedRanks.map(rank => (
                            <RankDisplay key={`${rank.gameId}-${rank.queueType}`} rank={rank} />
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Top Masteries */}
                    {account.topMasteries && account.topMasteries.length > 0 && (
                      <div className="mt-4 pt-4 border-t border-[var(--color-border)]/30">
                        <p className="text-xs font-medium text-[var(--color-muted-foreground)] mb-2 uppercase tracking-wide">Top Champions</p>
                        <div className="flex gap-2 flex-wrap">
                          {account.topMasteries.map((mastery) => (
                            <MasteryDisplay key={mastery.championId} mastery={mastery} />
                          ))}
                        </div>
                      </div>
                    )}
                  </Card>
                )
              })}
            </div>
          )}
        </div>
      </div>

      {/* Add Button - Instagram-style floating action */}
      <div className="px-4 sm:px-6 lg:px-8 py-2.5 sm:py-3 bg-[var(--color-background)]/80 backdrop-blur-lg border-t border-[var(--color-border)]/20 shrink-0">
        <div className="max-w-4xl mx-auto flex justify-center">
          <Button
            size="md"
            leadingIcon={<Plus className="w-4 h-4" />}
            onClick={() => setShowAddModal(true)}
          >
            Add Account
          </Button>
        </div>
      </div>

      {/* Add flow — progressive-disclosure wizard. Sticky during first-account
          onboarding so a stray click on the blurred backdrop (or a stray Esc)
          can't dump the user out of a flow they only just started. The Cancel
          button stays as the explicit exit. */}
      {showAddModal && (
        <AddAccountWizard
          onClose={() => setShowAddModal(false)}
          sticky={allAccounts.length === 0}
        />
      )}

      {/* Edit flow — one-page form (user already knows the fields) */}
      {editingAccount && (
        <AccountModal
          account={editingAccount}
          onClose={() => setEditingAccount(null)}
        />
      )}

      {/* Delete Confirmation Modal */}
      {deletingAccount && (
        <DeleteAccountModal
          account={deletingAccount}
          onCancel={() => setDeletingAccount(null)}
          onConfirm={confirmDelete}
        />
      )}

      {/* Link Account Modal */}
      {showLinkModal && detectedAccount && (
        <LinkAccountModal
          detectedAccount={detectedAccount}
          accounts={allAccounts}
          onLink={async (accountId) => {
            const account = allAccounts.find(a => a.id === accountId)
            if (account && detectedAccount) {
              await editAccount({
                ...account,
                riotId: detectedAccount.RiotID,
                puuid: detectedAccount.PUUID,
                cachedRanks: detectedAccount.Ranks || [],
                topMasteries: detectedAccount.TopMasteries || [],
              })
              await loadAccounts()
            }
            setShowLinkModal(false)
          }}
          onClose={() => setShowLinkModal(false)}
        />
      )}

      {/* Settings Modal */}
      {showSettingsModal && (
        <SettingsModal onClose={() => setShowSettingsModal(false)} />
      )}

      {/* Recovery Phrase Modal - shown after password change or generating for legacy users */}
      <RecoveryPhraseModal />
    </div>
  )
}

// Rank Display Component - Colorful tier pills with styled W/L
function RankDisplay({ rank }: { rank: models.CachedRank }) {
  const getTierStyle = (tier: string) => {
    switch (tier?.toUpperCase()) {
      case 'CHALLENGER': return 'bg-gradient-to-r from-amber-500/20 to-yellow-500/20 text-amber-300 border-amber-500/30'
      case 'GRANDMASTER': return 'bg-gradient-to-r from-red-500/20 to-rose-500/20 text-red-400 border-red-500/30'
      case 'MASTER': return 'bg-gradient-to-r from-purple-500/20 to-violet-500/20 text-purple-400 border-purple-500/30'
      case 'DIAMOND': return 'bg-gradient-to-r from-cyan-500/20 to-blue-500/20 text-cyan-400 border-cyan-500/30'
      case 'EMERALD': return 'bg-gradient-to-r from-emerald-500/20 to-green-500/20 text-emerald-400 border-emerald-500/30'
      case 'PLATINUM': return 'bg-gradient-to-r from-teal-500/20 to-cyan-500/20 text-teal-300 border-teal-500/30'
      case 'GOLD': return 'bg-gradient-to-r from-yellow-500/20 to-amber-500/20 text-yellow-400 border-yellow-500/30'
      case 'SILVER': return 'bg-gradient-to-r from-gray-400/20 to-slate-400/20 text-gray-300 border-gray-500/30'
      case 'BRONZE': return 'bg-gradient-to-r from-orange-500/20 to-amber-600/20 text-orange-400 border-orange-500/30'
      case 'IRON': return 'bg-gradient-to-r from-stone-500/20 to-neutral-500/20 text-stone-400 border-stone-500/30'
      default: return 'bg-[var(--color-muted)]/30 text-[var(--color-muted-foreground)] border-[var(--color-border)]/30'
    }
  }

  const getGameLabel = (gameId: string, queueName?: string) => {
    const game = (() => {
      switch (gameId) {
        case 'lol': return 'LoL'
        case 'tft': return 'TFT'
        case 'valorant': return 'VAL'
        default: return gameId
      }
    })()

    // If we have a queue name, show "Game Mode" format
    if (queueName) {
      return `${game} ${queueName}`
    }
    return game
  }

  // Calculate winrate
  const totalGames = rank.wins + rank.losses
  const winrate = totalGames > 0 ? Math.round((rank.wins / totalGames) * 100) : 0
  const getWinrateColor = (wr: number) => {
    if (wr >= 60) return 'text-green-400'
    if (wr >= 50) return 'text-emerald-400'
    if (wr >= 45) return 'text-yellow-400'
    return 'text-red-400'
  }

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-[10px] text-[var(--color-muted-foreground)] shrink-0">{getGameLabel(rank.gameId, rank.queueName)}</span>
      <span className={cn(
        'px-2.5 py-1 rounded-full text-xs font-semibold border',
        getTierStyle(rank.tier)
      )}>
        {rank.displayRank || 'Unranked'}
      </span>
      {totalGames > 0 && (
        <div className="flex items-center gap-1.5 ml-auto">
          <div className="flex items-center gap-1 text-[11px]">
            <span className="text-green-400 font-medium">{rank.wins}W</span>
            <span className="text-[var(--color-muted-foreground)]">/</span>
            <span className="text-red-400 font-medium">{rank.losses}L</span>
          </div>
          <span className={cn(
            'px-1.5 py-0.5 rounded text-[10px] font-bold bg-[var(--color-muted)]/30',
            getWinrateColor(winrate)
          )}>
            {winrate}%
          </span>
        </div>
      )}
    </div>
  )
}

// Mastery Display Component - Colorful badge style
function MasteryDisplay({ mastery }: { mastery: models.ChampionMastery }) {
  // Mastery level colors with gradient backgrounds
  const getMasteryStyle = (level: number) => {
    switch (level) {
      case 7: return 'bg-gradient-to-r from-cyan-500/20 to-blue-500/20 text-cyan-300 border-cyan-500/30'
      case 6: return 'bg-gradient-to-r from-fuchsia-500/20 to-purple-500/20 text-fuchsia-300 border-fuchsia-500/30'
      case 5: return 'bg-gradient-to-r from-red-500/20 to-orange-500/20 text-red-300 border-red-500/30'
      case 4: return 'bg-gradient-to-r from-amber-500/15 to-yellow-500/15 text-amber-300 border-amber-500/25'
      default: return 'bg-[var(--color-muted)]/30 text-[var(--color-muted-foreground)] border-[var(--color-border)]/30'
    }
  }

  // Format points
  const formatPoints = (points: number) => {
    if (points >= 1000000) return `${(points / 1000000).toFixed(1)}M`
    if (points >= 1000) return `${Math.floor(points / 1000)}K`
    return points.toString()
  }

  return (
    <div
      className={cn(
        'px-2.5 py-1.5 rounded-lg border flex items-center gap-2',
        getMasteryStyle(mastery.championLevel)
      )}
      title={`${mastery.championPoints.toLocaleString()} points`}
    >
      <span className="text-xs font-semibold">
        {mastery.championName || `Champ ${mastery.championId}`}
      </span>
      <span className="text-[10px] opacity-70">
        M{mastery.championLevel}
      </span>
    </div>
  )
}

// Link Account Modal - Responsive modal for linking Riot accounts
function LinkAccountModal({
  detectedAccount,
  accounts,
  onLink,
  onClose
}: {
  detectedAccount: providers.DetectedAccount
  accounts: models.Account[]
  onLink: (accountId: string) => Promise<void>
  onClose: () => void
}) {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [linking, setLinking] = useState(false)

  // Filter to show accounts that don't already have a riotId linked
  const availableAccounts = accounts.filter(acc => !acc.riotId || acc.riotId === '')

  const handleLink = async () => {
    if (!selectedId) return
    setLinking(true)
    await onLink(selectedId)
    setLinking(false)
  }

  return (
    <Modal onClose={onClose} size="md">
      <ModalHeader>
        <h2 className="text-base sm:text-lg font-bold text-[var(--color-foreground)]">
          Link Riot Account
        </h2>
        <p className="text-xs sm:text-sm text-[var(--color-muted-foreground)] mt-1">
          Connect <span className="font-semibold text-[var(--color-foreground)] break-all">{detectedAccount.RiotID}</span> to one of your saved accounts
        </p>
      </ModalHeader>

      <div className="p-3 sm:p-4 space-y-2 sm:space-y-3 max-h-[50vh] sm:max-h-80 overflow-y-auto">
          {availableAccounts.length === 0 ? (
            <p className="text-xs sm:text-sm text-[var(--color-muted-foreground)] text-center py-4">
              All accounts are already linked to a Riot ID
            </p>
          ) : (
            availableAccounts.map(account => (
              <button
                key={account.id}
                onClick={() => setSelectedId(account.id)}
                className={cn(
                  'w-full p-2.5 sm:p-3 rounded-lg sm:rounded-xl border text-left transition-all',
                  selectedId === account.id
                    ? 'border-[var(--color-primary)] bg-[var(--color-primary)]/10'
                    : 'border-[var(--color-border)] hover:border-[var(--color-primary)]/50'
                )}
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <p className="font-medium text-sm sm:text-base text-[var(--color-foreground)] truncate">
                      {account.displayName || account.username}
                    </p>
                    <p className="text-xs sm:text-sm text-[var(--color-muted-foreground)] truncate">
                      {account.username}
                    </p>
                  </div>
                  {selectedId === account.id && (
                    <div className="w-5 h-5 rounded-full bg-[var(--color-primary)] flex items-center justify-center shrink-0">
                      <svg className="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7" />
                      </svg>
                    </div>
                  )}
                </div>
                {account.tags.length > 0 && (
                  <div className="flex gap-1 mt-2 flex-wrap">
                    {account.tags.map(tag => (
                      <span
                        key={tag}
                        className="px-2 py-0.5 text-xs rounded-full bg-[var(--color-muted)] text-[var(--color-muted-foreground)]"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
              </button>
            ))
          )}
        </div>

      <ModalFooter>
        <Button fullWidth variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button fullWidth onClick={handleLink} disabled={!selectedId || linking}>
          {linking ? 'Linking...' : 'Link Account'}
        </Button>
      </ModalFooter>
    </Modal>
  )
}

// Account Modal Component - Responsive form modal
export function AccountModal({ account, onClose }: { account: models.Account | null; onClose: () => void }) {
  const { gameNetworks, tags, addAccount, editAccount, createTag } = useAppStore()
  const [formData, setFormData] = useState({
    displayName: account?.displayName || '',
    username: account?.username || '',
    password: account?.password || '',
    networkId: account?.networkId || 'riot',
    tags: account?.tags || [],
    notes: account?.notes || '',
    riotId: account?.riotId || '',
    region: account?.region || 'na1',
    games: account?.games || ['lol', 'tft'],
    customNetwork: account?.customNetwork || '',
    customGame: account?.customGame || '',
  })
  const [newTag, setNewTag] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    if (account) {
      await editAccount({
        id: account.id,
        ...formData,
        cachedRanks: account.cachedRanks || [],
        puuid: account.puuid || '',
      })
    } else {
      await addAccount({
        ...formData,
        cachedRanks: [],
      })
    }

    setLoading(false)
    onClose()
  }

  const handleAddTag = async () => {
    if (newTag.trim()) {
      await createTag(newTag.trim())
      setFormData(prev => ({
        ...prev,
        tags: [...prev.tags, newTag.trim()],
      }))
      setNewTag('')
    }
  }

  const toggleTag = (tag: string) => {
    setFormData(prev => ({
      ...prev,
      tags: prev.tags.includes(tag)
        ? prev.tags.filter(t => t !== tag)
        : [...prev.tags, tag],
    }))
  }

  return (
    <Modal onClose={onClose} size="md">
      <ModalHeader>
        <h2 className="text-base sm:text-lg font-bold text-[var(--color-foreground)]">
          {account ? 'Edit Account' : 'Add Account'}
        </h2>
      </ModalHeader>

      <form id="account-modal-form" onSubmit={handleSubmit} className="p-3 sm:p-4 space-y-3 sm:space-y-4 overflow-y-auto flex-1">
          <div className="space-y-1.5 sm:space-y-2">
            <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
              Display Name
            </label>
            <input
              type="text"
              value={formData.displayName}
              onChange={(e) => setFormData(prev => ({ ...prev, displayName: e.target.value }))}
              placeholder="Optional display name"
              className={cn(
                'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                'bg-[var(--color-muted)] border border-[var(--color-border)]',
                'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
              )}
            />
          </div>

          <div className="space-y-1.5 sm:space-y-2">
            <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
              Username *
            </label>
            <input
              type="text"
              value={formData.username}
              onChange={(e) => setFormData(prev => ({ ...prev, username: e.target.value }))}
              placeholder="Enter username"
              required
              className={cn(
                'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                'bg-[var(--color-muted)] border border-[var(--color-border)]',
                'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
              )}
            />
          </div>

          <div className="space-y-1.5 sm:space-y-2">
            <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
              Password *
            </label>
            <input
              type="text"
              value={formData.password}
              onChange={(e) => setFormData(prev => ({ ...prev, password: e.target.value }))}
              placeholder="Enter password"
              required
              className={cn(
                'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                'bg-[var(--color-muted)] border border-[var(--color-border)]',
                'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
              )}
            />
          </div>

          <div className="space-y-1.5 sm:space-y-2">
            <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
              Game Network
            </label>
            <select
              value={formData.networkId}
              onChange={(e) => {
                const next = e.target.value
                setFormData(prev => ({
                  ...prev,
                  networkId: next,
                  // Drop the other network's fields so we don't persist stale data
                  ...(next === 'custom'
                    ? { riotId: '', region: 'na1', games: [] }
                    : { customNetwork: '', customGame: '' }),
                }))
              }}
              className={cn(
                'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                'bg-[var(--color-muted)] border border-[var(--color-border)]',
                'text-[var(--color-foreground)]',
                'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
              )}
            >
              {gameNetworks.map(network => (
                <option key={network.id} value={network.id}>{network.name}</option>
              ))}
              <option value="custom">Custom</option>
            </select>
          </div>

          {/* Custom network fields */}
          {formData.networkId === 'custom' && (
            <>
              <div className="space-y-1.5 sm:space-y-2">
                <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
                  Network name *
                </label>
                <input
                  type="text"
                  value={formData.customNetwork}
                  onChange={(e) => setFormData(prev => ({ ...prev, customNetwork: e.target.value }))}
                  placeholder="e.g. Steam, Epic Games, Battle.net"
                  required
                  className={cn(
                    'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                    'bg-[var(--color-muted)] border border-[var(--color-border)]',
                    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
                  )}
                />
              </div>
              <div className="space-y-1.5 sm:space-y-2">
                <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
                  Game
                </label>
                <input
                  type="text"
                  value={formData.customGame}
                  onChange={(e) => setFormData(prev => ({ ...prev, customGame: e.target.value }))}
                  placeholder="Optional — e.g. CS2, Apex Legends"
                  className={cn(
                    'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                    'bg-[var(--color-muted)] border border-[var(--color-border)]',
                    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
                  )}
                />
              </div>
            </>
          )}

          {/* Riot-specific fields - Responsive */}
          {formData.networkId === 'riot' && (
            <>
              <div className="space-y-1.5 sm:space-y-2">
                <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
                  Riot ID
                </label>
                <input
                  type="text"
                  value={formData.riotId}
                  onChange={(e) => setFormData(prev => ({ ...prev, riotId: e.target.value }))}
                  placeholder="GameName#TAG"
                  className={cn(
                    'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                    'bg-[var(--color-muted)] border border-[var(--color-border)]',
                    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
                  )}
                />
              </div>

              <div className="space-y-1.5 sm:space-y-2">
                <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
                  Region
                </label>
                <select
                  value={formData.region}
                  onChange={(e) => setFormData(prev => ({ ...prev, region: e.target.value }))}
                  className={cn(
                    'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
                    'bg-[var(--color-muted)] border border-[var(--color-border)]',
                    'text-[var(--color-foreground)]',
                    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
                  )}
                >
                  <option value="na1">NA</option>
                  <option value="euw1">EUW</option>
                  <option value="eun1">EUNE</option>
                  <option value="kr">KR</option>
                  <option value="br1">BR</option>
                  <option value="jp1">JP</option>
                  <option value="oc1">OCE</option>
                  <option value="la1">LAN</option>
                  <option value="la2">LAS</option>
                  <option value="tr1">TR</option>
                  <option value="ru">RU</option>
                </select>
              </div>

              <div className="space-y-1.5 sm:space-y-2">
                <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
                  Games
                </label>
                <div className="flex flex-wrap gap-2 sm:gap-3">
                  {[
                    { id: 'lol', name: 'LoL' },
                    { id: 'tft', name: 'TFT' },
                    { id: 'valorant', name: 'Valorant' },
                  ].map(game => (
                    <label key={game.id} className="flex items-center gap-1.5 sm:gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={formData.games.includes(game.id)}
                        onChange={(e) => {
                          setFormData(prev => ({
                            ...prev,
                            games: e.target.checked
                              ? [...prev.games, game.id]
                              : prev.games.filter(g => g !== game.id)
                          }))
                        }}
                        className="w-3.5 h-3.5 sm:w-4 sm:h-4 rounded border-[var(--color-border)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]"
                      />
                      <span className="text-xs sm:text-sm text-[var(--color-foreground)]">{game.name}</span>
                    </label>
                  ))}
                </div>
              </div>
            </>
          )}

          <div className="space-y-1.5 sm:space-y-2">
            <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
              Tags
            </label>
            <div className="flex flex-wrap gap-1.5 sm:gap-2">
              {tags.map(tag => (
                <button
                  key={tag}
                  type="button"
                  onClick={() => toggleTag(tag)}
                  className={cn(
                    'px-2 sm:px-3 py-0.5 sm:py-1 rounded-full text-xs sm:text-sm transition-colors',
                    formData.tags.includes(tag)
                      ? 'bg-[var(--color-primary)] text-white'
                      : 'bg-[var(--color-muted)] text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]'
                  )}
                >
                  {tag}
                </button>
              ))}
            </div>
            <div className="flex gap-2">
              <input
                type="text"
                value={newTag}
                onChange={(e) => setNewTag(e.target.value)}
                placeholder="New tag"
                className={cn(
                  'flex-1 px-2.5 sm:px-3 py-1.5 rounded-lg text-xs sm:text-sm',
                  'bg-[var(--color-muted)] border border-[var(--color-border)]',
                  'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                  'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
                )}
              />
              <button
                type="button"
                onClick={handleAddTag}
                className="px-2.5 sm:px-3 py-1.5 rounded-lg text-xs sm:text-sm bg-[var(--color-muted)] hover:bg-[var(--color-border)] transition-colors"
              >
                Add
              </button>
            </div>
          </div>

          <div className="space-y-1.5 sm:space-y-2">
            <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)]">
              Notes
            </label>
            <textarea
              value={formData.notes}
              onChange={(e) => setFormData(prev => ({ ...prev, notes: e.target.value }))}
              placeholder="Optional notes"
              rows={2}
              className={cn(
                'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl resize-none text-sm',
                'bg-[var(--color-muted)] border border-[var(--color-border)]',
                'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
                'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]'
              )}
            />
          </div>
        </form>
      <ModalFooter>
        <Button fullWidth variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          fullWidth
          type="submit"
          form="account-modal-form"
          disabled={
            loading ||
            !formData.username ||
            !formData.password ||
            (formData.networkId === 'custom' && !formData.customNetwork.trim())
          }
        >
          {loading ? 'Saving...' : (account ? 'Save' : 'Add')}
        </Button>
      </ModalFooter>
    </Modal>
  )
}

// Delete confirmation modal — in-app replacement for window.confirm()
export function DeleteAccountModal({
  account,
  onCancel,
  onConfirm,
}: {
  account: models.Account
  onCancel: () => void
  onConfirm: () => void | Promise<void>
}) {
  const [loading, setLoading] = useState(false)

  const handleConfirm = async () => {
    setLoading(true)
    try {
      await onConfirm()
    } finally {
      setLoading(false)
    }
  }

  const label = account.displayName || account.username

  return (
    <Modal onClose={onCancel} size="sm">
      <ModalHeader className="flex items-center gap-2 sm:gap-3">
        <div className="w-8 h-8 sm:w-9 sm:h-9 rounded-full bg-red-500/10 flex items-center justify-center shrink-0">
          <Trash2 className="w-4 h-4 sm:w-4.5 sm:h-4.5 text-red-400" />
        </div>
        <h2 className="text-base sm:text-lg font-bold text-[var(--color-foreground)]">
          Delete account?
        </h2>
      </ModalHeader>

      <ModalBody className="text-sm text-[var(--color-muted-foreground)] space-y-2">
        <p>
          This will permanently delete{' '}
          <span className="text-[var(--color-foreground)] font-medium">{label}</span>{' '}
          from your vault. This cannot be undone.
        </p>
      </ModalBody>

      <ModalFooter>
        <Button fullWidth variant="secondary" onClick={onCancel} disabled={loading}>
          Cancel
        </Button>
        <Button fullWidth variant="destructive" onClick={handleConfirm} disabled={loading}>
          {loading ? 'Deleting...' : 'Delete'}
        </Button>
      </ModalFooter>
    </Modal>
  )
}
