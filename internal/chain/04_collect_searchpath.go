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

type Collection struct {
	ModificationTime time.Time
	Locations        []string
}

type CollectSearchPathState struct {
	Updated bool

	Collections []Collection
}

type CollectSearchPath struct {
	next Handler
}

func (c *CollectSearchPath) Handle(state *State) {
	if c.next != nil {
		defer c.next.Handle(state)
	}

	state.CollectSearchPath.Updated = false

	if state.FilterSearchPath.Locations == nil {
		c.clear(state)
		return
	}

	collections := make([]Collection, 0, len(state.FilterSearchPath.Locations))
	for _, loc := range state.FilterSearchPath.Locations {
		collection := c.collectLocations(loc)
		collections = append(collections, collection)
	}

	if state.FilterSearchPath.Updated || len(collections) != len(state.CollectSearchPath.Collections) {
		state.CollectSearchPath.Updated = true
		state.CollectSearchPath.Collections = collections
		return
	}

	for i := 0; i < len(collections); i++ {
		if collections[i].ModificationTime != state.CollectSearchPath.Collections[i].ModificationTime {
			state.CollectSearchPath.Updated = true
			break
		}
		if len(collections[i].Locations) != len(state.CollectSearchPath.Collections[i].Locations) {
			state.CollectSearchPath.Updated = true
			break
		}
		for j := 0; j < len(collections[i].Locations); j++ {
			if collections[i].Locations[j] != state.CollectSearchPath.Collections[i].Locations[j] {
				state.CollectSearchPath.Updated = true
				break
			}
		}
		if state.CollectSearchPath.Updated {
			break
		}
	}

	state.CollectSearchPath.Collections = collections
}

func (c *CollectSearchPath) SetNext(next Handler) {
	c.next = next
}

func (c *CollectSearchPath) clear(state *State) {
	state.CollectSearchPath.Updated = true
	state.CollectSearchPath.Collections = nil
}

func (c *CollectSearchPath) collectLocations(loc string) Collection {
	dir := filepath.Dir(loc)
	base := filepath.Base(loc)

	if strings.ContainsAny(dir, `*?`) {
		return Collection{}
	}

	if !strings.ContainsAny(base, `*?`) {
		s, err := os.Stat(filepath.Join(dir, base))
		if err != nil || !s.Mode().IsDir() {
			return Collection{}
		}
		return Collection{
			Locations: []string{loc},
		}
	}

	s, err := os.Stat(dir)
	if err != nil || !s.Mode().IsDir() {
		return Collection{}
	}
	modificationTime := s.ModTime()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return Collection{}
	}

	p := buildWildcardRegex(base)
	re, err := regexp.Compile(p)
	if err != nil {
		return Collection{}
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

	return Collection{
		ModificationTime: modificationTime,
		Locations:        locations,
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
