package chain

import (
	"slices"
	"strings"
)

type FilteredSearchPathState struct {
	Updated   bool
	Locations []string
}

type FilterSearchPath struct {
	next Handler
}

func (f *FilterSearchPath) Handle(state *State) {
	if f.next != nil {
		defer f.next.Handle(state)
	}

	state.FilteredSearchPath.Updated = false

	if !state.SearchPath.Updated && state.FilteredSearchPath.Locations != nil {
		return
	}

	if state.SearchPath.SearchPaths == nil {
		f.clear(state)
		return
	}

	locations := make([]string, 0, len(state.SearchPath.SearchPaths))
	for _, sp := range state.SearchPath.SearchPaths {
		if slices.Contains(sp.IDs, "vpk") || !slices.Contains(sp.IDs, "mod") {
			continue
		}

		if strings.HasSuffix(sp.Location, ".vpk") {
			continue
		}

		locations = append(locations, sp.Location)
	}

	state.FilteredSearchPath.Updated = true
	state.FilteredSearchPath.Locations = locations
}

func (f *FilterSearchPath) SetNext(next Handler) {
	f.next = next
}

func (f *FilterSearchPath) clear(state *State) {
	state.FilteredSearchPath.Updated = true
	state.FilteredSearchPath.Locations = nil
}
