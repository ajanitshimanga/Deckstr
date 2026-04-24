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
  Rocket,
  Store,
  Gamepad,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useAppStore } from '../stores/appStore'
import { cn } from '../lib/utils'
import { buildGamesCatalog, type CatalogGame } from '../lib/catalog'
import { MOTION_BASE, MOTION_FOCUS } from '../lib/motion'
import { Button } from './ui/Button'
import { Modal, ModalBody, ModalFooter } from './ui/Modal'
import { models } from '../../wailsjs/go/models'

// Progressive-disclosure add-account wizard.
// Step 1: Identity (username, password, optional display name)
// Step 2: Game (pick the game first — what players actually think about)
// Step 3: Network (only when the chosen game ships on 2+ stores, e.g. RL on
//         Epic + Steam — multi-select. Skipped for single-store games.)
// Step 4: Details (tags, notes, Riot ID/region for Riot games — all optional)

type Step = 'identity' | 'game' | 'network' | 'details'

// Sentinel for user-defined games we don't have first-class support for.
// Picking "Custom" on the game step routes straight to the details step where
// the user labels their own game + storefront.
export const CUSTOM_GAME_ID = 'custom'
// Backwards-compatible alias — the storage layer still stores NetworkID="custom".
export const CUSTOM_NETWORK_ID = 'custom'

const NETWORK_VISUAL: Record<string, { icon: LucideIcon; color: string }> = {
  riot: { icon: Flame, color: 'text-red-400 bg-red-500/10 border-red-500/30' },
  epic: { icon: Store, color: 'text-fuchsia-300 bg-fuchsia-500/10 border-fuchsia-500/30' },
  steam: { icon: Gamepad, color: 'text-sky-300 bg-sky-500/10 border-sky-500/30' },
  [CUSTOM_NETWORK_ID]: {
    icon: Puzzle,
    color: 'text-indigo-300 bg-indigo-500/10 border-indigo-500/30',
  },
}

const GAME_VISUAL: Record<string, { icon: LucideIcon; color: string }> = {
  lol: { icon: Swords, color: 'text-blue-300 bg-blue-500/10 border-blue-500/30' },
  tft: { icon: Sparkles, color: 'text-purple-300 bg-purple-500/10 border-purple-500/30' },
  valorant: { icon: Crosshair, color: 'text-rose-300 bg-rose-500/10 border-rose-500/30' },
  rl: { icon: Rocket, color: 'text-orange-300 bg-orange-500/10 border-orange-500/30' },
  [CUSTOM_GAME_ID]: {
    icon: Puzzle,
    color: 'text-indigo-300 bg-indigo-500/10 border-indigo-500/30',
  },
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
  gameId: string                // '', a real game id, or CUSTOM_GAME_ID
  // The single storefront this account lives on. Single-store games auto-fill
  // this on game pick; multi-store games leave it blank so the user has to
  // explicitly choose one (Epic OR Steam — different credentials, different
  // surfaces, so we never bundle them into one record).
  selectedNetwork: string
  tags: string[]
  notes: string
  riotId: string
  region: string
  alsoPlaysGames: string[]      // sibling games on the same network this account also covers
  customNetwork: string
  customGame: string
}

const EMPTY: WizardData = {
  displayName: '',
  username: '',
  password: '',
  gameId: '',
  selectedNetwork: '',
  tags: [],
  notes: '',
  riotId: '',
  region: 'na1',
  alsoPlaysGames: [],
  customNetwork: '',
  customGame: '',
}

// stepsFor decides which steps are visible given the current game choice.
// Single-store games (Valorant, LoL) skip the network step entirely;
// Custom skips it because the user types their own network label on details.
function stepsFor(data: WizardData, catalog: CatalogGame[]): Step[] {
  const steps: Step[] = ['identity', 'game']
  if (data.gameId && data.gameId !== CUSTOM_GAME_ID) {
    const game = catalog.find((g) => g.id === data.gameId)
    if (game && game.networks.length >= 2) steps.push('network')
  }
  steps.push('details')
  return steps
}

