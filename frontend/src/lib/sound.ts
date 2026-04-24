// Programmatic UI sounds via the Web Audio API.
//
// Why not audio files? Zero asset cost, perfect control of pitch/envelope, and
// trivial to iterate on. The "relaxing" direction here means soft attacks,
// warm harmonics (sine + triangle body), lower pitches, and gentle decays —
// the opposite of the sharp percussive ticks in tool-focused apps.
//
// Vocabulary:
//   hover   — very quiet low tick when the cursor enters an interactive
//             surface. Volume is ~30% of tick so continuous mousing doesn't
//             feel noisy.
//   tick    — small commit (toggle a tag, step a chevron).
//   pop     — bigger commit (pick a tile, add an account).
//   success — two-note ascending warm chime on meaningful completions.
//   error   — deflating descending tone on cancel/failure.
//
// Autoplay: AudioContext starts 'suspended' until a user gesture. We resume
// on the first play() call, which is always triggered from a user event.
// jsdom (unit tests) has no AudioContext — the module no-ops cleanly.

type AudioCtor = typeof AudioContext

declare global {
  interface Window {
    webkitAudioContext?: AudioCtor
  }
}

const PREF_KEY = 'osm.sounds.enabled'
// Versioned key — bump when we change the default so users testing an
// earlier volume aren't locked to the old (louder) value.
const VOL_KEY = 'osm.sounds.volume.v2'

let ctx: AudioContext | null = null
let master: GainNode | null = null
let enabled = loadEnabled()
let volume = loadVolume()

function loadEnabled(): boolean {
  if (typeof localStorage === 'undefined') return true
  const v = localStorage.getItem(PREF_KEY)
  return v === null ? true : v === '1'
}

function loadVolume(): number {
  if (typeof localStorage === 'undefined') return 0.2
  const v = Number(localStorage.getItem(VOL_KEY))
  return Number.isFinite(v) && v > 0 && v <= 1 ? v : 0.2
}

function ensureContext(): { ctx: AudioContext; master: GainNode } | null {
  if (typeof window === 'undefined') return null
  const Ctor = (window.AudioContext || window.webkitAudioContext) as AudioCtor | undefined
  if (!Ctor) return null
  if (!ctx) {
    try {
      ctx = new Ctor()
      master = ctx.createGain()
      master.gain.value = volume
      master.connect(ctx.destination)
    } catch {
      return null
    }
  }
  if (ctx.state === 'suspended') {
    // Fire-and-forget; first play call happens inside a user gesture so this
    // resolves immediately in practice.
    ctx.resume().catch(() => {})
  }
  return { ctx: ctx!, master: master! }
}

export function setSoundsEnabled(on: boolean): void {
  enabled = on
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(PREF_KEY, on ? '1' : '0')
  }
}

export function getSoundsEnabled(): boolean {
  return enabled
}

export function setSoundsVolume(v: number): void {
  volume = Math.max(0, Math.min(1, v))
  if (master) master.gain.value = volume
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(VOL_KEY, String(volume))
  }
}

// Two ±pct random multipliers — used to keep every voice slightly different.
// Real tactile surfaces never produce identical sound twice; our ears tune
// out repetition very quickly, which is what makes fixed synthesised clicks
// feel "annoying" after a few interactions. ±4% on pitch is below conscious
// pitch discrimination but enough to break the repetition.
function jitter(value: number, pct: number): number {
  return value * (1 + (Math.random() * 2 - 1) * pct)
}

