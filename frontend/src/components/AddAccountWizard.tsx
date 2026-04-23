import { useMemo, useState } from 'react'
import {
  Search,
  ChevronLeft,
  Check,
  Gamepad2,
  Swords,
  Crosshair,
  Sparkles,
  Flame,
  Plus,
  Puzzle,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useAppStore } from '../stores/appStore'
import { cn } from '../lib/utils'
import { models } from '../../wailsjs/go/models'

// Progressive-disclosure add-account wizard.
// Step 1: Identity (username, password, optional display name)
// Step 2: Network picker (visual tiles + fuzzy search)
// Step 3: Network-specific details (all optional — sensible defaults let users
//         skip straight to Save)

type Step = 'identity' | 'network' | 'details'

const STEPS: Step[] = ['identity', 'network', 'details']

// Sentinel for user-defined networks we don't have first-class support for
// (Steam, Epic, Battle.net, etc.). The user provides a label in step 3.
export const CUSTOM_NETWORK_ID = 'custom'

const NETWORK_VISUAL: Record<string, { icon: LucideIcon; color: string }> = {
  riot: { icon: Flame, color: 'text-red-400 bg-red-500/10 border-red-500/30' },
  [CUSTOM_NETWORK_ID]: {
    icon: Puzzle,
    color: 'text-indigo-300 bg-indigo-500/10 border-indigo-500/30',
  },
}

const GAME_VISUAL: Record<string, { icon: LucideIcon; color: string }> = {
  lol: { icon: Swords, color: 'text-blue-300 bg-blue-500/10 border-blue-500/30' },
  tft: { icon: Sparkles, color: 'text-purple-300 bg-purple-500/10 border-purple-500/30' },
  valorant: { icon: Crosshair, color: 'text-rose-300 bg-rose-500/10 border-rose-500/30' },
}

const DEFAULT_VISUAL = { icon: Gamepad2, color: 'text-[var(--color-muted-foreground)] bg-[var(--color-muted)] border-[var(--color-border)]' }

// Subsequence fuzzy match — lenient enough for typos ("leag" → "League of Legends")
// without pulling in a full fuzzy-search dep for <20 items.
export function fuzzyMatch(query: string, target: string): boolean {
  const q = query.toLowerCase().trim()
  if (!q) return true
  const t = target.toLowerCase()
  let qi = 0
  for (let ti = 0; ti < t.length && qi < q.length; ti++) {
    if (t[ti] === q[qi]) qi++
  }
  return qi === q.length
}

type WizardData = {
  displayName: string
  username: string
  password: string
  networkId: string
  tags: string[]
  notes: string
  riotId: string
  region: string
  games: string[]
  customNetwork: string
  customGame: string
}

const EMPTY: WizardData = {
  displayName: '',
  username: '',
  password: '',
  networkId: '',
  tags: [],
  notes: '',
  riotId: '',
  region: 'na1',
  games: [],
  customNetwork: '',
  customGame: '',
}