export function AddAccountWizard({
  onClose,
  sticky = false,
}: {
  onClose: () => void
  // sticky disables backdrop + Esc dismissal so a misclick can't tear the
  // user out of an unfinished flow. The Cancel button stays as the explicit
  // exit. Use during first-account onboarding.
  sticky?: boolean
}) {
  const { gameNetworks, addAccount } = useAppStore()
  const catalog = useMemo(() => buildGamesCatalog(gameNetworks), [gameNetworks])
  const [step, setStep] = useState<Step>('identity')
  const [data, setData] = useState<WizardData>(EMPTY)
  const [submitting, setSubmitting] = useState(false)

  const steps = useMemo(() => stepsFor(data, catalog), [data, catalog])
  const stepIndex = Math.max(0, steps.indexOf(step))
  const isCustom = data.gameId === CUSTOM_GAME_ID
  const selectedGame = catalog.find((g) => g.id === data.gameId)

  // Custom needs a user-provided network name before submission — otherwise
  // the account would show up as a nameless "Custom" in the list.
  const customReady =
    !isCustom || data.customNetwork.trim().length > 0
  const networksReady =
    !steps.includes('network') || data.selectedNetwork.length > 0

  const canAdvance =
    (step === 'identity' && data.username.length > 0 && data.password.length > 0) ||
    (step === 'game' && data.gameId.length > 0) ||
    (step === 'network' && data.selectedNetwork.length > 0) ||
    (step === 'details' && customReady && networksReady)

  const goBack = () => {
    if (stepIndex > 0) setStep(steps[stepIndex - 1])
    else onClose()
  }

  const goNext = () => {
    if (!canAdvance) return
    if (step === 'details') {
      submit()
      return
    }
    setStep(steps[stepIndex + 1])
  }

  const submit = async () => {
    setSubmitting(true)
    try {
      if (isCustom) {
        await addAccount({
          displayName: data.displayName || data.username,
          username: data.username,
          password: data.password,
          networkId: CUSTOM_NETWORK_ID,
          tags: data.tags,
          notes: data.notes,
          riotId: '',
          region: '',
          games: [],
          customNetwork: data.customNetwork.trim(),
          customGame: data.customGame.trim(),
          cachedRanks: [],
        })
      } else {
        // One record per pick — multi-store games (Rocket League on Epic vs
        // Steam) require the user to run the wizard again for the second
        // store, since each storefront uses different credentials.
        const games = uniqueGames(data.gameId, data.alsoPlaysGames)
        const networkId = data.selectedNetwork
        await addAccount({
          displayName: data.displayName || data.username,
          username: data.username,
          password: data.password,
          networkId,
          tags: data.tags,
          notes: data.notes,
          riotId: networkId === 'riot' ? data.riotId : '',
          region: networkId === 'riot' ? data.region : '',
          games,
          customNetwork: '',
          customGame: '',
          cachedRanks: [],
        })
      }
      onClose()
    } finally {
      setSubmitting(false)
    }
  }

  const update = <K extends keyof WizardData>(key: K, value: WizardData[K]) =>
    setData((prev) => ({ ...prev, [key]: value }))

  // Picking a game pre-fills the network only when there's exactly one option
  // (single-store games like LoL) so the network step can be skipped. For
  // multi-store games (Rocket League on Epic + Steam) we leave selectedNetwork
  // blank — the user must explicitly pick a storefront because they're
  // separate accounts with separate credentials.
  //
  // If the chosen network has SharedAccount=true (Riot), every sibling game
  // on that network is auto-tagged: a Riot login always covers LoL + TFT +
  // Valorant, so it would be wrong to ask the user to opt in. The DetailsStep
  // surfaces this as an informational note rather than interactive tiles.
  const selectGame = (game: CatalogGame) =>
    setData((prev) =>
      prev.gameId === game.id
        ? prev
        : {
            ...prev,
            gameId: game.id,
            selectedNetwork: game.networks.length === 1 ? game.networks[0].id : '',
            alsoPlaysGames: linkedSiblings(game, gameNetworks),
            customNetwork: '',
            customGame: '',
          },
    )

  const selectCustomGame = () =>
    setData((prev) =>
      prev.gameId === CUSTOM_GAME_ID
        ? prev
        : {
            ...prev,
            gameId: CUSTOM_GAME_ID,
            selectedNetwork: CUSTOM_NETWORK_ID,
            alsoPlaysGames: [],
          },
    )

  const pickNetwork = (networkId: string) =>
    setData((prev) => ({ ...prev, selectedNetwork: networkId }))

  return (
    <Modal
      onClose={onClose}
      size="md"
      dismissOnBackdrop={!sticky}
      dismissOnEsc={!sticky}
    >
      <WizardHeader step={step} steps={steps} isCustom={isCustom} />

      <ModalBody className="space-y-3 sm:space-y-4">
        {step === 'identity' && <IdentityStep data={data} update={update} />}
        {step === 'game' && (
          <GameStep
            data={data}
            catalog={catalog}
            onSelectGame={selectGame}
            onSelectCustom={selectCustomGame}
          />
        )}
        {step === 'network' && selectedGame && (
          <NetworkStep
            data={data}
            game={selectedGame}
            onPickNetwork={pickNetwork}
          />
        )}
        {step === 'details' && (
          <DetailsStep
            data={data}
            update={update}
            catalog={catalog}
            networks={gameNetworks}
          />
        )}
      </ModalBody>

      <ModalFooter>
        <Button
          fullWidth
          variant="secondary"
          onClick={goBack}
          disabled={submitting}
          leadingIcon={stepIndex > 0 ? <ChevronLeft className="w-4 h-4" /> : undefined}
        >
          {stepIndex === 0 ? 'Cancel' : 'Back'}
        </Button>
        <Button
          fullWidth
          variant="primary"
          onClick={goNext}
          disabled={!canAdvance || submitting}
        >
          {step === 'details' ? (submitting ? 'Adding...' : 'Add Account') : 'Next'}
        </Button>
      </ModalFooter>
    </Modal>
  )
}

