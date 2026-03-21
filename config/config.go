package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Server struct {
	URL      string `toml:"url"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

type Player struct {
	Volume int `toml:"volume"`
}

type ListenBrainz struct {
	Token    string `toml:"token"`
	Username string `toml:"username"`
}

type Config struct {
	Server       Server       `toml:"server"`
	Player       Player       `toml:"player"`
	ListenBrainz ListenBrainz `toml:"listenbrainz"`
}

func defaultConfig() Config {
	return Config{
		Server: Server{
			URL:      "https://my-server.com",
			Username: "user",
			Password: "pass",
		},
		Player: Player{
			Volume: 80,
		},
	}
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "subtonic"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return writeDefault(path)
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func writeDefault(path string) (Config, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return Config{}, err
	}
	cfg := defaultConfig()
	f, err := os.Create(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	return cfg, toml.NewEncoder(f).Encode(cfg)
}
