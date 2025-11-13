package internal

import (
	"context"
	"log"
	"time"

	"sync"
	"sync/atomic"

	"github.com/spf13/afero"

	"github.com/cyborghosting/fastdl/internal/game"
)

// FileSystemManager manages the filesystem with automatic updates

type FileSystemManager interface {
	GetFileSystem() afero.Fs
	Close()
}

type fileSystemManager struct {
	root  string
	bases map[string]string

	fs  atomic.Pointer[afero.Fs]
	gim *game.GameInfoManager
	spm *game.SearchPathManager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewFileSystemManager(root string, bases map[string]string) (FileSystemManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	fsm := &fileSystemManager{
		root:  root,
		bases: bases,

		ctx:    ctx,
		cancel: cancel,
	}

	fsm.gim = game.NewGameInfo(root, bases)

	fsm.updateFileSystem()
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

func (fsm *fileSystemManager) updateFileSystem() {
	var err error

	err = fsm.gim.Fetch(func() {
		spm, err := game.NewSearchPathManager(fsm.root, fsm.bases, fsm.gim.KV)
		if err != nil {
			log.Printf("Failed to create search path manager: %v", err)
			return
		}
		fsm.spm = spm
	})
	if err != nil {
		log.Printf("Failed to fetch gameinfo: %v", err)
	}

	err = fsm.spm.Fetch(func() {
		newFs := buildOverlay(fsm.spm.Get())
		fsm.fs.Store(&newFs)
	})
	if err != nil {
		log.Printf("Failed to fetch search paths: %v", err)
	}
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
				fsm.updateFileSystem()
			}
		}
	}()
}

func buildOverlay(paths []string) afero.Fs {
	if len(paths) == 0 {
		return afero.NewReadOnlyFs(afero.NewMemMapFs())
	}

	osFs := afero.NewOsFs()

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