function uniqueGames(primary: string, also: string[]): string[] {
  const set = new Set<string>([primary, ...also])
  return Array.from(set)
}

// linkedSiblings returns the sibling games that share a login with the given
// game — i.e. games on a SharedAccount network the chosen game lives on.
// Used to default-tag an account with every game its credentials grant access
// to (Riot LoL pick → also TFT + Valorant). Returns [] for storefronts where
// each game is a separate purchase.
function linkedSiblings(game: CatalogGame, networks: models.GameNetwork[]): string[] {
  const linked = new Set<string>()
  for (const cn of game.networks) {
    if (!cn.sharedAccount) continue
    const network = networks.find((n) => n.id === cn.id)
    if (!network) continue
    for (const g of network.games) {
      if (g.id !== game.id) linked.add(g.id)
    }
  }
  return Array.from(linked)
}

function WizardHeader({
  step,
  steps,
  isCustom,
}: {
  step: Step
  steps: Step[]
  isCustom: boolean
}) {
  const titles: Record<Step, { title: string; subtitle: string }> = {
    identity: { title: 'Add Account', subtitle: 'Start with your sign-in credentials' },
    game: { title: 'Choose Game', subtitle: 'What are you playing?' },
    network: { title: 'Choose Storefront', subtitle: 'Where do you play this game?' },
    details: {
      title: 'Finishing Touches',
      subtitle: isCustom ? 'Name your platform and you\'re done' : 'All optional — skip to save',
    },
  }
  const { title, subtitle } = titles[step]
  const stepIndex = Math.max(0, steps.indexOf(step))

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
          Step {stepIndex + 1} of {steps.length}
        </span>
      </div>
      <div className="flex gap-1.5">
        {steps.map((s, i) => (
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

function GameStep({
  data,
  catalog,
  onSelectGame,
  onSelectCustom,
}: {
  data: WizardData
  catalog: CatalogGame[]
  onSelectGame: (game: CatalogGame) => void
  onSelectCustom: () => void
}) {
  const [query, setQuery] = useState('')
  const filtered = useMemo(
    () => catalog.filter((g) => fuzzyMatch(query, g.name) || fuzzyMatch(query, g.id)),
    [catalog, query],
  )
  // The Custom tile is the escape hatch for games we don't catalog — keep it
  // discoverable even when the search filters real games out.
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
          placeholder="Search games"
          aria-label="Search games"
          className={cn(
            'w-full pl-9 pr-3 py-2 rounded-lg sm:rounded-xl text-sm',
            'bg-[var(--color-muted)] border border-[var(--color-border)]',
            'text-[var(--color-foreground)] placeholder:text-[var(--color-muted-foreground)]',
            'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]',
          )}
        />
      </div>

      <div className="grid grid-cols-2 gap-2 sm:gap-3">
        {filtered.map((g) => (
          <Tile
            key={g.id}
            selected={data.gameId === g.id}
            onClick={() => onSelectGame(g)}
            visual={GAME_VISUAL[g.id] || DEFAULT_VISUAL}
            title={g.name}
            subtitle={
              g.networks.length === 1
                ? g.networks[0].name
                : `${g.networks.length} stores`
            }
          />
        ))}
        {showCustom && (
          <Tile
            selected={data.gameId === CUSTOM_GAME_ID}
            onClick={onSelectCustom}
            visual={GAME_VISUAL[CUSTOM_GAME_ID]}
            title="Custom"
            subtitle="Not listed? Add your own"
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

function NetworkStep({
  data,
  game,
  onPickNetwork,
}: {
  data: WizardData
  game: CatalogGame
  onPickNetwork: (networkId: string) => void
}) {
  return (
    <>
      <p className="text-xs sm:text-sm text-[var(--color-muted-foreground)]">
        {game.name} ships on {game.networks.length} storefronts. Pick the one
        this account is for — each store has its own login, so a second store
        means a second account (run the wizard again).
      </p>

      <div className="grid grid-cols-2 gap-2 sm:gap-3">
        {game.networks.map((n) => (
          <Tile
            key={n.id}
            selected={data.selectedNetwork === n.id}
            onClick={() => onPickNetwork(n.id)}
            visual={NETWORK_VISUAL[n.id] || DEFAULT_VISUAL}
            title={n.name}
            subtitle={data.selectedNetwork === n.id ? 'Selected' : 'Tap to pick'}
          />
        ))}
      </div>
    </>
  )
}

function DetailsStep({
  data,
  update,
  catalog,
  networks,
}: {
  data: WizardData
  update: <K extends keyof WizardData>(key: K, value: WizardData[K]) => void
  catalog: CatalogGame[]
  networks: models.GameNetwork[]
}) {
  const { tags: availableTags, createTag } = useAppStore()
  const isCustom = data.gameId === CUSTOM_GAME_ID
  const selectedGame = catalog.find((g) => g.id === data.gameId)
  const showsRiotFields = !isCustom && data.selectedNetwork === 'riot'

  // Two distinct sibling cases:
  //   - linkedSiblingsInfo: networks where the login spans every game
  //     (Riot). Read-only — we surface them as an info note because the user
  //     can't actually un-link an account from a sibling on the same login.
  //   - optionalSiblings: networks where each game is a separate purchase
  //     (Steam/Epic). Render as opt-in tiles for users who happen to play
  //     siblings on the same account.
  const linkedSiblingsInfo = useMemo(() => {
    if (isCustom || !selectedGame || !data.selectedNetwork) return [] as { id: string; name: string }[]
    const cn = selectedGame.networks.find((n) => n.id === data.selectedNetwork)
    if (!cn || !cn.sharedAccount) return []
    const network = networks.find((n) => n.id === cn.id)
    if (!network) return []
    return network.games
      .filter((g) => g.id !== selectedGame.id)
      .map((g) => ({ id: g.id, name: g.name }))
  }, [isCustom, selectedGame, data.selectedNetwork, networks])

  const optionalSiblings = useMemo(() => {
    if (isCustom || !selectedGame || !data.selectedNetwork) return []
    const cn = selectedGame.networks.find((n) => n.id === data.selectedNetwork)
    if (!cn || cn.sharedAccount) return [] // shared-account siblings shown as info instead
    const network = networks.find((n) => n.id === cn.id)
    if (!network) return []
    return network.games.filter((g) => g.id !== selectedGame.id)
  }, [isCustom, selectedGame, data.selectedNetwork, networks])

  const [newTag, setNewTag] = useState('')

  const toggleTag = (tag: string) => {
    const has = data.tags.includes(tag)
    update('tags', has ? data.tags.filter((t) => t !== tag) : [...data.tags, tag])
  }

  const toggleAlsoPlays = (gameId: string) => {
    const has = data.alsoPlaysGames.includes(gameId)
    update(
      'alsoPlaysGames',
      has ? data.alsoPlaysGames.filter((g) => g !== gameId) : [...data.alsoPlaysGames, gameId],
    )
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

      {linkedSiblingsInfo.length > 0 && (
        <Field label="Linked games">
          <div
            role="note"
            aria-label="Linked games"
            className={cn(
              'flex items-start gap-2 px-3 py-2 rounded-lg sm:rounded-xl text-xs',
              'bg-[var(--color-muted)]/40 border border-[var(--color-border)]',
              'text-[var(--color-muted-foreground)]',
            )}
          >
            <Check className="w-4 h-4 mt-0.5 text-[var(--color-primary)] shrink-0" />
            <span>
              One login covers{' '}
              <span className="text-[var(--color-foreground)] font-medium">
                {[selectedGame?.name, ...linkedSiblingsInfo.map((s) => s.name)]
                  .filter(Boolean)
                  .join(', ')}
              </span>
              {' '}— this account will show up under all of them automatically.
            </span>
          </div>
        </Field>
      )}

      {optionalSiblings.length > 0 && (
        <Field label="Also plays on this account">
          <div className="grid grid-cols-2 gap-2 sm:gap-3">
            {optionalSiblings.map((g) => (
              <Tile
                key={g.id}
                selected={data.alsoPlaysGames.includes(g.id)}
                onClick={() => toggleAlsoPlays(g.id)}
                visual={GAME_VISUAL[g.id] || DEFAULT_VISUAL}
                title={g.name}
                subtitle={data.alsoPlaysGames.includes(g.id) ? 'Selected' : 'Tap to add'}
              />
            ))}
          </div>
          <p className="text-[11px] text-[var(--color-muted-foreground)] mt-1.5">
            Each title on this storefront is a separate purchase — tag the
            extras you happen to own on the same account.
          </p>
        </Field>
      )}

      {showsRiotFields && (
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
        'group relative flex flex-col items-center gap-2 p-3 sm:p-4 rounded-xl border-2',
        MOTION_BASE,
        MOTION_FOCUS,
        'hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.98] motion-reduce:hover:translate-y-0 motion-reduce:active:scale-100',
        selected
          ? 'border-[var(--color-primary)] bg-[var(--color-primary)]/5 shadow-[0_0_0_3px_var(--color-primary)]/10 ring-1 ring-[var(--color-primary)]/30'
          : 'border-[var(--color-border)] bg-[var(--color-muted)]/30 hover:border-[var(--color-muted-foreground)]/40 hover:bg-[var(--color-muted)]/50 hover:shadow-md',
      )}
      aria-pressed={selected}
    >
      {selected && (
        <span className="absolute top-1.5 right-1.5 w-5 h-5 rounded-full bg-[var(--color-primary)] flex items-center justify-center shadow-md shadow-[var(--color-primary)]/30 animate-pop-in">
          <Check className="w-3 h-3 text-white" />
        </span>
      )}
      <div
        className={cn(
          'w-10 h-10 sm:w-12 sm:h-12 rounded-lg flex items-center justify-center border',
          MOTION_BASE,
          'group-hover:scale-110 motion-reduce:group-hover:scale-100',
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
