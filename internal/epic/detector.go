package epic

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"OpenSmurfManager/internal/process"
	"OpenSmurfManager/internal/riotclient"
)

var defaultDataKeys = []string{
	"A09C853C9E95409BB94D707EADEFA52E",
}

type rememberMeEntry struct {
	DisplayName string `json:"DisplayName"`
	Email       string `json:"Email"`
	Name        string `json:"Name"`
	Token       string `json:"Token"`
}

// DetectSignedInAccount reads Epic launcher local session state and returns the signed-in account.
func DetectSignedInAccount() (*riotclient.DetectedAccount, error) {
	if runtime.GOOS != "windows" {
		return nil, &riotclient.DetectionError{
			Code:    "epic_client_offline",
			Message: "Epic detection is unavailable on this platform",
			Retry:   false,
		}
	}

	if !process.IsRunning("EpicGamesLauncher.exe") {
		return nil, &riotclient.DetectionError{
			Code:    "epic_client_offline",
			Message: "Epic Games Launcher is not running",
			Retry:   true,
		}
	}

	data, err := loadRememberMeData()
	if err != nil {
		return nil, err
	}

	entry, err := decodeRememberMeEntry(data)
	if err != nil {
		return nil, err
	}

	label := firstNonEmpty(entry.DisplayName, entry.Name, entry.Email)
	if label == "" {
		return nil, &riotclient.DetectionError{
			Code:    "epic_session_not_found",
			Message: "Epic launcher session is missing account details",
			Retry:   true,
		}
	}

	return &riotclient.DetectedAccount{
		NetworkID:   "epic",
		DisplayName: label,
		Email:       strings.TrimSpace(entry.Email),
		DetectedAt:  time.Now(),
	}, nil
}

func loadRememberMeData() (string, error) {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return "", &riotclient.DetectionError{
			Code:    "epic_config_not_found",
			Message: "Epic launcher config directory was not found",
			Retry:   true,
		}
	}

	candidates := []string{
		filepath.Join(localAppData, "EpicGamesLauncher", "Saved", "Config", "WindowsEditor", "GameUserSettings.ini"),
		filepath.Join(localAppData, "EpicGamesLauncher", "Saved", "Config", "Windows", "GameUserSettings.ini"),
		filepath.Join(localAppData, "EpicGamesLauncher", "Saved", "Config", "WindowsNoEditor", "GameUserSettings.ini"),
	}

	for _, candidate := range candidates {
		data, err := readRememberMeData(candidate)
		if err == nil {
			return data, nil
		}
	}

	return "", &riotclient.DetectionError{
		Code:    "epic_config_not_found",
		Message: "Epic launcher RememberMe data was not found",
		Retry:   true,
	}
}

func readRememberMeData(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inRememberMe := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			inRememberMe = strings.EqualFold(section, "RememberMe")
			continue
		}

		if !inRememberMe {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "Data") {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"")
		if value == "" {
			break
		}
		return value, nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("remember me data not present in %s", path)
}

func decodeRememberMeEntry(encoded string) (*rememberMeEntry, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, &riotclient.DetectionError{
			Code:    "epic_session_not_found",
			Message: "Epic launcher RememberMe data is invalid",
			Retry:   true,
		}
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		entry, err := unmarshalRememberMeEntry(trimmed)
		if err == nil {
			return entry, nil
		}
	}

	for _, key := range defaultDataKeys {
		decoded, err := decryptRememberMe(key, raw)
		if err != nil {
			continue
		}
		entry, err := unmarshalRememberMeEntry(decoded)
		if err == nil {
			return entry, nil
		}
	}

	return nil, &riotclient.DetectionError{
		Code:    "epic_session_not_found",
		Message: "Epic launcher login session could not be decoded",
		Retry:   true,
	}
}

func unmarshalRememberMeEntry(data []byte) (*rememberMeEntry, error) {
	var entries []rememberMeEntry
	if err := json.Unmarshal(data, &entries); err == nil && len(entries) > 0 {
		if entries[0].Token == "" {
			return nil, fmt.Errorf("token missing from remember me data")
		}
		return &entries[0], nil
	}

	var entry rememberMeEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	if entry.Token == "" {
		return nil, fmt.Errorf("token missing from remember me data")
	}
	return &entry, nil
}

func decryptRememberMe(key string, encrypted []byte) ([]byte, error) {
	if len(encrypted) == 0 || len(encrypted)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid encrypted payload length")
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	decrypted := make([]byte, len(encrypted))
	for offset := 0; offset < len(encrypted); offset += aes.BlockSize {
		block.Decrypt(decrypted[offset:offset+aes.BlockSize], encrypted[offset:offset+aes.BlockSize])
	}

	decrypted, err = pkcs7Unpad(decrypted)
	if err != nil {
		return nil, err
	}

	decrypted = bytes.TrimRight(decrypted, "\x00")
	return bytes.TrimSpace(decrypted), nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty payload")
	}

	padding := int(data[len(data)-1])
	if padding <= 0 || padding > aes.BlockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid padding")
	}

	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