export function AddAccountWizard({ onClose }: { onClose: () => void }) {
  const { gameNetworks, addAccount } = useAppStore()
  const [step, setStep] = useState<Step>('identity')
  const [data, setData] = useState<WizardData>(EMPTY)
  const [submitting, setSubmitting] = useState(false)

  const stepIndex = STEPS.indexOf(step)
  // Custom network needs a user-provided name before submission — otherwise
  // the account would show up as a nameless "Custom" in the list.
  const customReady =
    data.networkId !== CUSTOM_NETWORK_ID || data.customNetwork.trim().length > 0
  const canAdvance =
    (step === 'identity' && data.username.length > 0 && data.password.length > 0) ||
    (step === 'network' && data.networkId.length > 0) ||
    (step === 'details' && customReady)

  const goBack = () => {
    if (stepIndex > 0) setStep(STEPS[stepIndex - 1])
    else onClose()
  }

  const goNext = () => {
    if (!canAdvance) return
    if (step === 'details') {
      submit()
      return
    }
    setStep(STEPS[stepIndex + 1])
  }

  const submit = async () => {
    setSubmitting(true)
    try {
      const isCustom = data.networkId === CUSTOM_NETWORK_ID
      await addAccount({
        displayName: data.displayName || data.username,
        username: data.username,
        password: data.password,
        networkId: data.networkId,
        tags: data.tags,
        notes: data.notes,
        // Riot fields only meaningful for the Riot network
        riotId: isCustom ? '' : data.riotId,
        region: isCustom ? '' : data.region,
        games: isCustom ? [] : data.games,
        customNetwork: isCustom ? data.customNetwork.trim() : '',
        customGame: isCustom ? data.customGame.trim() : '',
        cachedRanks: [],
      })
      onClose()
    } finally {
      setSubmitting(false)
    }
  }

  const update = <K extends keyof WizardData>(key: K, value: WizardData[K]) =>
    setData((prev) => ({ ...prev, [key]: value }))

  // Picking a network pre-selects all its games (opt-out UX > opt-in).
  // Switching to a different network resets to that network's full set and
  // clears any previously-entered custom-network fields.
  const selectNetwork = (network: models.GameNetwork) =>
    setData((prev) =>
      prev.networkId === network.id
        ? prev
        : {
            ...prev,
            networkId: network.id,
            games: network.games.map((g) => g.id),
            customNetwork: '',
            customGame: '',
          },
    )

  // Custom path — no pre-selected games because we don't know what the
  // network offers. User fills in the label on step 3.
  const selectCustom = () =>
    setData((prev) =>
      prev.networkId === CUSTOM_NETWORK_ID ? prev : { ...prev, networkId: CUSTOM_NETWORK_ID, games: [] },
    )

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 z-50">
      <div className="w-full max-w-[95%] sm:max-w-md bg-[var(--color-card)] rounded-xl sm:rounded-2xl border border-[var(--color-border)] overflow-hidden shadow-2xl max-h-[90vh] flex flex-col">
        <WizardHeader step={step} isCustom={data.networkId === CUSTOM_NETWORK_ID} />

        <div className="p-3 sm:p-4 overflow-y-auto flex-1 space-y-3 sm:space-y-4">
          {step === 'identity' && <IdentityStep data={data} update={update} />}
          {step === 'network' && (
            <NetworkStep
              data={data}
              onSelectNetwork={selectNetwork}
              onSelectCustom={selectCustom}
              networks={gameNetworks}
            />
          )}
          {step === 'details' && (
            <DetailsStep data={data} update={update} networks={gameNetworks} />
          )}
        </div>

        <div className="p-3 sm:p-4 border-t border-[var(--color-border)] shrink-0 flex gap-2 sm:gap-3">
          <button
            type="button"
            onClick={goBack}
            disabled={submitting}
            className="flex-1 py-2 sm:py-2.5 rounded-lg sm:rounded-xl font-medium text-sm bg-[var(--color-muted)] hover:bg-[var(--color-border)] transition-colors disabled:opacity-50 flex items-center justify-center gap-1.5"
          >
            {stepIndex > 0 && <ChevronLeft className="w-4 h-4" />}
            {stepIndex === 0 ? 'Cancel' : 'Back'}
          </button>
          <button
            type="button"
            onClick={goNext}
            disabled={!canAdvance || submitting}
            className={cn(
              'flex-1 py-2 sm:py-2.5 rounded-lg sm:rounded-xl font-medium text-sm transition-colors text-white',
              'bg-[var(--color-primary)] hover:bg-[var(--color-primary)]/90',
              'disabled:opacity-50 disabled:cursor-not-allowed',
            )}
          >
            {step === 'details' ? (submitting ? 'Adding...' : 'Add Account') : 'Next'}
          </button>
        </div>
      </div>
    </div>
  )
}

