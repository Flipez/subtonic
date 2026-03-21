package ui

import "charm.land/bubbles/v2/key"

type globalKeys struct {
	Quit      key.Binding
	PlayPause key.Binding
	Next      key.Binding
	Prev      key.Binding
	VolUp      key.Binding
	VolDown    key.Binding
	VolUpLarge key.Binding
	VolDownLarge key.Binding
	Search    key.Binding
	Back      key.Binding
	Tab1      key.Binding
	Tab2      key.Binding
	Tab3      key.Binding
	Tab4      key.Binding
	Tab5      key.Binding
	Tab6      key.Binding
	Refresh   key.Binding
	Shuffle   key.Binding
	Repeat    key.Binding
	Star      key.Binding
	Queue     key.Binding
	Delete    key.Binding
	MoveDown    key.Binding
	MoveUp      key.Binding
	ShufflePlay key.Binding
	PlayNext    key.Binding
	AddTo       key.Binding
	NewPlaylist key.Binding
	SonosToggle  key.Binding
	HelpToggle   key.Binding
	QuickActions key.Binding
}

var GlobalKeys = globalKeys{
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	PlayPause: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "play/pause")),
	Next:      key.NewBinding(key.WithKeys(">", "."), key.WithHelp(">", "next")),
	Prev:      key.NewBinding(key.WithKeys("<", ","), key.WithHelp("<", "prev")),
	VolUp:        key.NewBinding(key.WithKeys("="), key.WithHelp("=", "vol+1")),
	VolDown:      key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "vol-1")),
	VolUpLarge:   key.NewBinding(key.WithKeys("+"), key.WithHelp("+", "vol+5")),
	VolDownLarge: key.NewBinding(key.WithKeys("_"), key.WithHelp("_", "vol-5")),
	Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Tab1:      key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "discover")),
	Tab2:      key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "artists")),
	Tab3:      key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "albums")),
	Tab4:      key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "playlists")),
	Tab5:      key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "podcasts")),
	Tab6:      key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "search")),
	Refresh:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
	Shuffle:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shuffle")),
	Repeat:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "repeat")),
	Star:      key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "star")),
	Queue:     key.NewBinding(key.WithKeys("Q"), key.WithHelp("Q", "queue")),
	Delete:    key.NewBinding(key.WithKeys("d", "delete"), key.WithHelp("d", "delete/remove")),
	MoveDown:    key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "move down")),
	MoveUp:      key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "move up")),
	ShufflePlay: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "shuffle play")),
	PlayNext:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "play next")),
	AddTo:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add to playlist")),
	NewPlaylist: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new playlist")),
	SonosToggle:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "toggle sonos output")),
	HelpToggle:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	QuickActions: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "quick actions")),
}
