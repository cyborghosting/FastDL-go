package chain

import (
	"os"
	"path/filepath"
	"time"

	keyvalues "github.com/galaco/KeyValues"
)

type GameInfoState struct {
	Updated bool
	KV      *keyvalues.KeyValue

	modTime time.Time
}

type ParseGameInfo struct {
	next Handler
}

func (g *ParseGameInfo) Handle(state *State) {
	if g.next != nil {
		defer g.next.Handle(state)
	}

	state.GameInfo.Updated = false

	_, ok := state.Dictionary["gameinfo_path"]
	if !ok {
		g.clear(state)
		return
	}

	p := filepath.Join(state.InstallationPath, state.Dictionary["gameinfo_path"], "gameinfo.txt")
	f, err := os.Open(p)
	if err != nil {
		g.clear(state)
		return
	}
	defer f.Close()

	s, err := f.Stat()
	if err != nil {
		g.clear(state)
		return
	}

	if !s.ModTime().After(state.GameInfo.modTime) {
		return
	}

	r := keyvalues.NewReader(f)
	kv, err := r.Read()
	if err != nil {
		g.clear(state)
		return
	}

	state.GameInfo.Updated = true
	state.GameInfo.modTime = s.ModTime()
	state.GameInfo.KV = &kv
}

func (g *ParseGameInfo) SetNext(next Handler) {
	g.next = next
}

func (g *ParseGameInfo) clear(state *State) {
	state.GameInfo.Updated = true
	state.GameInfo.modTime = time.Time{}
	state.GameInfo.KV = nil
}