function WizardHeader({ step, isCustom }: { step: Step; isCustom: boolean }) {
  const titles: Record<Step, { title: string; subtitle: string }> = {
    identity: { title: 'Add Account', subtitle: 'Start with your sign-in credentials' },
    network: { title: 'Choose Network', subtitle: 'Which platform is this account for?' },
    details: {
      title: 'Finishing Touches',
      subtitle: isCustom ? 'Name your platform and you\'re done' : 'All optional — skip to save',
    },
  }
  const { title, subtitle } = titles[step]
  const stepIndex = STEPS.indexOf(step)

  return (
    <div className="p-3 sm:p-4 border-b border-[var(--color-border)] shrink-0">
      <div className="flex items-center justify-between gap-3 mb-2">
        <div className="min-w-0">
          <h2 className="text-base sm:text-lg font-bold text-[var(--color-foreground)] truncate">
            {title}
          </h2>
          <p className="text-xs text-[var(--color-muted-foreground)] truncate">{subtitle}</p>
        </div>
        <span className="text-[10px] sm:text-xs font-medium text-[var(--color-muted-foreground)] shrink-0">
          Step {stepIndex + 1} of {STEPS.length}
        </span>
      </div>
      <div className="flex gap-1.5">
        {STEPS.map((s, i) => (
          <div
            key={s}
            className={cn(
              'h-1 flex-1 rounded-full transition-colors',
              i <= stepIndex ? 'bg-[var(--color-primary)]' : 'bg-[var(--color-muted)]',
            )}
          />
        ))}
      </div>
    </div>
  )
}

function IdentityStep({
  data,
  update,
}: {
  data: WizardData
  update: <K extends keyof WizardData>(key: K, value: WizardData[K]) => void
}) {
  const inputClass = cn(
    'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
    'bg-[var(--color-muted)] border border-[var(--color-border)]',
    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]',
  )
  return (
    <>
      <Field label="Username" required>
        <input
          type="text"
          value={data.username}
          onChange={(e) => update('username', e.target.value)}
          placeholder="Enter username"
          autoFocus
          className={inputClass}
        />
      </Field>
      <Field label="Password" required>
        <input
          type="password"
          value={data.password}
          onChange={(e) => update('password', e.target.value)}
          placeholder="Enter password"
          className={inputClass}
        />
      </Field>
      <Field label="Display Name">
        <input
          type="text"
          value={data.displayName}
          onChange={(e) => update('displayName', e.target.value)}
          placeholder="Optional — defaults to username"
          className={inputClass}
        />
      </Field>
    </>
  )
}

function NetworkStep({
  data,
  onSelectNetwork,
  onSelectCustom,
  networks,
}: {
  data: WizardData
  onSelectNetwork: (network: models.GameNetwork) => void
  onSelectCustom: () => void
  networks: models.GameNetwork[]
}) {
  const [query, setQuery] = useState('')
  const filtered = useMemo(
    () => networks.filter((n) => fuzzyMatch(query, n.name) || fuzzyMatch(query, n.id)),
    [networks, query],
  )
  // The Custom tile is the escape hatch for platforms we don't cover — show it
  // whenever the query is empty or fuzzy-matches "custom"/"other" so it stays
  // discoverable, especially when no real networks match.
  const q = query.trim().toLowerCase()
  const showCustom =
    q.length === 0 ||
    fuzzyMatch(query, 'Custom') ||
    fuzzyMatch(query, 'Other') ||
    filtered.length === 0

  return (
    <>
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-muted-foreground)]" />
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search networks"
          aria-label="Search networks"
          className={cn(
            'w-full pl-9 pr-3 py-2 rounded-lg sm:rounded-xl text-sm',
            'bg-[var(--color-muted)] border border-[var(--color-border)]',
            'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
            'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]',
          )}
        />
      </div>

      <div className="grid grid-cols-2 gap-2 sm:gap-3">
        {filtered.map((n) => (
          <Tile
            key={n.id}
            selected={data.networkId === n.id}
            onClick={() => onSelectNetwork(n)}
            visual={NETWORK_VISUAL[n.id] || DEFAULT_VISUAL}
            title={n.name}
            subtitle={`${n.games.length} game${n.games.length === 1 ? '' : 's'}`}
          />
        ))}
        {showCustom && (
          <Tile
            selected={data.networkId === CUSTOM_NETWORK_ID}
            onClick={onSelectCustom}
            visual={NETWORK_VISUAL[CUSTOM_NETWORK_ID]}
            title="Custom"
            subtitle="Not listed? Name your own"
          />
        )}
      </div>

      {filtered.length === 0 && q.length > 0 && (
        <p className="text-xs text-[var(--color-muted-foreground)] text-center pt-1">
          No match for "{query}" — add it as Custom.
        </p>
      )}
    </>
  )
}

