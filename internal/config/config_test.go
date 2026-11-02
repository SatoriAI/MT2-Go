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
