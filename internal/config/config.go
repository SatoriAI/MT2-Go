// Package config is the one place that reads the environment. Everything else receives
// its settings as plain values.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
)

type Config struct {
	DatabaseURL string
	// AuthListen and GameListen are where the two ports listen.
	AuthListen netip.AddrPort
	GameListen netip.AddrPort
	// GamePublic is the game address handed to the client, which must be able to reach
	// it; it differs from GameListen when the server listens on 0.0.0.0.
	GamePublic netip.AddrPort
}

// Load reads the environment, falling back to the local defaults the client in this
// project is configured for.
func Load() (Config, error) {
	var c Config
	c.DatabaseURL = os.Getenv("DATABASE_URL")
	if c.DatabaseURL == "" {
		return c, errors.New("config: DATABASE_URL is not set")
	}
	var err error
	if c.AuthListen, err = addrPort("AUTH_LISTEN", "127.0.0.1:11002"); err != nil {
		return c, err
	}
	if c.GameListen, err = addrPort("GAME_LISTEN", "127.0.0.1:13001"); err != nil {
		return c, err
	}
	public, err := addr("GAME_PUBLIC_ADDRESS", "127.0.0.1")
	if err != nil {
		return c, err
	}
	if public.IsUnspecified() || !public.Is4() {
		return c, fmt.Errorf("config: GAME_PUBLIC_ADDRESS must be a reachable IPv4 address, got %s", public)
	}
	c.GamePublic = netip.AddrPortFrom(public, c.GameListen.Port())
	return c, nil
}

func addrPort(name, fallback string) (netip.AddrPort, error) {
	v, err := netip.ParseAddrPort(env(name, fallback))
	if err != nil {
		return v, fmt.Errorf("config: %s: %w", name, err)
	}
	return v, nil
}

func addr(name, fallback string) (netip.Addr, error) {
	v, err := netip.ParseAddr(env(name, fallback))
	if err != nil {
		return v, fmt.Errorf("config: %s: %w", name, err)
	}
	return v, nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
