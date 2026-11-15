// Package tamagotchi provides save/load functionality for the virtual creature.
package tamagotchi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SavePath returns the path where the Tamagotchi state is stored.
func SavePath() string {
	// Use XDG config directory or fallback to home
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		home = "~"
	}
	return filepath.Join(home, ".config", "xscapes", "tamagotchi.json")
}

// SaveState saves the Tamagotchi state to disk.
func SaveState(state interface{}) error {
	dir := filepath.Dir(SavePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(SavePath(), data, 0600)
}

// LoadState loads the Tamagotchi state from disk.
func LoadState() (interface{}, error) {
	data, err := os.ReadFile(SavePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No state yet
		}
		return nil, err
	}
	
	var state interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	
	return state, nil
}

// HasState returns true if a saved state exists.
func HasState() bool {
	_, err := os.Stat(SavePath())
	return err == nil
}

// DeleteState removes the saved state file.
func DeleteState() error {
	return os.Remove(SavePath())
}

// StateAge returns how long the creature has been alive.
func StateAge(state interface{}) time.Duration {
	// This would be implemented with proper type assertion
	return 0
}
