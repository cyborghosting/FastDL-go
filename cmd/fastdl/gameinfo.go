package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	keyvalues "github.com/galaco/KeyValues"
)

type GameInfo struct {
	BasePath    string
	Directories map[string]string
	Path        string
	stat        os.FileInfo
	kv          *keyvalues.KeyValue
}

func NewGameInfo(basePath string, directories map[string]string) (*GameInfo, error) {
	if basePath == "" {
		return nil, errors.New("basePath cannot be empty")
	}
	if directories == nil {
		return nil, errors.New("directories cannot be nil")
	}

	path, err := buildGameInfoPath(basePath, directories)
	if err != nil {
		return nil, fmt.Errorf("failed to build gameinfo path: %w", err)
	}

	return &GameInfo{
		BasePath:    basePath,
		Directories: directories,
		Path:        path,
	}, nil
}

func (g *GameInfo) IsChanged() (bool, error) {
	stat, err := g.statFile()
	if err != nil {
		return false, err
	}

	// Check if file has been modified
	if g.stat != nil && g.stat.ModTime().Equal(stat.ModTime()) {
		return false, nil
	}

	g.stat = stat

	kv, err := g.parseGameInfo()
	if err != nil {
		return false, fmt.Errorf("failed to parse gameinfo: %w", err)
	}

	g.kv = kv
	return true, nil
}

func (g *GameInfo) GetKeyValues() *keyvalues.KeyValue {
	return g.kv
}

func buildGameInfoPath(basePath string, directories map[string]string) (string, error) {
	const gameInfoFile = "gameinfo.txt"

	subPath, exists := directories["gameinfo_path"]
	if !exists {
		return "", errors.New("directory 'gameinfo_path' not found in path mapping")
	}

	if subPath == "" {
		return "", errors.New("gameinfo_path cannot be empty")
	}

	return filepath.Join(basePath, subPath, gameInfoFile), nil
}

func (g *GameInfo) statFile() (os.FileInfo, error) {
	stat, err := os.Stat(g.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("gameinfo file does not exist: %s", g.Path)
		}
		if errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("permission denied accessing gameinfo file: %s", g.Path)
		}
		return nil, fmt.Errorf("failed to stat gameinfo file %s: %w", g.Path, err)
	}
	return stat, nil
}

func (g *GameInfo) parseGameInfo() (*keyvalues.KeyValue, error) {
	file, err := os.Open(g.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open gameinfo file %s: %w", g.Path, err)
	}
	defer file.Close()

	reader := keyvalues.NewReader(file)
	kv, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to parse gameinfo file: %w", err)
	}

	return &kv, nil
}
