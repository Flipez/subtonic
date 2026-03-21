package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/Flipez/subtonic/api"
	"github.com/Flipez/subtonic/config"
	"github.com/Flipez/subtonic/listenbrainz"
	"github.com/Flipez/subtonic/player"
	"github.com/Flipez/subtonic/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	client := api.NewClient(cfg.Server.URL, cfg.Server.Username, cfg.Server.Password)

	if err := client.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "server ping failed: %v\n", err)
		os.Exit(1)
	}

	pl, err := player.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "player init: %v\n", err)
		os.Exit(1)
	}
	pl.SetVolume(cfg.Player.Volume)

	lb := listenbrainz.NewClient(cfg.ListenBrainz.Token, cfg.ListenBrainz.Username)

	model := ui.New(client, pl, cfg, lb)
	prog := tea.NewProgram(model)
	pl.SetProgram(prog)

	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}
