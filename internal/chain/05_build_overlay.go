package chain

import (
	"slices"

	"github.com/spf13/afero"
)

type OverlayFsState struct {
	Updated bool
	Fs      afero.Fs
}

type BuildOverlayFs struct {
	next Handler
}

func (b *BuildOverlayFs) Handle(state *State) {
	if b.next != nil {
		defer b.next.Handle(state)
	}

	state.OverlayFs.Updated = false

	if !state.ResolvedSearchPath.Updated && state.OverlayFs.Fs != nil {
		return
	}

	state.OverlayFs.Updated = true

	resolutions := flatten(state.ResolvedSearchPath.Resolutions)

	osFs := afero.NewOsFs()

	fs := afero.NewMemMapFs()

	for i, path := range slices.Backward(resolutions) {
		if i == len(resolutions)-1 {
			fs = afero.NewBasePathFs(osFs, path)
		} else {
			fs = afero.NewCopyOnWriteFs(fs, afero.NewBasePathFs(osFs, path))
		}
	}

	state.OverlayFs.Fs = afero.NewReadOnlyFs(fs)
}

func (b *BuildOverlayFs) SetNext(next Handler) {
	b.next = next
}

func flatten(in []Resolution) []string {
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
