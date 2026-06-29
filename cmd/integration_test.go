package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lifefinity/passauto/internal/data"
)

// setupTestData creates a temporary data.json for testing.
func setupTestData(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".passauto", "data.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("creating test dir: %v", err)
	}

	d := &data.Data{
		Config: data.Config{KeyFile: filepath.Join(dir, "fake_key")},
		Profiles: data.Profiles{Entries: map[string]data.Profile{
			"myserver": {
				Command:     "ssh user@host",
				Description: "Production SSH",
				Patterns:    []data.Pattern{{Match: "(?i)password:", Hidden: true}},
				Secret:      "ZW5jcnlwdGVk", // base64("encrypted")
				Timeout:     data.Duration{Duration: 30 * time.Second},
				After:       []string{"echo done"},
			},
			"mydb": {
				Command:     "psql -h localhost -U admin",
				Description: "Local PostgreSQL",
				Patterns:    []data.Pattern{{Match: "(?i)password", Hidden: true}},
				Secret:      "ZGJzZWNyZXQ=", // base64("dbsecret")
				Prompt:      "=>\\s*$",
				Timeout:     data.Duration{Duration: 60 * time.Second},
				Steps:       []string{"\\timing on", "SELECT 1;"},
			},
		}},
	}

	raw, _ := json.MarshalIndent(d, "", "  ")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("writing test data: %v", err)
	}

	return path, func() { _ = os.RemoveAll(dir) }
}

func TestDataLoad_Integration(t *testing.T) {
	path, cleanup := setupTestData(t)
	defer cleanup()

	d, err := data.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(d.Entries) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(d.Entries))
	}

	p := d.Entries["myserver"]
	if p.Command != "ssh user@host" {
		t.Errorf("unexpected command: %s", p.Command)
	}
	if p.Patterns[0].Match != "(?i)password:" {
		t.Errorf("unexpected pattern: %s", p.Patterns[0].Match)
	}
}

func TestDataAddRemove_Integration(t *testing.T) {
	path, cleanup := setupTestData(t)
	defer cleanup()

	d, err := data.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Add a new profile
	err = d.AddProfile("redis", data.Profile{
		Command:  "redis-cli -h cache.example.com",
		Patterns: []data.Pattern{{Match: "(?i)password:", Hidden: true}},
		Secret:   "cmVkaXM=",
		Timeout:  data.Duration{Duration: 10 * time.Second},
	})
	if err != nil {
		t.Fatalf("AddProfile failed: %v", err)
	}

	if err := data.Save(path, d); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify
	d2, err := data.Load(path)
	if err != nil {
		t.Fatalf("Load after add failed: %v", err)
	}
	if len(d2.Entries) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(d2.Entries))
	}
	if d2.Entries["redis"].Command != "redis-cli -h cache.example.com" {
		t.Fatalf("unexpected redis command: %s", d2.Entries["redis"].Command)
	}

	// Remove
	if err := d2.RemoveProfile("redis", ""); err != nil {
		t.Fatalf("RemoveProfile failed: %v", err)
	}
	if err := data.Save(path, d2); err != nil {
		t.Fatalf("Save after remove failed: %v", err)
	}

	d3, _ := data.Load(path)
	if len(d3.Entries) != 2 {
		t.Fatalf("expected 2 profiles after remove, got %d", len(d3.Entries))
	}
}

func TestDataUpdate_Integration(t *testing.T) {
	path, cleanup := setupTestData(t)
	defer cleanup()

	d, err := data.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Update command field
	p := d.Entries["myserver"]
	p.Command = "ssh newuser@newhost"
	d.Entries["myserver"] = p

	if err := data.Save(path, d); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify
	d2, _ := data.Load(path)
	if d2.Entries["myserver"].Command != "ssh newuser@newhost" {
		t.Fatalf("update not persisted: %s", d2.Entries["myserver"].Command)
	}
	// Other fields unchanged
	if d2.Entries["myserver"].Patterns[0].Match != "(?i)password:" {
		t.Fatalf("pattern should be unchanged: %s", d2.Entries["myserver"].Patterns[0].Match)
	}
}

