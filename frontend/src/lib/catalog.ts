import { models } from '../../wailsjs/go/models'

// CatalogNetwork describes one storefront a game ships on, carrying the
// sharedAccount flag so wizards can decide whether to auto-link sibling games
// (Riot binds one login to every Riot game; Steam/Epic don't).
export type CatalogNetwork = {
  id: string
  name: string
  sharedAccount: boolean
}

// CatalogGame is the game-first projection of GameNetwork[]: one entry per
// unique game.id, with the list of networks that ship it. Used by the
// AddAccountWizard so users pick "Rocket League" first and only choose a
// storefront when the game lives on more than one (Epic + Steam).
export type CatalogGame = {
  id: string
  name: string
  networks: CatalogNetwork[]
}

// buildGamesCatalog flattens GameNetwork[] into one CatalogGame per game.id,
// preserving the source network order so Riot games appear before Epic/Steam
// titles when the underlying catalog is in that order. A game shared across
// stores (like Rocket League under both Epic and Steam) appears once with two
// network entries.
export function buildGamesCatalog(networks: models.GameNetwork[]): CatalogGame[] {
  const order: string[] = []
  const byId = new Map<string, CatalogGame>()

  for (const network of networks) {
    const entry: CatalogNetwork = {
      id: network.id,
      name: network.name,
      sharedAccount: Boolean(network.sharedAccount),
    }
    for (const game of network.games) {
      const existing = byId.get(game.id)
      if (existing) {
        if (!existing.networks.some((n) => n.id === network.id)) {
          existing.networks.push(entry)
        }
        continue
      }
      byId.set(game.id, {
        id: game.id,
        name: game.name,
        networks: [entry],
      })
      order.push(game.id)
    }
  }

  return order.map((id) => byId.get(id)!)
}
