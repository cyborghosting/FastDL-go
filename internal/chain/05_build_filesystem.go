package chain

import (
	"github.com/spf13/afero"
)

type BuildFileSystemState struct {
	Updated    bool
	FileSystem afero.Fs
}

type BuildFileSystem struct {
	next Handler
}

func (b *BuildFileSystem) Handle(state *State) {
	if b.next != nil {
		defer b.next.Handle(state)
	}

	state.BuildFileSystem.Updated = false

	if !state.CollectSearchPath.Updated && state.BuildFileSystem.FileSystem != nil {
		return
	}

	state.BuildFileSystem.Updated = true

	paths := flatten(state.CollectSearchPath.Collections)

	if len(paths) == 0 {
		state.BuildFileSystem.FileSystem = afero.NewReadOnlyFs(afero.NewMemMapFs())
		return
	}

	osFs := afero.NewOsFs()

	fileSystems := make([]afero.Fs, 0, len(paths))
	for _, path := range paths {
		fileSystems = append(fileSystems, afero.NewBasePathFs(osFs, path))
	}

	if len(fileSystems) == 1 {
		state.BuildFileSystem.FileSystem = afero.NewReadOnlyFs(fileSystems[0])
		return
	}

	// Build overlay from last to first
	fs := fileSystems[len(fileSystems)-1]
	for i := len(fileSystems) - 2; i >= 0; i-- {
		fs = afero.NewCopyOnWriteFs(fs, fileSystems[i])
	}

	state.BuildFileSystem.FileSystem = afero.NewReadOnlyFs(fs)
}

func (b *BuildFileSystem) SetNext(next Handler) {
	b.next = next
}

func flatten(in []Collection) []string {
	n := 0
	for _, s := range in {
		n += len(s.Locations)
	}
	out := make([]string, 0, n)
	for _, s := range in {
		out = append(out, s.Locations...)
	}
	return out
}