function DetailsStep({
  data,
  update,
  networks,
}: {
  data: WizardData
  update: <K extends keyof WizardData>(key: K, value: WizardData[K]) => void
  networks: models.GameNetwork[]
}) {
  const { tags: availableTags, createTag } = useAppStore()
  const network = networks.find((n) => n.id === data.networkId)
  const isCustom = data.networkId === CUSTOM_NETWORK_ID
  const [gameQuery, setGameQuery] = useState('')
  const [newTag, setNewTag] = useState('')
  const filteredGames = useMemo(
    () => (network?.games || []).filter((g) => fuzzyMatch(gameQuery, g.name) || fuzzyMatch(gameQuery, g.id)),
    [network, gameQuery],
  )

  const toggleTag = (tag: string) => {
    const has = data.tags.includes(tag)
    update('tags', has ? data.tags.filter((t) => t !== tag) : [...data.tags, tag])
  }

  const addNewTag = async () => {
    const trimmed = newTag.trim()
    if (!trimmed) return
    if (!availableTags.includes(trimmed)) {
      await createTag(trimmed)
    }
    if (!data.tags.includes(trimmed)) {
      update('tags', [...data.tags, trimmed])
    }
    setNewTag('')
  }

  const inputClass = cn(
    'w-full px-2.5 sm:px-3 py-2 rounded-lg sm:rounded-xl text-sm',
    'bg-[var(--color-muted)] border border-[var(--color-border)]',
    'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
    'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]',
  )

  const toggleGame = (gameId: string) => {
    const has = data.games.includes(gameId)
    update('games', has ? data.games.filter((g) => g !== gameId) : [...data.games, gameId])
  }

  return (
    <>
      {isCustom && (
        <>
          <Field label="Network name" required>
            <input
              type="text"
              value={data.customNetwork}
              onChange={(e) => update('customNetwork', e.target.value)}
              placeholder="e.g. Steam, Epic Games, Battle.net"
              autoFocus
              className={inputClass}
            />
          </Field>
          <Field label="Game">
            <input
              type="text"
              value={data.customGame}
              onChange={(e) => update('customGame', e.target.value)}
              placeholder="Optional — e.g. CS2, Apex Legends, Overwatch"
              className={inputClass}
            />
          </Field>
        </>
      )}

      {!isCustom && network && network.games.length > 0 && (
        <Field label="Games">
          <div className="relative mb-2">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-muted-foreground)]" />
            <input
              type="text"
              value={gameQuery}
              onChange={(e) => setGameQuery(e.target.value)}
              placeholder="Search games"
              aria-label="Search games"
              className={cn(inputClass, 'pl-9')}
            />
          </div>
          <div className="grid grid-cols-2 gap-2 sm:gap-3">
            {filteredGames.map((g) => (
              <Tile
                key={g.id}
                selected={data.games.includes(g.id)}
                onClick={() => toggleGame(g.id)}
                visual={GAME_VISUAL[g.id] || DEFAULT_VISUAL}
                title={g.name}
                subtitle={data.games.includes(g.id) ? 'Selected' : 'Tap to add'}
              />
            ))}
          </div>
          <p className="text-[11px] text-[var(--color-muted-foreground)] mt-1.5">
            All games selected by default — deselect any you don't play.
          </p>
        </Field>
      )}

      {data.networkId === 'riot' && (
        <>
          <Field label="Riot ID">
            <input
              type="text"
              value={data.riotId}
              onChange={(e) => update('riotId', e.target.value)}
              placeholder="GameName#TAG"
              className={inputClass}
            />
          </Field>
          <Field label="Region">
            <select
              value={data.region}
              onChange={(e) => update('region', e.target.value)}
              className={inputClass}
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
          </Field>
        </>
      )}

      <Field label="Tags">
        {availableTags.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mb-2">
            {availableTags.map((tag) => {
              const selected = data.tags.includes(tag)
              return (
                <button
                  key={tag}
                  type="button"
                  onClick={() => toggleTag(tag)}
                  aria-pressed={selected}
                  className={cn(
                    'px-2.5 py-1 rounded-full text-xs font-medium border transition-all',
                    'flex items-center gap-1 active:scale-[0.97]',
                    selected
                      ? 'bg-[var(--color-primary)] text-white border-[var(--color-primary)] shadow-sm'
                      : 'bg-[var(--color-muted)] text-[var(--color-muted-foreground)] border-[var(--color-border)] hover:text-[var(--color-foreground)] hover:border-[var(--color-muted-foreground)]/40',
                  )}
                >
                  {selected && <Check className="w-3 h-3" />}
                  {tag}
                </button>
              )
            })}
          </div>
        )}
        <div className="flex gap-2">
          <input
            type="text"
            value={newTag}
            onChange={(e) => setNewTag(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                addNewTag()
              }
            }}
            placeholder="Create a new tag"
            aria-label="Create a new tag"
            className={cn(inputClass, 'flex-1')}
          />
          <button
            type="button"
            onClick={addNewTag}
            disabled={!newTag.trim()}
            className={cn(
              'px-3 rounded-lg sm:rounded-xl text-sm font-medium transition-colors',
              'bg-[var(--color-muted)] hover:bg-[var(--color-border)]',
              'text-[var(--color-foreground)] flex items-center gap-1',
              'disabled:opacity-50 disabled:cursor-not-allowed',
            )}
          >
            <Plus className="w-3.5 h-3.5" />
            Add
          </button>
        </div>
      </Field>

      <Field label="Notes">
        <textarea
          value={data.notes}
          onChange={(e) => update('notes', e.target.value)}
          placeholder="Optional"
          rows={2}
          className={cn(inputClass, 'resize-none')}
        />
      </Field>
    </>
  )
}

