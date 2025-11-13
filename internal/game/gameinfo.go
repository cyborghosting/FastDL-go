package game

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/cyborghosting/fastdl/utils"
	keyvalues "github.com/galaco/KeyValues"
)

type GameInfoManager struct {
	path string

	modTime time.Time

	KV *keyvalues.KeyValue
}

func NewGameInfo(root string, bases map[string]string) *GameInfoManager {
	return &GameInfoManager{
		path: path.Join(root, bases["gameinfo_path"], "gameinfo.txt"),
	}
}

func (gi *GameInfoManager) Fetch(onUpdate func()) error {
	info, err := os.Stat(gi.path)
	if err != nil {
		return fmt.Errorf("failed to stat gameinfo file %s: %w", gi.path, err)
	}
	if info.ModTime().Equal(gi.modTime) {
		return nil
	}
	gi.modTime = info.ModTime()

	f, err := os.Open(gi.path)
	if err != nil {
		return fmt.Errorf("failed to open gameinfo file %s: %w", gi.path, err)
	}
	defer f.Close()

	r := keyvalues.NewReader(f)
	kv, err := r.Read()
	if err != nil {
		return fmt.Errorf("failed to parse gameinfo file: %w", err)
	}

	gi.KV = &kv
	if onUpdate != nil {
		utils.DEBUG("GameInfo at %s updated, invoking onUpdate callback", gi.path)
		onUpdate()
	}
	return nil
}
