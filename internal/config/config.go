// Package config is the one place that reads the environment. Everything else receives
// its settings as plain values.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	// AuthListen and GameListen are where the two ports listen.
	AuthListen netip.AddrPort
	GameListen netip.AddrPort
	// GamePublic is the game address handed to the client, which must be able to reach
	// it; it differs from GameListen when the server listens on 0.0.0.0.
	GamePublic netip.AddrPort
	// DevCommands turns on the chat commands that hand a tester items and yang.
	DevCommands bool
}

// Load reads the environment, falling back to the local defaults the client in this
// project is configured for.
func Load() (Config, error) { return load(os.Getenv) }

// TestDatabaseURL is the database the tests that need PostgreSQL use and wipe.
func TestDatabaseURL() string { return os.Getenv(testDatabaseURL) }

const testDatabaseURL = "TEST_DATABASE_URL"

// load takes the lookup as a parameter so a test can see every variable it reads.
func load(getenv func(string) string) (Config, error) {
	env := func(name, fallback string) string {
		if v := getenv(name); v != "" {
			return v
		}
		return fallback
	}
	var c Config
	c.DatabaseURL = getenv("DATABASE_URL")
	if c.DatabaseURL == "" {
		return c, errors.New("config: DATABASE_URL is not set")
	}
	var err error
	if c.AuthListen, err = addrPort("AUTH_LISTEN", env("AUTH_LISTEN", "127.0.0.1:11002")); err != nil {
		return c, err
	}
	if c.GameListen, err = addrPort("GAME_LISTEN", env("GAME_LISTEN", "127.0.0.1:13001")); err != nil {
		return c, err
	}
	public, err := netip.ParseAddr(env("GAME_PUBLIC_ADDRESS", "127.0.0.1"))
	if err != nil {
		return c, fmt.Errorf("config: GAME_PUBLIC_ADDRESS: %w", err)
	}
	if public.IsUnspecified() || !public.Is4() {
		return c, fmt.Errorf("config: GAME_PUBLIC_ADDRESS must be a reachable IPv4 address, got %s", public)
	}
	c.GamePublic = netip.AddrPortFrom(public, c.GameListen.Port())
	if c.DevCommands, err = strconv.ParseBool(env("DEV_COMMANDS", "false")); err != nil {
		return c, fmt.Errorf("config: DEV_COMMANDS: %w", err)
	}
	return c, nil
}

func addrPort(name, value string) (netip.AddrPort, error) {
	v, err := netip.ParseAddrPort(value)
	if err != nil {
		return v, fmt.Errorf("config: %s: %w", name, err)
	}
	return v, nil
}
