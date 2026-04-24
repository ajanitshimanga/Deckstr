import { describe, it, expect } from 'vitest'
import { buildGamesCatalog } from '../catalog'
import type { models } from '../../../wailsjs/go/models'

// Wails generates models as classes (with a runtime convertValues method),
// but tests only need the JSON-shaped data. Cast through unknown so the
// fixtures stay readable.
const game = (id: string, name: string, networkId: string): models.Game =>
  ({ id, name, networkId, clientProcess: {}, clientTitle: '', gameProcesses: {} }) as unknown as models.Game

const network = (
  id: string,
  name: string,
  games: models.Game[],
  opts: { sharedAccount?: boolean } = {},
): models.GameNetwork =>
  ({ id, name, games, sharedAccount: opts.sharedAccount ?? false }) as unknown as models.GameNetwork

describe('buildGamesCatalog', () => {
  it('produces one entry per game with its network listed', () => {
    const catalog = buildGamesCatalog([
      network('riot', 'Riot Games', [game('lol', 'League of Legends', 'riot')]),
    ])

    expect(catalog).toHaveLength(1)
    expect(catalog[0].id).toBe('lol')
    expect(catalog[0].networks).toEqual([
      { id: 'riot', name: 'Riot Games', sharedAccount: false },
    ])
  })

  it('merges a cross-store game (e.g. Rocket League) into a single entry with both networks', () => {
    const catalog = buildGamesCatalog([
      network('epic', 'Epic Games', [game('rl', 'Rocket League', 'epic')]),
      network('steam', 'Steam', [game('rl', 'Rocket League', 'steam')]),
    ])

    expect(catalog).toHaveLength(1)
    expect(catalog[0].id).toBe('rl')
    expect(catalog[0].networks.map((n) => n.id)).toEqual(['epic', 'steam'])
  })

  it('preserves source order — first-seen game wins its slot', () => {
    const catalog = buildGamesCatalog([
      network('riot', 'Riot', [
        game('lol', 'League', 'riot'),
        game('tft', 'TFT', 'riot'),
      ]),
      network('epic', 'Epic', [game('rl', 'RL', 'epic')]),
    ])
    expect(catalog.map((g) => g.id)).toEqual(['lol', 'tft', 'rl'])
  })

  it('does not duplicate a network if the same game appears twice under it', () => {
    const catalog = buildGamesCatalog([
      network('riot', 'Riot', [
        game('lol', 'League', 'riot'),
        game('lol', 'League', 'riot'),
      ]),
    ])
    expect(catalog).toHaveLength(1)
    expect(catalog[0].networks).toHaveLength(1)
  })

  it('propagates sharedAccount onto each catalog network entry', () => {
    const catalog = buildGamesCatalog([
      network('riot', 'Riot', [game('lol', 'League', 'riot')], { sharedAccount: true }),
      network('epic', 'Epic', [game('rl', 'RL', 'epic')], { sharedAccount: false }),
    ])
    const lol = catalog.find((g) => g.id === 'lol')!
    const rl = catalog.find((g) => g.id === 'rl')!
    expect(lol.networks[0].sharedAccount).toBe(true)
    expect(rl.networks[0].sharedAccount).toBe(false)
  })

  it('defaults sharedAccount to false when the source omits it', () => {
    const raw = { id: 'epic', name: 'Epic', games: [game('rl', 'RL', 'epic')] } as unknown as models.GameNetwork
    const catalog = buildGamesCatalog([raw])
    expect(catalog[0].networks[0].sharedAccount).toBe(false)
  })
})
