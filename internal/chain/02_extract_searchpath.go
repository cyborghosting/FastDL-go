package chain

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cyborghosting/fastdl/utils"
	keyvalues "github.com/galaco/KeyValues"
)

type SearchPath struct {
	IDs      []string
	Location string
}

type SearchPathState struct {
	Updated     bool
	SearchPaths []*SearchPath
}

type ExtractSearchPath struct {
	next Handler
}

func (s *ExtractSearchPath) Handle(state *State) {
	if s.next != nil {
		defer s.next.Handle(state)
	}

	state.SearchPath.Updated = false

	if !state.GameInfo.Updated && state.SearchPath.SearchPaths != nil {
		return
	}

	if state.GameInfo.KV == nil {
		s.clear(state)
		return
	}

	kv, err := utils.TraverseKeyValue(state.GameInfo.KV, "FileSystem", "SearchPaths")
	if err != nil {
		s.clear(state)
		return
	}

	children, err := kv.Children()
	if err != nil {
		s.clear(state)
		return
	}

	parser := &searchPathParser{
		installation: state.InstallationPath,
		dictionary:   state.Dictionary,
	}

	searchPaths := make([]*SearchPath, 0, len(children))

	for _, child := range children {
		sp, err := parser.Parse(child)
		if err != nil {
			continue
		}
		searchPaths = append(searchPaths, sp)
	}

	state.SearchPath.Updated = true
	state.SearchPath.SearchPaths = searchPaths
}

func (s *ExtractSearchPath) SetNext(next Handler) {
	s.next = next
}

func (s *ExtractSearchPath) clear(state *State) {
	state.SearchPath.Updated = true
	state.SearchPath.SearchPaths = nil
}

type searchPathParser struct {
	installation string
	dictionary   map[string]string
}

func (p *searchPathParser) Parse(kv *keyvalues.KeyValue) (*SearchPath, error) {
	ids, err := p.parseIDs(kv)
	if err != nil {
		return nil, err
	}

	location, err := p.parseLocation(kv)
	if err != nil {
		return nil, err
	}

	return &SearchPath{
		IDs:      ids,
		Location: location,
	}, nil
}

func (p *searchPathParser) parseIDs(kv *keyvalues.KeyValue) ([]string, error) {
	k := kv.Key()
	in := strings.Split(k, "+")
	out := make([]string, 0, len(in))
	for _, id := range in {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		id = strings.ToLower(id)
		out = append(out, id)
	}
	return out, nil
}

const locationPattern = `^(?:\|([^|]+)\|)?(.*)$`

var locationRegexp = regexp.MustCompile(locationPattern)

func (p *searchPathParser) parseLocation(kv *keyvalues.KeyValue) (string, error) {
	v, err := kv.AsString()
	if err != nil {
		return "", err
	}

	m := locationRegexp.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("invalid location format: %s", v)
	}

	token := m[1]
	location := m[2]

	if filepath.IsAbs(location) {
		return filepath.Clean(location), nil
	}

	if token == "" {
		return filepath.Join(p.installation, location), nil
	}

	baseDirectory, ok := p.dictionary[token]
	if !ok {
		return "", fmt.Errorf("unknown token: %s", token)
	}

	if filepath.IsAbs(baseDirectory) {
		return filepath.Join(baseDirectory, location), nil
	}

	return filepath.Join(p.installation, baseDirectory, location), nil
}
