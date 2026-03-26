package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// LCU Monitor - Captures all LCU API events and logs them to a file
// Run this while interacting with the Riot client to see what APIs are being called

type LockfileData struct {
	ProcessName string
	PID         string
	Port        string
	Password    string
	Protocol    string
}

func findLockfile() (string, error) {
	// Get user's home directory for AppData paths
	homeDir, _ := os.UserHomeDir()
	localAppData := filepath.Join(homeDir, "AppData", "Local")

	// Common Riot Games installation paths
	paths := []string{
		// AppData Local (most common for Riot Client)
		filepath.Join(localAppData, "Riot Games", "Riot Client", "Config", "lockfile"),
		filepath.Join(localAppData, "Riot Games", "League of Legends", "lockfile"),
		// Standard installation paths
		`C:\Riot Games\League of Legends\lockfile`,
		`C:\Riot Games\Riot Client\Config\lockfile`,
		`D:\Riot Games\League of Legends\lockfile`,
		`D:\Riot Games\Riot Client\Config\lockfile`,
	}

	// Also check Program Files
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	if programFiles != "" {
		paths = append(paths, filepath.Join(programFiles, "Riot Games", "League of Legends", "lockfile"))
	}
	if programFilesX86 != "" {
		paths = append(paths, filepath.Join(programFilesX86, "Riot Games", "League of Legends", "lockfile"))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("lockfile not found - is the Riot Client or League running?")
}

func parseLockfile(path string) (*LockfileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read lockfile: %w", err)
	}

	parts := strings.Split(string(data), ":")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid lockfile format")
	}

	return &LockfileData{
		ProcessName: parts[0],
		PID:         parts[1],
		Port:        parts[2],
		Password:    parts[3],
		Protocol:    parts[4],
	}, nil
}

func main() {
	// Output file
	outputFile := "lcu_monitor.log"
	if len(os.Args) > 1 {
		outputFile = os.Args[1]
	}

	// Open log file
	f, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open output file: %v", err)
	}
	defer f.Close()

	// Log to both console and file
	logger := log.New(f, "", log.LstdFlags)

	writeLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		fmt.Println(msg)
		logger.Println(msg)
	}

	writeLog("=== LCU Monitor Started ===")
	writeLog("Output file: %s", outputFile)
	writeLog("Searching for Riot Client lockfile...")

	// Find and parse lockfile
	lockfilePath, err := findLockfile()
	if err != nil {
		writeLog("ERROR: %v", err)
		writeLog("Please start the League of Legends or Riot Client first!")
		os.Exit(1)
	}

	writeLog("Found lockfile: %s", lockfilePath)

	lockfile, err := parseLockfile(lockfilePath)
	if err != nil {
		writeLog("ERROR: %v", err)
		os.Exit(1)
	}

	writeLog("Process: %s (PID: %s)", lockfile.ProcessName, lockfile.PID)
	writeLog("Port: %s", lockfile.Port)
	writeLog("")

	// Connect to WebSocket
	wsURL := fmt.Sprintf("wss://127.0.0.1:%s/", lockfile.Port)
	auth := base64.StdEncoding.EncodeToString([]byte("riot:" + lockfile.Password))

	writeLog("Connecting to LCU WebSocket: %s", wsURL)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // LCU uses self-signed cert
		},
	}

	headers := make(map[string][]string)
	headers["Authorization"] = []string{"Basic " + auth}

	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		writeLog("ERROR connecting to WebSocket: %v", err)
		os.Exit(1)
	}
	defer conn.Close()

	writeLog("Connected to LCU WebSocket!")
	writeLog("")

	// Subscribe to ALL events
	subscribeMsg := `[5, "OnJsonApiEvent"]`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(subscribeMsg)); err != nil {
		writeLog("ERROR subscribing to events: %v", err)
		os.Exit(1)
	}

	writeLog("Subscribed to all JSON API events")
	writeLog("===========================================")
	writeLog("Monitoring... Interact with the Riot Client!")
	writeLog("Press Ctrl+C to stop")
	writeLog("===========================================")
	writeLog("")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		writeLog("\n=== Monitor Stopped ===")
		conn.Close()
		os.Exit(0)
	}()

	// Regex to extract useful info from events
	uriRegex := regexp.MustCompile(`"uri"\s*:\s*"([^"]+)"`)
	eventTypeRegex := regexp.MustCompile(`"eventType"\s*:\s*"([^"]+)"`)

	// Read messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			writeLog("ERROR reading message: %v", err)
			break
		}

		msgStr := string(message)

		// Skip empty or heartbeat messages
		if len(msgStr) < 10 {
			continue
		}

		timestamp := time.Now().Format("15:04:05.000")

		// Extract URI and event type for cleaner logging
		uri := ""
		eventType := ""

		if matches := uriRegex.FindStringSubmatch(msgStr); len(matches) > 1 {
			uri = matches[1]
		}
		if matches := eventTypeRegex.FindStringSubmatch(msgStr); len(matches) > 1 {
			eventType = matches[1]
		}

		// Categorize the event
		category := categorizeEvent(uri)

		// Log summary line
		writeLog("[%s] [%s] %s %s", timestamp, category, eventType, uri)

		// Log full message (pretty print if possible)
		logger.Printf("  Full message: %s\n", msgStr)
	}
}

func categorizeEvent(uri string) string {
	switch {
	case strings.Contains(uri, "/lol-summoner"):
		return "SUMMONER"
	case strings.Contains(uri, "/lol-ranked"):
		return "RANKED"
	case strings.Contains(uri, "/lol-match"):
		return "MATCH"
	case strings.Contains(uri, "/lol-lobby"):
		return "LOBBY"
	case strings.Contains(uri, "/lol-gameflow"):
		return "GAMEFLOW"
	case strings.Contains(uri, "/lol-champ-select"):
		return "CHAMPSELECT"
	case strings.Contains(uri, "/lol-chat"):
		return "CHAT"
	case strings.Contains(uri, "/lol-login"):
		return "LOGIN"
	case strings.Contains(uri, "/riotclient"):
		return "RIOTCLIENT"
	case strings.Contains(uri, "/lol-store"):
		return "STORE"
	case strings.Contains(uri, "/lol-inventory"):
		return "INVENTORY"
	case strings.Contains(uri, "/lol-collections"):
		return "COLLECTIONS"
	case strings.Contains(uri, "/lol-end-of-game"):
		return "ENDGAME"
	case strings.Contains(uri, "/lol-honor"):
		return "HONOR"
	case strings.Contains(uri, "/lol-careers"):
		return "CAREERS"
	case strings.Contains(uri, "/lol-statstones"):
		return "ETERNALS"
	case strings.Contains(uri, "/lol-challenges"):
		return "CHALLENGES"
	case strings.Contains(uri, "/patcher"):
		return "PATCHER"
	case strings.Contains(uri, "/process-control"):
		return "PROCESS"
	case strings.Contains(uri, "/entitlements"):
		return "ENTITLEMENTS"
	case strings.Contains(uri, "/player-notifications"):
		return "NOTIFICATIONS"
	default:
		return "OTHER"
	}
}