func TestDataReservedNames_Integration(t *testing.T) {
	reserved := []string{"init", "add", "update", "list", "remove", "version", "help"}

	d := &data.Data{Profiles: data.Profiles{Entries: make(map[string]data.Profile)}}

	for _, name := range reserved {
		err := d.AddProfile(name, data.Profile{Command: "test"})
		if err == nil {
			t.Errorf("expected error for reserved name %q", name)
		}
	}
}

func TestDataListProfiles_Integration(t *testing.T) {
	path, cleanup := setupTestData(t)
	defer cleanup()

	d, _ := data.Load(path)
	names := d.ListProfiles()

	if len(names) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(names))
	}
	// Should be sorted
	if names[0] != "mydb" || names[1] != "myserver" {
		t.Fatalf("expected sorted [mydb, myserver], got %v", names)
	}
}

func TestProfileFields_Integration(t *testing.T) {
	path, cleanup := setupTestData(t)
	defer cleanup()

	d, err := data.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify Description
	if d.Entries["myserver"].Description != "Production SSH" {
		t.Errorf("expected description 'Production SSH', got %q", d.Entries["myserver"].Description)
	}

	// Verify Steps
	if len(d.Entries["mydb"].Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(d.Entries["mydb"].Steps))
	}
	if d.Entries["mydb"].Steps[0] != "\\timing on" {
		t.Errorf("unexpected step[0]: %s", d.Entries["mydb"].Steps[0])
	}

	// Verify After
	if len(d.Entries["myserver"].After) != 1 || d.Entries["myserver"].After[0] != "echo done" {
		t.Errorf("unexpected after: %v", d.Entries["myserver"].After)
	}
}

func TestParseProfileArgs_Env(t *testing.T) {
	args := []string{"myprofile", "-e", "HOST=prod", "--env", "USER=admin"}
	name, opts := parseProfileArgs(args)
	if name != "myprofile" {
		t.Errorf("expected name 'myprofile', got %q", name)
	}
	if len(opts.env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(opts.env))
	}
	if opts.env[0] != "HOST=prod" || opts.env[1] != "USER=admin" {
		t.Errorf("unexpected env: %v", opts.env)
	}
}

func TestParseProfileArgs_After(t *testing.T) {
	args := []string{"mwinit", "--after", "date", "--after", "echo ok"}
	name, opts := parseProfileArgs(args)
	if name != "mwinit" {
		t.Errorf("expected name 'mwinit', got %q", name)
	}
	if len(opts.after) != 2 {
		t.Fatalf("expected 2 after cmds, got %d", len(opts.after))
	}
	if opts.after[0] != "date" || opts.after[1] != "echo ok" {
		t.Errorf("unexpected after: %v", opts.after)
	}
}

func TestParseProfileArgs_Combined(t *testing.T) {
	args := []string{"prod", "--then", "ls", "--script", "deploy.sql", "--prompt", "=>", "-e", "ENV=prod", "--after", "notify", "--quiet"}
	name, opts := parseProfileArgs(args)
	if name != "prod" {
		t.Errorf("expected name 'prod', got %q", name)
	}
	if len(opts.then) != 1 || opts.then[0] != "ls" {
		t.Errorf("unexpected then: %v", opts.then)
	}
	if opts.scriptFile != "deploy.sql" {
		t.Errorf("unexpected script: %s", opts.scriptFile)
	}
	if opts.prompt != "=>" {
		t.Errorf("unexpected prompt: %s", opts.prompt)
	}
	if len(opts.env) != 1 || opts.env[0] != "ENV=prod" {
		t.Errorf("unexpected env: %v", opts.env)
	}
	if len(opts.after) != 1 || opts.after[0] != "notify" {
		t.Errorf("unexpected after: %v", opts.after)
	}
	if !opts.quiet {
		t.Error("expected quiet=true")
	}
}
