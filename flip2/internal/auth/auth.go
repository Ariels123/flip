package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDirName  = ".config" // or just .flip2? Standard is .config/flip2
	flipDirName    = "flip2"
	authFileName   = "auth.json"
)

type AuthData struct {
	Token string `json:"token"`
	Email string `json:"email"`
	ID    string `json:"id"`
}

func getAuthFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName, flipDirName, authFileName), nil
}

func SaveAuth(data AuthData) error {
	path, err := getAuthFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func LoadAuth() (*AuthData, error) {
	path, err := getAuthFilePath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No auth file, not an error
		}
		return nil, err
	}
	defer file.Close()

	var data AuthData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

func Logout() error {
	path, err := getAuthFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
