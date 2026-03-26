package accounts

import (
	"errors"
	"strings"
	"time"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/storage"

	"github.com/google/uuid"
)

var (
	ErrAccountNotFound = errors.New("account not found")
	ErrInvalidAccount  = errors.New("invalid account data")
)

// AccountService handles account CRUD operations
type AccountService struct {
	storage *storage.StorageService
}

// NewAccountService creates a new account service
func NewAccountService(storage *storage.StorageService) *AccountService {
	return &AccountService{
		storage: storage,
	}
}

// GetAll returns all accounts
func (s *AccountService) GetAll() ([]models.Account, error) {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}
	return data.Accounts, nil
}

// GetByID returns a single account by ID
func (s *AccountService) GetByID(id string) (*models.Account, error) {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}

	for _, acc := range data.Accounts {
		if acc.ID == id {
			return &acc, nil
		}
	}

	return nil, ErrAccountNotFound
}

// GetByNetwork returns accounts filtered by network ID
func (s *AccountService) GetByNetwork(networkID string) ([]models.Account, error) {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}

	var filtered []models.Account
	for _, acc := range data.Accounts {
		if acc.NetworkID == networkID {
			filtered = append(filtered, acc)
		}
	}

	return filtered, nil
}

// GetByTag returns accounts filtered by tag
func (s *AccountService) GetByTag(tag string) ([]models.Account, error) {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}

	var filtered []models.Account
	for _, acc := range data.Accounts {
		for _, t := range acc.Tags {
			if strings.EqualFold(t, tag) {
				filtered = append(filtered, acc)
				break
			}
		}
	}

	return filtered, nil
}

// Search returns accounts matching the query (searches display name, username, notes)
func (s *AccountService) Search(query string) ([]models.Account, error) {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []models.Account

	for _, acc := range data.Accounts {
		if strings.Contains(strings.ToLower(acc.DisplayName), query) ||
			strings.Contains(strings.ToLower(acc.Username), query) ||
			strings.Contains(strings.ToLower(acc.Notes), query) {
			filtered = append(filtered, acc)
		}
	}

	return filtered, nil
}

// Create adds a new account
func (s *AccountService) Create(account models.Account) (*models.Account, error) {
	if account.Username == "" {
		return nil, ErrInvalidAccount
	}

	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}

	// Generate ID and timestamps
	account.ID = uuid.New().String()
	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()

	// Initialize empty slices if nil
	if account.Tags == nil {
		account.Tags = []string{}
	}
	if account.CachedRanks == nil {
		account.CachedRanks = []models.CachedRank{}
	}
	if account.Games == nil {
		account.Games = []string{}
	}

	data.Accounts = append(data.Accounts, account)

	if err := s.storage.UpdateVaultData(data); err != nil {
		return nil, err
	}

	if err := s.storage.Save(); err != nil {
		return nil, err
	}

	return &account, nil
}

// Update modifies an existing account
func (s *AccountService) Update(account models.Account) (*models.Account, error) {
	if account.ID == "" || account.Username == "" {
		return nil, ErrInvalidAccount
	}

	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}

	found := false
	for i, acc := range data.Accounts {
		if acc.ID == account.ID {
			account.CreatedAt = acc.CreatedAt // Preserve original creation time
			account.UpdatedAt = time.Now()
			data.Accounts[i] = account
			found = true
			break
		}
	}

	if !found {
		return nil, ErrAccountNotFound
	}

	if err := s.storage.UpdateVaultData(data); err != nil {
		return nil, err
	}

	if err := s.storage.Save(); err != nil {
		return nil, err
	}

	return &account, nil
}

// Delete removes an account by ID
func (s *AccountService) Delete(id string) error {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return err
	}

	found := false
	for i, acc := range data.Accounts {
		if acc.ID == id {
			data.Accounts = append(data.Accounts[:i], data.Accounts[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return ErrAccountNotFound
	}

	if err := s.storage.UpdateVaultData(data); err != nil {
		return err
	}

	return s.storage.Save()
}

// UpdateRank updates rank info for an account and game (legacy manual update)
func (s *AccountService) UpdateRank(accountID, gameID, rank string, lp int) error {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return err
	}

	for i, acc := range data.Accounts {
		if acc.ID == accountID {
			// Find existing rank for this game or create new
			found := false
			for j, r := range acc.CachedRanks {
				if r.GameID == gameID {
					data.Accounts[i].CachedRanks[j].DisplayRank = rank
					data.Accounts[i].CachedRanks[j].LP = lp
					data.Accounts[i].CachedRanks[j].LastUpdated = time.Now()
					found = true
					break
				}
			}

			if !found {
				data.Accounts[i].CachedRanks = append(data.Accounts[i].CachedRanks, models.CachedRank{
					GameID:      gameID,
					DisplayRank: rank,
					LP:          lp,
					LastUpdated: time.Now(),
				})
			}

			data.Accounts[i].UpdatedAt = time.Now()

			if err := s.storage.UpdateVaultData(data); err != nil {
				return err
			}

			return s.storage.Save()
		}
	}

	return ErrAccountNotFound
}

// GetAllTags returns all available tags
func (s *AccountService) GetAllTags() ([]string, error) {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}
	return data.Tags, nil
}

// AddTag adds a new tag to the available tags
func (s *AccountService) AddTag(tag string) error {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return err
	}

	// Check if tag already exists
	for _, t := range data.Tags {
		if strings.EqualFold(t, tag) {
			return nil // Already exists
		}
	}

	data.Tags = append(data.Tags, tag)

	if err := s.storage.UpdateVaultData(data); err != nil {
		return err
	}

	return s.storage.Save()
}

// GetGameNetworks returns all game networks
func (s *AccountService) GetGameNetworks() ([]models.GameNetwork, error) {
	data, err := s.storage.GetVaultData()
	if err != nil {
		return nil, err
	}
	return data.GameNetworks, nil
}