// Low-level voice: single oscillator with soft attack + exponential decay
// routed through a lowpass that opens briefly on attack. The filter gives the
// "woody" warmth that straight sines lack.
function voice(opts: {
  freq: number
  type?: OscillatorType
  peak?: number
  attackMs?: number
  decayMs?: number
  filterStart?: number
  filterEnd?: number
  delayMs?: number
  // Set true to keep the voice deterministic (used rarely — e.g. when a
  // caller needs a specific target pitch for a two-note chord).
  fixed?: boolean
}): void {
  if (!enabled) return
  const audio = ensureContext()
  if (!audio) return
  const { ctx, master } = audio

  const {
    freq,
    type = 'sine',
    peak = 0.35,
    attackMs = 4,
    decayMs = 90,
    filterStart = 2400,
    filterEnd = 800,
    delayMs = 0,
    fixed = false,
  } = opts

  // Organic variance. Pitch ±4%, loudness ±12%.
  const effectiveFreq = fixed ? freq : jitter(freq, 0.04)
  const effectivePeak = fixed ? peak : jitter(peak, 0.12)

  const t0 = ctx.currentTime + delayMs / 1000
  const atk = attackMs / 1000
  const dec = decayMs / 1000

  const osc = ctx.createOscillator()
  osc.type = type
  osc.frequency.value = effectiveFreq

  const gain = ctx.createGain()
  gain.gain.setValueAtTime(0, t0)
  gain.gain.linearRampToValueAtTime(effectivePeak, t0 + atk)
  // exponential can't ramp to 0, use a small epsilon
  gain.gain.exponentialRampToValueAtTime(0.0001, t0 + atk + dec)

  const filter = ctx.createBiquadFilter()
  filter.type = 'lowpass'
  filter.frequency.setValueAtTime(filterStart, t0)
  filter.frequency.exponentialRampToValueAtTime(Math.max(200, filterEnd), t0 + atk + dec)

  osc.connect(filter).connect(gain).connect(master)
  osc.start(t0)
  osc.stop(t0 + atk + dec + 0.05)
}

// Deduped hover — 120ms window so grazing across a list of cards doesn't
// machine-gun the ear. Soft, short, felt-like — the user said the earlier
// profile felt "annoying on repetition", so we lean toward "barely there".
let lastHover = 0
export function playHover(): void {
  const now = typeof performance !== 'undefined' ? performance.now() : Date.now()
  if (now - lastHover < 120) return
  lastHover = now
  voice({ freq: 280, type: 'sine', peak: 0.05, attackMs: 3, decayMs: 90, filterStart: 900, filterEnd: 380 })
}

export function playTick(): void {
  // Warmer two-layer: a sine body with a soft triangle under-tone for weight.
  // Peak lowered from 0.22 to ~0.15 so repeated clicks don't fatigue.
  voice({ freq: 620, type: 'sine', peak: 0.15, attackMs: 3, decayMs: 85, filterStart: 1900, filterEnd: 700 })
  voice({ freq: 310, type: 'triangle', peak: 0.1, attackMs: 4, decayMs: 100, filterStart: 1100, filterEnd: 400 })
}

// Layered: high blip for articulation + low triangle body for "satisfaction".
export function playPop(): void {
  voice({ freq: 820, type: 'sine', peak: 0.18, attackMs: 4, decayMs: 80, filterStart: 2400, filterEnd: 950 })
  voice({ freq: 210, type: 'triangle', peak: 0.26, attackMs: 6, decayMs: 160, filterStart: 1300, filterEnd: 380 })
}

export function playSuccess(): void {
  // Fixed pitches so the interval (perfect fifth) stays musical.
  voice({ freq: 523.25, type: 'sine', peak: 0.22, attackMs: 8, decayMs: 200, filterStart: 2200, filterEnd: 750, fixed: true }) // C5
  voice({ freq: 784, type: 'sine', peak: 0.22, attackMs: 8, decayMs: 240, filterStart: 2200, filterEnd: 850, delayMs: 100, fixed: true }) // G5
}

export function playError(): void {
  voice({ freq: 420, type: 'sine', peak: 0.18, attackMs: 6, decayMs: 140, filterStart: 1600, filterEnd: 520 })
  voice({ freq: 315, type: 'sine', peak: 0.18, attackMs: 6, decayMs: 180, filterStart: 1300, filterEnd: 450, delayMs: 80 })
}
