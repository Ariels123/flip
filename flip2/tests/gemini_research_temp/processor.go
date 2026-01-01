package processor

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// User represents a user in the system.
type User struct {
	ID        string
	Username  string
	Email     string
	Age       int
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Roles     []string
	Metadata  map[string]interface{}
}

// UserDTO represents the data transfer object for user creation/updates.
type UserDTO struct {
	Username string                 `json:"username"`
	Email    string                 `json:"email"`
	Age      int                    `json:"age"`
	Roles    []string               `json:"roles"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Processor defines the interface for processing user data.
type Processor interface {
	Validate(dto UserDTO) error
	Transform(dto UserDTO) (User, error)
	ProcessBatch(dtos []UserDTO) ([]User, []error)
}

// DataProcessor implements the Processor interface.
type DataProcessor struct {
	minAge    int
	maxAge    int
	adminRole string
}

// NewDataProcessor creates a new instance of DataProcessor.
func NewDataProcessor(minAge, maxAge int) *DataProcessor {
	return &DataProcessor{
		minAge:    minAge,
		maxAge:    maxAge,
		adminRole: "admin",
	}
}

// Validate checks if the UserDTO is valid.
func (dp *DataProcessor) Validate(dto UserDTO) error {
	if strings.TrimSpace(dto.Username) == "" {
		return errors.New("username is required")
	}

	if len(dto.Username) < 3 {
		return errors.New("username must be at least 3 characters")
	}

	if strings.TrimSpace(dto.Email) == "" {
		return errors.New("email is required")
	}

	if !strings.Contains(dto.Email, "@") {
		return errors.New("invalid email format")
	}

	if dto.Age < dp.minAge || dto.Age > dp.maxAge {
		return fmt.Errorf("age must be between %d and %d", dp.minAge, dp.maxAge)
	}

	return nil
}

// Transform converts a UserDTO into a User struct.
func (dp *DataProcessor) Transform(dto UserDTO) (User, error) {
	// Simulate some complex transformation logic
	normalizedEmail := strings.ToLower(strings.TrimSpace(dto.Email))
	normalizedUsername := strings.TrimSpace(dto.Username)

	// Default metadata if nil
	metadata := dto.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["processed_at"] = time.Now().Format(time.RFC3339)

	// Auto-assign role if missing
	roles := dto.Roles
	if len(roles) == 0 {
		roles = []string{"user"}
	}

	return User{
		ID:        generateID(),
		Username:  normalizedUsername,
		Email:     normalizedEmail,
		Age:       dto.Age,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Roles:     roles,
		Metadata:  metadata,
	}, nil
}

// ProcessBatch processes a list of UserDTOs.
func (dp *DataProcessor) ProcessBatch(dtos []UserDTO) ([]User, []error) {
	var users []User
	var errs []error

	for i, dto := range dtos {
		if err := dp.Validate(dto); err != nil {
			errs = append(errs, fmt.Errorf("item %d validation failed: %w", i, err))
			continue
		}

		user, err := dp.Transform(dto)
		if err != nil {
			errs = append(errs, fmt.Errorf("item %d transformation failed: %w", i, err))
			continue
		}

		// Simulate enrichment
		user = dp.enrichUser(user)
		users = append(users, user)
	}

	return users, errs
}

// enrichUser adds additional computed fields to the user.
func (dp *DataProcessor) enrichUser(u User) User {
	// Logic to add more metadata or computed fields
	u.Metadata["version"] = "v1"
	
	// Check if user is an admin based on roles
	isAdmin := false
	for _, role := range u.Roles {
		if role == dp.adminRole {
			isAdmin = true
			break
		}
	}
	u.Metadata["is_admin"] = isAdmin

	// Generate a display name
	u.Metadata["display_name"] = fmt.Sprintf("%s (%s)", u.Username, u.Email)
	
	return u
}

// Helper function to simulate ID generation
func generateID() string {
	return fmt.Sprintf("usr_%d", time.Now().UnixNano())
}

// MockDatabase simulates a database for saving users.
type MockDatabase struct {
	store map[string]User
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		store: make(map[string]User),
	}
}

func (db *MockDatabase) Save(u User) error {
	if _, exists := db.store[u.ID]; exists {
		return errors.New("user already exists")
	}
	db.store[u.ID] = u
	return nil
}

func (db *MockDatabase) Get(id string) (User, error) {
	u, exists := db.store[id]
	if !exists {
		return User{}, errors.New("user not found")
	}
	return u, nil
}

// BatchSave saves multiple users.
func (db *MockDatabase) BatchSave(users []User) error {
	for _, u := range users {
		if err := db.Save(u); err != nil {
			return err
		}
	}
	return nil
}

// FilterByRole returns users with a specific role.
func (db *MockDatabase) FilterByRole(role string) []User {
	var result []User
	for _, u := range db.store {
		for _, r := range u.Roles {
			if r == role {
				result = append(result, u)
				break
			}
		}
	}
	return result
}

// Stats returns some statistics about the data.
func (db *MockDatabase) Stats() string {
	total := len(db.store)
	active := 0
	for _, u := range db.store {
		if u.IsActive {
			active++
		}
	}
	return fmt.Sprintf("Total Users: %d, Active: %d", total, active)
}