function Field({
  label,
  required,
  children,
}: {
  label: string
  required?: boolean
  children: React.ReactNode
}) {
  return (
    <div className="space-y-1.5 sm:space-y-2">
      <label className="text-xs sm:text-sm font-medium text-[var(--color-foreground)] flex items-center gap-1">
        {label}
        {required && <span className="text-red-400">*</span>}
      </label>
      {children}
    </div>
  )
}

function Tile({
  selected,
  onClick,
  visual,
  title,
  subtitle,
}: {
  selected: boolean
  onClick: () => void
  visual: { icon: LucideIcon; color: string }
  title: string
  subtitle: string
}) {
  const Icon = visual.icon
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'group relative flex flex-col items-center gap-2 p-3 sm:p-4 rounded-xl border-2 transition-all',
        'hover:scale-[1.02] active:scale-[0.98]',
        selected
          ? 'border-[var(--color-primary)] bg-[var(--color-primary)]/5'
          : 'border-[var(--color-border)] bg-[var(--color-muted)]/30 hover:border-[var(--color-muted-foreground)]/40',
      )}
      aria-pressed={selected}
    >
      {selected && (
        <span className="absolute top-1.5 right-1.5 w-5 h-5 rounded-full bg-[var(--color-primary)] flex items-center justify-center">
          <Check className="w-3 h-3 text-white" />
        </span>
      )}
      <div
        className={cn(
          'w-10 h-10 sm:w-12 sm:h-12 rounded-lg flex items-center justify-center border',
          visual.color,
        )}
      >
        <Icon className="w-5 h-5 sm:w-6 sm:h-6" />
      </div>
      <div className="text-center">
        <div className="text-sm font-medium text-[var(--color-foreground)]">{title}</div>
        <div className="text-[10px] sm:text-xs text-[var(--color-muted-foreground)]">{subtitle}</div>
      </div>
    </button>
  )
}
