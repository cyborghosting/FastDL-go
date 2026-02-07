package game

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyborghosting/fastdl/utils"
	keyvalues "github.com/galaco/KeyValues"
)

type SearchPathManager struct {
	items []searchPathItem
}

func NewSearchPathManager(root string, bases map[string]string, kv *keyvalues.KeyValue) (*SearchPathManager, error) {
	kv, err := utils.NewKeyValueTraverser(kv).Traverse("FileSystem", "SearchPaths")
	if err != nil {
		return nil, err
	}
	children, err := kv.Children()
	if err != nil {
		return nil, err
	}
	manager := &SearchPathManager{
		items: make([]searchPathItem, 0, len(children)),
	}
	parser := &SearchPathParser{
		installation: root,
		dictionary:   bases,
	}
	for _, child := range children {
		directory, ok := parser.Parse(child)
		if !ok {
			continue
		}
		utils.DEBUG("Adding search path directory: %s", directory)
		manager.items = append(manager.items, newSearchPathItem(directory))
	}
	return manager, nil
}

func (spm *SearchPathManager) Fetch(onUpdate func()) error {
	updated := false
	for i := range spm.items {
		err := spm.items[i].Fetch(func() {
			updated = true
		})
		if err != nil {
			utils.DEBUG("Warning: failed to fetch search path item: %v", err)
		}
	}
	if updated && onUpdate != nil {
		utils.DEBUG("SearchPathManager updated, invoking onUpdate callback")
		onUpdate()
	}
	return nil
}

func (spm *SearchPathManager) Get() []string {
	directories := make([]string, 0)
	for _, item := range spm.items {
		directories = append(directories, item.Get()...)
	}
	return directories
}

type searchPathItem interface {
	Fetch(onUpdate func()) error
	Get() []string
}

func newSearchPathItem(directory string) searchPathItem {
	if strings.HasSuffix(directory, "*") {
		return &wildcardSearchPathItem{
			directory: strings.TrimSuffix(directory, "*"),
			matches:   []matchInfo{},
		}
	} else {
		return &normalSearchPathItem{
			directory: directory,
		}
	}
}

type normalSearchPathItem struct {
	directory string
	modTime   time.Time
}

func (sp *normalSearchPathItem) Fetch(onUpdate func()) error {
	stat, err := os.Stat(sp.directory)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return nil
	}
	if sp.modTime.Equal(stat.ModTime()) {
		return nil
	}
	sp.modTime = stat.ModTime()
	if onUpdate != nil {
		utils.DEBUG("Normal search path at %s updated, invoking onUpdate callback", sp.directory)
		onUpdate()
	}
	return nil
}

func (sp *normalSearchPathItem) Get() []string {
	return []string{sp.directory}
}

type wildcardSearchPathItem struct {
	directory string
	matches   []matchInfo
}

type matchInfo struct {
	path    string
	modTime time.Time
}

func (sp *wildcardSearchPathItem) Fetch(onUpdate func()) error {
	dirEntries, err := os.ReadDir(sp.directory)
	if err != nil {
		return err
	}
	matches := make([]matchInfo, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		info, err := dirEntry.Info()
		if err != nil {
			utils.DEBUG("Warning: failed to get info for directory entry %s: %v", dirEntry.Name(), err)
			continue
		}
		matches = append(matches, matchInfo{
			path:    filepath.Join(sp.directory, dirEntry.Name()),
			modTime: info.ModTime(),
		})
	}
	updated := false
	if len(matches) != len(sp.matches) {
		updated = true
	} else {
		for i, match := range matches {
			if match.path != sp.matches[i].path || !match.modTime.Equal(sp.matches[i].modTime) {
				updated = true
				break
			}
		}
	}
	sp.matches = matches
	if updated && onUpdate != nil {
		utils.DEBUG("Wildcard search path at %s updated, invoking onUpdate callback", sp.directory)
		onUpdate()
	}
	return nil
}

func (sp *wildcardSearchPathItem) Get() []string {
	paths := make([]string, 0, len(sp.matches))
	for _, match := range sp.matches {
		paths = append(paths, match.path)
	}
	return paths
}
