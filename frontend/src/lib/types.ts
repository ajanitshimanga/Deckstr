// Re-export types from Wails-generated models
export type { models, riotclient } from '../../wailsjs/go/models'

// Import the classes for use
import { models } from '../../wailsjs/go/models'

export type Account = models.Account
export type GameNetwork = models.GameNetwork
export type Game = models.Game
export type CachedRank = models.CachedRank
export type ChampionMastery = models.ChampionMastery
export type Settings = models.Settings

// App state
export type AppState = 'loading' | 'locked' | 'create' | 'unlocked'

// Helper to create a plain account object for API calls
export interface AccountInput {
  id?: string
  displayName: string
  username: string
  password: string
  networkId: string
  tags: string[]
  notes: string
  // Riot-specific fields
  riotId?: string
  epicEmail?: string
  region?: string
  games?: string[]
  cachedRanks?: models.CachedRank[]
  topMasteries?: models.ChampionMastery[]
  puuid?: string
}
