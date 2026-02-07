package chain

import (
	"os"
	"path/filepath"
	"time"

	keyvalues "github.com/galaco/KeyValues"
)

type ParseGameInfoState struct {
	Updated bool

	ModificationTime time.Time
	KV               *keyvalues.KeyValue
}

type ParseGameInfo struct {
	next Handler
}

func (g *ParseGameInfo) Handle(state *State) {
	if g.next != nil {
		defer g.next.Handle(state)
	}

	state.ParseGameInfo.Updated = false

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

	t := s.ModTime()
	if !t.After(state.ParseGameInfo.ModificationTime) {
		return
	}

	r := keyvalues.NewReader(f)
	kv, err := r.Read()
	if err != nil {
		g.clear(state)
		return
	}

	state.ParseGameInfo.Updated = true
	state.ParseGameInfo.ModificationTime = t
	state.ParseGameInfo.KV = &kv
}

func (g *ParseGameInfo) SetNext(next Handler) {
	g.next = next
}

func (g *ParseGameInfo) clear(state *State) {
	state.ParseGameInfo.Updated = true
	state.ParseGameInfo.ModificationTime = time.Time{}
	state.ParseGameInfo.KV = nil
}
