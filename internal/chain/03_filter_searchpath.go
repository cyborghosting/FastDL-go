package chain

import (
	"slices"
	"strings"
)

type FilterSearchPathState struct {
	Updated bool

	Locations []string
}

type FilterSearchPath struct {
	next Handler
}

func (f *FilterSearchPath) Handle(state *State) {
	if f.next != nil {
		defer f.next.Handle(state)
	}

	state.FilterSearchPath.Updated = false

	if !state.ParseSearchPath.Updated && state.FilterSearchPath.Locations != nil {
		return
	}

	if state.ParseSearchPath.SearchPaths == nil {
		f.clear(state)
		return
	}

	locations := make([]string, 0, len(state.ParseSearchPath.SearchPaths))
	for _, sp := range state.ParseSearchPath.SearchPaths {
		if slices.Contains(sp.IDs, "vpk") || !slices.Contains(sp.IDs, "mod") {
			continue
		}

		if strings.HasSuffix(sp.Location, ".vpk") {
			continue
		}

		locations = append(locations, sp.Location)
	}

	state.FilterSearchPath.Updated = true
	state.FilterSearchPath.Locations = locations
}

func (f *FilterSearchPath) SetNext(next Handler) {
	f.next = next
}

func (f *FilterSearchPath) clear(state *State) {
	state.FilterSearchPath.Updated = true
	state.FilterSearchPath.Locations = nil
}
