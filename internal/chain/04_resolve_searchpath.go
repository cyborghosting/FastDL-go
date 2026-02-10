package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cyborghosting/fastdl/utils"
)

type Resolution struct {
	Locations []string

	modTime time.Time
}

type ResolvedSearchPathState struct {
	Updated     bool
	Resolutions []Resolution
}

type ResolveSearchPath struct {
	next Handler
}

func (c *ResolveSearchPath) Handle(state *State) {
	if c.next != nil {
		defer c.next.Handle(state)
	}

	state.ResolvedSearchPath.Updated = false

	if state.FilteredSearchPath.Locations == nil {
		c.clear(state)
		return
	}

	resolutions := make([]Resolution, 0, len(state.FilteredSearchPath.Locations))
	for _, loc := range state.FilteredSearchPath.Locations {
		resolution := c.collectLocations(loc)
		resolutions = append(resolutions, resolution)
	}

	defer func() {
		state.ResolvedSearchPath.Resolutions = resolutions
	}()

	if state.FilteredSearchPath.Updated || len(resolutions) != len(state.ResolvedSearchPath.Resolutions) {
		state.ResolvedSearchPath.Updated = true
		return
	}

	for i := 0; i < len(resolutions); i++ {
		lhs, rhs := resolutions[i], state.ResolvedSearchPath.Resolutions[i]
		if lhs.modTime != rhs.modTime {
			state.ResolvedSearchPath.Updated = true
			break
		}
		if len(lhs.Locations) != len(rhs.Locations) {
			state.ResolvedSearchPath.Updated = true
			break
		}
		for j := 0; j < len(lhs.Locations); j++ {
			if lhs.Locations[j] != rhs.Locations[j] {
				state.ResolvedSearchPath.Updated = true
				break
			}
		}
		if state.ResolvedSearchPath.Updated {
			break
		}
	}
}

func (c *ResolveSearchPath) SetNext(next Handler) {
	c.next = next
}

func (c *ResolveSearchPath) clear(state *State) {
	state.ResolvedSearchPath.Updated = true
	state.ResolvedSearchPath.Resolutions = nil
}

func (c *ResolveSearchPath) collectLocations(loc string) Resolution {
	dir := filepath.Dir(loc)
	base := filepath.Base(loc)

	if strings.ContainsAny(dir, `*?`) {
		return Resolution{}
	}

	if !strings.ContainsAny(base, `*?`) {
		s, err := os.Stat(filepath.Join(dir, base))
		if err != nil || !s.Mode().IsDir() {
			return Resolution{}
		}
		return Resolution{
			Locations: []string{loc},
		}
	}

	s, err := os.Stat(dir)
	if err != nil || !s.Mode().IsDir() {
		return Resolution{}
	}
	modificationTime := s.ModTime()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return Resolution{}
	}

	p := buildWildcardRegex(base)
	re, err := regexp.Compile(p)
	if err != nil {
		return Resolution{}
	}

	locations := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if re.MatchString(entry.Name()) {
			locations = append(locations, filepath.Join(dir, entry.Name()))
		}
	}

	return Resolution{
		modTime:   modificationTime,
		Locations: locations,
	}
}

func buildWildcardRegex(pattern string) string {
	if pattern == "*.*" {
		return `(?i)^.*$`
	}

	var sb strings.Builder
	sb.WriteString(`(?i)^`)

	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '*':
			if i+1 < len(runes) {
				next := utils.RegexpQuoteRune(runes[i+1])
				fmt.Fprintf(&sb, `[^%s]*%s`, next, next)
				i++
			} else {
				sb.WriteString(`.*`)
			}
		case '?':
			sb.WriteString(`.`)
		default:
			sb.WriteString(utils.RegexpQuoteRune(runes[i]))
		}
	}
	sb.WriteString(`$`)
	return sb.String()
}
