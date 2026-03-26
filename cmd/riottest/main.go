package main

import (
	"encoding/json"
	"fmt"
	"os"

	"OpenSmurfManager/internal/riotclient"
)

func main() {
	fmt.Println("=== Riot Client API Test ===")
	fmt.Println()

	// Connect to LCU
	fmt.Println("Connecting to Riot Client...")
	lcu, err := riotclient.NewLCUClient()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Make sure the Riot Client is running and you're logged in.")
		os.Exit(1)
	}
	fmt.Println("Connected!")
	fmt.Println()

	// Get current user from Riot Client auth (always available when logged in)
	fmt.Println("=== Riot Account ===")
	riotAuth, err := lcu.GetRiotClientAuth()
	if err != nil {
		fmt.Printf("Error getting Riot auth: %v\n", err)
	} else {
		fmt.Printf("Riot ID: %s#%s\n", riotAuth.GameName, riotAuth.TagLine)
		fmt.Printf("PUUID: %s\n", riotAuth.PUUID)
	}
	fmt.Println()

	// Try connecting to League Client specifically for ranked data
	fmt.Println("=== Connecting to League Client ===")
	leagueLCU, err := riotclient.NewLeagueLCUClient()
	if err != nil {
		fmt.Printf("League Client not available: %v\n", err)
		fmt.Println("(Launch League of Legends to get ranked data)")
	} else {
		fmt.Println("Connected to League Client!")

		// Get League summoner
		summoner, err := leagueLCU.GetCurrentSummoner()
		if err != nil {
			fmt.Printf("Error getting summoner: %v\n", err)
		} else {
			fmt.Printf("Summoner: %s#%s (Level %d)\n", summoner.GameName, summoner.TagLine, summoner.SummonerLevel)
		}
		fmt.Println()

		// Get League ranks
		fmt.Println("=== League of Legends Ranks ===")
		leagueClient := riotclient.NewLeagueClient(leagueLCU)

		allRanks, err := leagueClient.GetAllRanks()
		if err != nil {
			fmt.Printf("Error getting League ranks: %v\n", err)
		} else if len(allRanks) == 0 {
			fmt.Println("No League ranked data found (unranked)")
		} else {
			for queueType, rank := range allRanks {
				queueName := queueType
				switch queueType {
				case "RANKED_SOLO_5x5":
					queueName = "Solo/Duo"
				case "RANKED_FLEX_SR":
					queueName = "Flex 5v5"
				}
				fmt.Printf("%s: %s (W:%d L:%d)\n", queueName, rank.DisplayRank, rank.Wins, rank.Losses)
			}
		}
		fmt.Println()

		// Get TFT ranks
		fmt.Println("=== Teamfight Tactics Ranks ===")
		tftClient := riotclient.NewTFTClient(leagueLCU)

		tftRanks, err := tftClient.GetAllTFTRanks()
		if err != nil {
			fmt.Printf("Error getting TFT ranks: %v\n", err)
		} else if len(tftRanks) == 0 {
			fmt.Println("No TFT ranked data found (unranked)")
		} else {
			for queueType, rank := range tftRanks {
				queueName := queueType
				switch queueType {
				case "RANKED_TFT":
					queueName = "TFT Ranked"
				case "RANKED_TFT_DOUBLE_UP":
					queueName = "Double Up"
				case "RANKED_TFT_TURBO":
					queueName = "Hyper Roll"
				}
				fmt.Printf("%s: %s (W:%d L:%d)\n", queueName, rank.DisplayRank, rank.Wins, rank.Losses)
			}
		}
	}
	fmt.Println()

	// Get active sessions
	fmt.Println("=== Active Game Sessions ===")
	sessions, err := lcu.GetProductSessions()
	if err != nil {
		fmt.Printf("Error getting sessions: %v\n", err)
	} else if len(sessions) == 0 {
		fmt.Println("No active game sessions")
	} else {
		for id, session := range sessions {
			fmt.Printf("Session %s: %s\n", id, session.ProductID)
		}
	}
	fmt.Println()

	// Output raw ranked data for debugging
	if len(os.Args) > 1 && os.Args[1] == "--raw" {
		fmt.Println("=== Raw Ranked Data (JSON) ===")
		leagueLCURaw, err := riotclient.NewLeagueLCUClient()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			leagueClientRaw := riotclient.NewLeagueClient(leagueLCURaw)
			rankedData, err := leagueClientRaw.GetRankedStats()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				jsonData, _ := json.MarshalIndent(rankedData.QueueMap, "", "  ")
				fmt.Println(string(jsonData))
			}
		}
	}
}
