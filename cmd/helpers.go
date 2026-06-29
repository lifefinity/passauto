package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/term"

	"github.com/lifefinity/passauto/internal/cache"
	"github.com/lifefinity/passauto/internal/crypto"
	"github.com/lifefinity/passauto/internal/data"
)

var (
	noCache  bool
	cacheTTL = 1 * time.Hour
)

func dataPath() (string, error) {
	return data.ProfilesPath()
}

func configPath() (string, error) {
	return data.ConfigPath()
}

func loadData() (*data.Data, error) {
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}
	profPath, err := dataPath()
	if err != nil {
		return nil, err
	}

	// Check if initialized (config.json exists)
	if _, statErr := os.Stat(cfgPath); os.IsNotExist(statErr) {
		// Try legacy data.json
		dir, _ := data.Dir()
		legacyPath := filepath.Join(dir, "data.json")
		if _, legErr := os.Stat(legacyPath); legErr == nil {
			return data.Load(legacyPath)
		}
		fmt.Println("passauto is not initialized. Running setup now...")
		if initErr := runInit(nil, nil); initErr != nil {
			return nil, fmt.Errorf("auto-initialization failed: %w", initErr)
		}
	}

	cfg, err := data.LoadConfig(cfgPath)
	if err != nil {
		return nil, err
	}
	prof, err := data.LoadProfiles(profPath)
	if err != nil {
		return nil, err
	}

	return &data.Data{Config: *cfg, Profiles: *prof}, nil
}

func saveData(d *data.Data) error {
	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	profPath, err := dataPath()
	if err != nil {
		return err
	}
	if err := data.SaveConfig(cfgPath, &d.Config); err != nil {
		return err
	}
	return data.SaveProfiles(profPath, &d.Profiles)
}

func deriveKey() ([]byte, error) {
	return deriveKeyForProfile("")
}

func deriveKeyForProfile(profile string) ([]byte, error) {
	// Try cache first
	if !noCache && profile != "" {
		if cached, _ := cache.Get(profile, cacheTTL); cached != nil {
			return cached, nil
		}
	}

	d, err := loadData()
	if err != nil {
		return nil, err
	}

	// Priority: key_command > key_file
	if d.KeyCommand != "" {
		key, err := deriveKeyFromCommand(d.KeyCommand)
		if err != nil {
			return nil, err
		}
		if !noCache && profile != "" {
			_ = cache.Set(profile, key)
		}
		return key, nil
	}

	home, _ := os.UserHomeDir()
	keyFilePath := d.KeyFile
	if len(keyFilePath) > 2 && keyFilePath[:2] == "~/" {
		keyFilePath = filepath.Join(home, keyFilePath[2:])
	}

	key, err := crypto.DeriveKey(keyFilePath, nil)
	if err != nil {
		fmt.Print("Enter SSH key passphrase: ")
		passphrase, readErr := term.ReadPassword(int(os.Stdin.Fd())) // #nosec G115
		fmt.Println()
		if readErr != nil {
			return nil, fmt.Errorf("reading passphrase: %w", readErr)
		}
		key, err = crypto.DeriveKey(keyFilePath, passphrase)
		if err != nil {
			return nil, fmt.Errorf("deriving key: %w", err)
		}
	}

	if !noCache && profile != "" {
		_ = cache.Set(profile, key)
	}

	return key, nil
}

func deriveKeyFromCommand(command string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command) // #nosec G204 -- user-configured key command
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("key_command failed: %w", err)
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return nil, fmt.Errorf("key_command returned empty output")
	}

	key, err := crypto.DeriveKeyFromBytes(raw)
	// Zero the raw material
	for i := range raw {
		raw[i] = 0
	}
	if err != nil {
		return nil, err
	}

	return key, nil
}
