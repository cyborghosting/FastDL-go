package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	keyvalues "github.com/galaco/KeyValues"
)

type SearchPath struct {
	path     string
	wildcard bool
	stat     os.FileInfo
}

type SearchPaths struct {
	items       []SearchPath
	initialized bool
}

func NewSearchPaths(gi *GameInfo) (*SearchPaths, error) {
	if gi.GetKeyValues() == nil {
		return nil, errors.New("nil keyvalues.KeyValue provided")
	}

	fileSystemKV, err := gi.GetKeyValues().Find("FileSystem")
	if err != nil {
		return nil, fmt.Errorf("failed to find FileSystem in gameinfo: %w", err)
	}

	searchPathsKV, err := fileSystemKV.Find("SearchPaths")
	if err != nil {
		return nil, fmt.Errorf("failed to find FileSystem.SearchPaths in gameinfo: %w", err)
	}

	children, err := searchPathsKV.Children()
	if err != nil {
		return nil, fmt.Errorf("failed to get children of FileSystem.SearchPaths in gameinfo: %w", err)
	}

	searchPaths := &SearchPaths{items: make([]SearchPath, 0, len(children))}

	for _, child := range children {
		if err := processChild(child, gi.BasePath, gi.Directories, searchPaths); err != nil {
			// Log error but continue processing other paths
			fmt.Printf("Warning: failed to process search path: %v\n", err)
		}
	}

	return searchPaths, nil
}

func processChild(child *keyvalues.KeyValue, basePath string, directories map[string]string, searchPaths *SearchPaths) error {
	keys := strings.Split(child.Key(), "+")

	path, err := child.AsString()
	if err != nil {
		return fmt.Errorf("failed to get directory from SearchPath in gameinfo: %s", err)
	}

	if !slices.Contains(keys, "mod") || slices.Contains(keys, "vpk") || strings.HasSuffix(path, ".vpk") {
		return nil
	}

	path, err = translate(path, directories)
	if err != nil {
		return fmt.Errorf("failed to resolve directory in SearchPath in gameinfo: %s", err)
	}

	wildcard := strings.HasSuffix(path, "*")
	if wildcard {
		path = strings.TrimSuffix(path, "*")
	}

	searchPaths.items = append(searchPaths.items, SearchPath{
		path:     filepath.Join(basePath, path),
		wildcard: wildcard,
		stat:     nil,
	})

	return nil
}

func translate(raw_directory string, directories map[string]string) (string, error) {
	if !strings.HasPrefix(raw_directory, "|") {
		return filepath.Clean(raw_directory), nil
	}

	i := strings.Index(raw_directory[1:], "|") + 1
	if i == 0 {
		return "", fmt.Errorf("invalid search path format: %s", raw_directory)
	}

	directoryKey := raw_directory[1:i]
	directory, ok := directories[directoryKey]
	if !ok {
		return "", fmt.Errorf("directory '%s' not found", directoryKey)
	}

	return filepath.Join(directory, raw_directory[i+1:]), nil
}

func (sp *SearchPath) IsChanged() bool {
	if !sp.wildcard {
		return false
	}

	stat, _ := os.Stat(sp.path)

	defer func() { sp.stat = stat }()

	nilBefore := sp.stat == nil
	nilAfter := stat == nil

	if nilBefore && nilAfter {
		return false
	}

	if nilBefore || nilAfter {
		return true
	}

	if sp.stat.ModTime().Equal(stat.ModTime()) {
		return false
	}

	return true
}

func (sp *SearchPaths) AreChanged() bool {
	result := false

	if !sp.initialized {
		sp.initialized = true
		result = true
	}

	for i := range sp.items {
		if sp.items[i].IsChanged() {
			result = true
		}
	}

	return result
}

func (sp *SearchPaths) Resolve() []string {
	var resolvedPaths []string

	for _, sp := range sp.items {
		stat, err := os.Stat(sp.path)
		if err != nil || !stat.IsDir() {
			continue
		}

		if !sp.wildcard {
			resolvedPaths = append(resolvedPaths, sp.path)
			continue
		}

		entries, err := os.ReadDir(sp.path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			path := filepath.Join(sp.path, entry.Name())
			stat, err := os.Stat(path)
			if err == nil && stat.IsDir() {
				resolvedPaths = append(resolvedPaths, path)
			}
		}
	}

	return resolvedPaths
}
