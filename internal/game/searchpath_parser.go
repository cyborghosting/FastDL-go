package game

import (
	"log"
	"path/filepath"
	"slices"
	"strings"

	keyvalues "github.com/galaco/KeyValues"
)

type SearchPathParser struct {
	installation string
	dictionary   map[string]string
}

func NewSearchPathParser(installation string, dictionary map[string]string) *SearchPathParser {
	return &SearchPathParser{
		installation: installation,
		dictionary:   dictionary,
	}
}

func (p *SearchPathParser) Parse(entry *keyvalues.KeyValue) (string, bool) {
	keys, valid := p.parseKeys(entry)
	if !valid || slices.Contains(keys, "vpk") || !slices.Contains(keys, "mod") {
		return "", false
	}

	directory, valid := p.parseDirectory(entry)
	if !valid || strings.HasSuffix(directory, ".vpk") {
		return "", false
	}

	return directory, true
}

func (p *SearchPathParser) parseKeys(kv *keyvalues.KeyValue) ([]string, bool) {
	return strings.Split(kv.Key(), "+"), true
}

func (p *SearchPathParser) parseDirectory(kv *keyvalues.KeyValue) (string, bool) {
	s, err := kv.AsString()
	if err != nil {
		return "", false
	}

	if !strings.HasPrefix(s, "|") {
		return filepath.Join(p.installation, s), true
	}

	i := strings.Index(s[1:], "|") + 1
	if i == 0 {
		return "", false
	}

	baseDirKey := s[1:i]
	baseDir, ok := p.dictionary[baseDirKey]
	if !ok {
		log.Printf("Warning: unknown directory key '%s' in search path\n", baseDirKey)
		return "", false
	}
	relPath := s[i+1:]

	path := filepath.Join(p.installation, baseDir, relPath)
	return path, true
}
