package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var reservedNames = map[string]bool{
	"init":    true,
	"add":     true,
	"update":  true,
	"list":    true,
	"remove":  true,
	"export":  true,
	"import":  true,
	"backup":  true,
	"restore": true,
	"version": true,
	"help":    true,
}

// Config holds user-editable settings (~/.passauto/config.json)
type Config struct {
	KeyFile    string `json:"key_file"`
	KeyCommand string `json:"key_command,omitempty"`
}

// Profiles holds encrypted profile data (~/.passauto/data/profiles.json)
type Profiles struct {
	Entries map[string]Profile `json:"profiles"`
}

// Data combines Config + Profiles for backward-compatible API surface
type Data struct {
	Config
	Profiles
}

type Profile struct {
	Command     string    `json:"command"`
	Description string    `json:"description,omitempty"`
	Service     string    `json:"service,omitempty"`
	Patterns    []Pattern `json:"patterns"`
	Secret      string    `json:"secret,omitempty"`
	TOTPSecret  string    `json:"totp_secret,omitempty"`
	Prompt      string    `json:"prompt,omitempty"`
	Timeout     Duration  `json:"timeout"`
	Steps       []string  `json:"steps,omitempty"`
	After       []string  `json:"after,omitempty"`
	KMSKeyID    string    `json:"kms_key_id,omitempty"`
}

