package main

import (
	"context"
	"log"
	"time"

	"sync"
	"sync/atomic"

	"github.com/spf13/afero"
)

// FileSystemManager manages the filesystem with automatic updates

type FileSystemManager interface {
	GetFileSystem() afero.Fs
	Close()
}

type fileSystemManager struct {
	fs          atomic.Pointer[afero.Fs]
	gameInfo    *GameInfo
	searchPaths *SearchPaths

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewFileSystemManager(basePath string, directories map[string]string) (FileSystemManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	fsm := &fileSystemManager{
		ctx:    ctx,
		cancel: cancel,
	}

	var err error
	fsm.gameInfo, err = NewGameInfo(basePath, directories)
	if err != nil {
		cancel()
		return nil, err
	}

	if err := fsm.updateFileSystem(); err != nil {
		cancel()
		return nil, err
	}

	fsm.startMonitoring()
	return fsm, nil
}

func (fsm *fileSystemManager) GetFileSystem() afero.Fs {
	return *fsm.fs.Load()
}

func (fsm *fileSystemManager) Close() {
	fsm.cancel()
	fsm.wg.Wait()
}

func (fsm *fileSystemManager) updateFileSystem() error {
	changed, err := fsm.gameInfo.IsChanged()
	if err != nil {
		return err
	}

	if changed {
		fsm.searchPaths, err = NewSearchPaths(fsm.gameInfo)
		if err != nil {
			return err
		}
	}

	if fsm.searchPaths.AreChanged() {
		newFs := buildOverlay(fsm.searchPaths)
		fsm.fs.Store(&newFs)
	}

	return nil
}

func (fsm *fileSystemManager) startMonitoring() {
	fsm.wg.Add(1)
	go func() {
		defer fsm.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-fsm.ctx.Done():
				return
			case <-ticker.C:
				if err := fsm.updateFileSystem(); err != nil {
					log.Printf("Error updating filesystem: %v", err)
				}
			}
		}
	}()
}

func buildOverlay(searchPaths *SearchPaths) afero.Fs {
	osFs := afero.NewOsFs()
	paths := searchPaths.Resolve()

	if len(paths) == 0 {
		return afero.NewReadOnlyFs(afero.NewMemMapFs())
	}

	fileSystems := make([]afero.Fs, len(paths))
	for i, path := range paths {
		fileSystems[i] = afero.NewBasePathFs(osFs, path)
	}

	if len(fileSystems) == 1 {
		return afero.NewReadOnlyFs(fileSystems[0])
	}

	// Build overlay from last to first
	fs := fileSystems[len(fileSystems)-1]
	for i := len(fileSystems) - 2; i >= 0; i-- {
		fs = afero.NewCopyOnWriteFs(fs, fileSystems[i])
	}

	return afero.NewReadOnlyFs(fs)
}
