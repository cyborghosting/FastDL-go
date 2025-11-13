package game

import (
	"log"
	"path/filepath"
	"slices"
	"strings"

	keyvalues "github.com/galaco/KeyValues"
)

type searchPathParser struct {
	rootDir  string
	baseDirs map[string]string
}

func (p *searchPathParser) Parse(entry *keyvalues.KeyValue) (string, bool) {
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

func (p *searchPathParser) parseKeys(kv *keyvalues.KeyValue) ([]string, bool) {
	return strings.Split(kv.Key(), "+"), true
}

func (p *searchPathParser) parseDirectory(kv *keyvalues.KeyValue) (string, bool) {
	s, err := kv.AsString()
	if err != nil {
		return "", false
	}

	if !strings.HasPrefix(s, "|") {
		return filepath.Join(p.rootDir, s), true
	}

	i := strings.Index(s[1:], "|") + 1
	if i == 0 {
		return "", false
	}

	baseDirKey := s[1:i]
	baseDir, ok := p.baseDirs[baseDirKey]
	if !ok {
		log.Printf("Warning: unknown directory key '%s' in search path\n", baseDirKey)
		return "", false
	}
	relPath := s[i+1:]

	path := filepath.Join(p.rootDir, baseDir, relPath)
	return path, true
}