type Pattern struct {
	Match         string `json:"match"`
	Hidden        bool   `json:"hidden"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
	TOTP          bool   `json:"totp,omitempty"`
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// Dir returns the passauto root directory (~/.passauto)
// Falls back to ~/.autopass for backward compatibility.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	newDir := filepath.Join(home, ".passauto")
	if _, err := os.Stat(newDir); err == nil {
		return newDir, nil
	}
	oldDir := filepath.Join(home, ".autopass")
	if _, err := os.Stat(oldDir); err == nil {
		return oldDir, nil
	}
	return newDir, nil
}

// ConfigPath returns ~/.passauto/config.json
func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// ProfilesPath returns ~/.passauto/data/profiles.json
func ProfilesPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data", "profiles.json"), nil
}

// LoadConfig reads config.json
func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(stripComments(raw), &c); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &c, nil
}

// SaveConfig writes config.json
func SaveConfig(path string, c *Config) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, raw, 0600)
}

// LoadProfiles reads data/profiles.json
func LoadProfiles(path string) (*Profiles, error) {
	raw, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return &Profiles{Entries: make(map[string]Profile)}, nil
		}
		return nil, fmt.Errorf("reading profiles: %w", err)
	}
	var p Profiles
	if err := json.Unmarshal(stripComments(raw), &p); err != nil {
		return nil, fmt.Errorf("parsing profiles: %w", err)
	}
	if p.Entries == nil {
		p.Entries = make(map[string]Profile)
	}
	if err := validate(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveProfiles writes data/profiles.json
func SaveProfiles(path string, p *Profiles) error {
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, raw, 0600)
}

// --- Legacy compatibility: Load/Save single file ---

// Load reads the old single-file format (data.json) OR new split format.
// Returns a combined Data struct either way.
func Load(path string) (*Data, error) {
	raw, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return &Data{Profiles: Profiles{Entries: make(map[string]Profile)}}, nil
		}
		return nil, fmt.Errorf("reading data file: %w", err)
	}

	// Try legacy single-file format
	var legacy struct {
		KeyFile    string             `json:"key_file"`
		SSHKey     string             `json:"ssh_key"`
		KeyCommand string             `json:"key_command,omitempty"`
		Profiles   map[string]Profile `json:"profiles"`
	}
	if err := json.Unmarshal(stripComments(raw), &legacy); err != nil {
		return nil, fmt.Errorf("parsing data file: %w", err)
	}

	keyFile := legacy.KeyFile
	if keyFile == "" {
		keyFile = legacy.SSHKey
	}

	d := &Data{
		Config:   Config{KeyFile: keyFile, KeyCommand: legacy.KeyCommand},
		Profiles: Profiles{Entries: legacy.Profiles},
	}
	if d.Entries == nil {
		d.Entries = make(map[string]Profile)
	}
	if err := validate(&d.Profiles); err != nil {
		return nil, err
	}
	return d, nil
}

// Save writes the old single-file format (for backward compat during transition)
func Save(path string, d *Data) error {
	legacy := struct {
		KeyFile    string             `json:"key_file"`
		KeyCommand string             `json:"key_command,omitempty"`
		Profiles   map[string]Profile `json:"profiles"`
	}{
		KeyFile:    d.KeyFile,
		KeyCommand: d.KeyCommand,
		Profiles:   d.Entries,
	}
	raw, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, raw, 0600)
}

// --- Profile operations ---

var validProfileName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ProfileKey returns the map key for a (name, service) pair.
func ProfileKey(name, service string) string {
	if service == "" {
		return name
	}
	return name + "@" + service
}

// ParseProfileKey splits a map key back into (name, service).
func ParseProfileKey(key string) (name, service string) {
	if idx := strings.LastIndex(key, "@"); idx > 0 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

func (p *Profiles) AddProfile(name string, profile Profile) error {
	if reservedNames[name] {
		return fmt.Errorf("profile name %q conflicts with a subcommand", name)
	}
	if name == "" {
		return fmt.Errorf("profile name must not be empty")
	}
	if !validProfileName.MatchString(name) {
		return fmt.Errorf("profile name %q is invalid: must start with alphanumeric and contain only alphanumeric, dot, dash, or underscore", name)
	}
	if p.Entries == nil {
		p.Entries = make(map[string]Profile)
	}
	key := ProfileKey(name, profile.Service)
	if _, exists := p.Entries[key]; exists {
		if profile.Service != "" {
			return fmt.Errorf("profile %q (service=%s) already exists", name, profile.Service)
		}
		return fmt.Errorf("profile %q already exists (use 'passauto update %s' to modify)", name, name)
	}
	p.Entries[key] = profile
	return nil
}

// LookupProfile resolves a profile by name and optional service flag.
// If service is provided, returns exact (name, service) match.
// If service is empty and name matches exactly one profile, returns it.
// If service is empty and name matches multiple, returns them all with an error.
func (p *Profiles) LookupProfile(name, service string) (string, Profile, error) {
	if service != "" {
		key := ProfileKey(name, service)
		if prof, ok := p.Entries[key]; ok {
			return key, prof, nil
		}
		return "", Profile{}, fmt.Errorf("profile %q (service=%s) not found", name, service)
	}
	// Try exact key first (no service)
	if prof, ok := p.Entries[name]; ok {
		matches := p.FindProfilesByName(name)
		if len(matches) == 1 {
			return name, prof, nil
		}
		// Multiple profiles with this name exist
		return "", Profile{}, &AmbiguousProfileError{Name: name, Matches: matches}
	}
	// Search all entries for matching name
	matches := p.FindProfilesByName(name)
	if len(matches) == 1 {
		return matches[0].Key, matches[0].Profile, nil
	}
	if len(matches) > 1 {
		return "", Profile{}, &AmbiguousProfileError{Name: name, Matches: matches}
	}
	return "", Profile{}, fmt.Errorf("profile %q not found", name)
}

// ProfileMatch represents a matched profile entry.
type ProfileMatch struct {
	Key     string
	Name    string
	Service string
	Profile Profile
}

// FindProfilesByName returns all profiles whose name matches.
func (p *Profiles) FindProfilesByName(name string) []ProfileMatch {
	var matches []ProfileMatch
	for key, prof := range p.Entries {
		n, s := ParseProfileKey(key)
		if n == name {
			matches = append(matches, ProfileMatch{Key: key, Name: n, Service: s, Profile: prof})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Service < matches[j].Service
	})
	return matches
}

// AmbiguousProfileError is returned when multiple profiles match a name.
type AmbiguousProfileError struct {
	Name    string
	Matches []ProfileMatch
}

func (e *AmbiguousProfileError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "multiple profiles named %q found. Use -s to specify service:\n\n", e.Name)
	for _, m := range e.Matches {
		svc := m.Service
		if svc == "" {
			svc = "(default)"
		}
		fmt.Fprintf(&sb, "  passauto %s -s %s\n", e.Name, svc)
	}
	return sb.String()
}

func (p *Profiles) RemoveProfile(name, service string) error {
	key := ProfileKey(name, service)
	if _, ok := p.Entries[key]; !ok {
		if service != "" {
			return fmt.Errorf("profile %q (service=%s) not found", name, service)
		}
		return fmt.Errorf("profile %q not found", name)
	}
	delete(p.Entries, key)
	return nil
}

func (p *Profiles) ListProfiles() []string {
	keys := make([]string, 0, len(p.Entries))
	for key := range p.Entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// --- Helpers ---

func stripComments(data []byte) []byte {
	var out []byte
	inString := false
	for i := 0; i < len(data); i++ {
		if inString {
			out = append(out, data[i])
			if data[i] == '\\' && i+1 < len(data) {
				i++
				out = append(out, data[i])
			} else if data[i] == '"' {
				inString = false
			}
			continue
		}
		if data[i] == '"' {
			inString = true
			out = append(out, data[i])
		} else if data[i] == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
		} else {
			out = append(out, data[i])
		}
	}
	return out
}

func validate(p *Profiles) error {
	for name, profile := range p.Entries {
		if reservedNames[name] {
			return fmt.Errorf("profile name %q conflicts with a subcommand", name)
		}
		for _, pat := range profile.Patterns {
			if _, err := regexp.Compile(pat.Match); err != nil {
				return fmt.Errorf("invalid regex in profile %q pattern %q: %w", name, pat.Match, err)
			}
		}
	}
	return nil
}
