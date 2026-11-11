package config

import (
	"bufio"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

// example reads .env.example as a map, uncommented lines only.
func example(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	vars := map[string]string{}
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		if name, value, ok := strings.Cut(lines.Text(), "="); ok && !strings.HasPrefix(name, "#") {
			vars[name] = value
		}
	}
	return vars
}

func TestEveryVariableReadIsDocumentedInEnvExampleAndNoOtherIs(t *testing.T) {
	documented := example(t)
	read := map[string]bool{testDatabaseURL: true}
	if _, err := load(func(name string) string {
		read[name] = true
		return documented[name]
	}); err != nil {
		t.Fatalf("the values in .env.example do not load: %v", err)
	}

	got := slices.Sorted(maps.Keys(read))
	want := slices.Sorted(maps.Keys(documented))
	if !slices.Equal(got, want) {
		t.Fatalf("read %v, documented %v", got, want)
	}
}

// The development commands are off unless DEV_COMMANDS says true, and a value that is no
// yes or no stops the server rather than guessing.
func TestDevCommandsAreOffUnlessTurnedOn(t *testing.T) {
	with := func(value string) (Config, error) {
		return load(func(name string) string {
			switch name {
			case "DATABASE_URL":
				return "postgres://localhost/x"
			case "DEV_COMMANDS":
				return value
			}
			return ""
		})
	}
	for value, want := range map[string]bool{"": false, "false": false, "true": true} {
		if c, err := with(value); err != nil || c.DevCommands != want {
			t.Errorf("DEV_COMMANDS=%q: %v, %v; want %v", value, c.DevCommands, err, want)
		}
	}
	if _, err := with("yes"); err == nil {
		t.Error("DEV_COMMANDS=yes loaded")
	}
}
