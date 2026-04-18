package detection

import (
	"errors"
	"strings"
	"time"

	"OpenSmurfManager/internal/epic"
	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/riotclient"
)

// DetectSignedInAccount checks supported launchers/clients and returns the signed-in account.
func DetectSignedInAccount() (*riotclient.DetectedAccount, error) {
	detected, err := riotclient.DetectAndFetchRanks()
	if err == nil && detected != nil {
		return detected, nil
	}
	riotErr := err

	detected, err = epic.DetectSignedInAccount()
	if err == nil && detected != nil {
		return detected, nil
	}
	epicErr := err

	if isInactiveDetectionError(riotErr) && isInactiveDetectionError(epicErr) {
		return nil, &riotclient.DetectionError{
			Code:    "client_offline",
			Message: "No supported client running",
			Retry:   true,
		}
	}

	if riotErr != nil && !isInactiveDetectionError(riotErr) {
		return nil, riotErr
	}
	if epicErr != nil && !isInactiveDetectionError(epicErr) {
		return nil, epicErr
	}
	if epicErr != nil {
		return nil, epicErr
	}
	return nil, riotErr
}

// MatchStoredAccount finds the matching stored account for a detected session.
func MatchStoredAccount(accounts []models.Account, detected *riotclient.DetectedAccount) *models.Account {
	if detected == nil {
		return nil
	}

	switch strings.ToLower(detected.NetworkID) {
	case "epic":
		return matchEpicAccount(accounts, detected)
	default:
		return matchRiotAccount(accounts, detected)
	}
}

// ApplyDetection updates a stored account with network-specific detected details.
func ApplyDetection(account *models.Account, detected *riotclient.DetectedAccount) {
	if account == nil || detected == nil {
		return
	}

	switch strings.ToLower(detected.NetworkID) {
	case "epic":
		if account.EpicEmail == "" && detected.Email != "" {
			account.EpicEmail = detected.Email
		}
		if account.DisplayName == "" && detected.DisplayName != "" {
			account.DisplayName = detected.DisplayName
		}
		account.UpdatedAt = time.Now()
	default:
		riotclient.UpdateAccountRanks(account, detected)
	}
}

func matchRiotAccount(accounts []models.Account, detected *riotclient.DetectedAccount) *models.Account {
	// Try to match by PUUID first (most reliable)
	if detected.PUUID != "" {
		if matched := riotclient.MatchAccountByPUUID(accounts, detected.PUUID); matched != nil {
			return matched
		}
	}

	if detected.RiotID != "" {
		return riotclient.MatchAccountByRiotID(accounts, detected.RiotID)
	}

	return nil
}

func matchEpicAccount(accounts []models.Account, detected *riotclient.DetectedAccount) *models.Account {
	detectedEmail := strings.TrimSpace(detected.Email)
	detectedName := strings.TrimSpace(detected.DisplayName)

	for i := range accounts {
		if accounts[i].NetworkID != "epic" {
			continue
		}
		if detectedEmail != "" && strings.EqualFold(accounts[i].EpicEmail, detectedEmail) {
			return &accounts[i]
		}
	}

	for i := range accounts {
		if accounts[i].NetworkID != "epic" {
			continue
		}
		if detectedEmail != "" && strings.EqualFold(accounts[i].Username, detectedEmail) {
			return &accounts[i]
		}
	}

	for i := range accounts {
		if accounts[i].NetworkID != "epic" {
			continue
		}
		if detectedName != "" && strings.EqualFold(accounts[i].DisplayName, detectedName) {
			return &accounts[i]
		}
	}

	return nil
}

func isInactiveDetectionError(err error) bool {
	if err == nil {
		return false
	}

	var detectionErr *riotclient.DetectionError
	if !errors.As(err, &detectionErr) {
		return false
	}

	switch detectionErr.Code {
	case "client_offline", "lockfile_not_found", "epic_client_offline", "epic_config_not_found", "epic_session_not_found":
		return true
	default:
		return false
	}
}
