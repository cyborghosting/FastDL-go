package internal

import (
	"context"
	"time"

	"sync"
	"sync/atomic"

	"github.com/spf13/afero"

	"github.com/cyborghosting/fastdl/internal/chain"
)

// FileSystemManager manages the filesystem with automatic updates

type FileSystemManager interface {
	GetFileSystem() afero.Fs
	Close()
}

type fileSystemManager struct {
	installation string
	dictionary   map[string]string

	chainState  *chain.State
	chainHandle func(*chain.State)

	fileSystem atomic.Pointer[afero.Fs]

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewFileSystemManager(installation string, dictionary map[string]string) (FileSystemManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	f := &fileSystemManager{
		installation: installation,
		dictionary:   dictionary,

		ctx:    ctx,
		cancel: cancel,
	}

	buildFileSystem := &chain.BuildFileSystem{}

	collectSearchPath := &chain.CollectSearchPath{}
	collectSearchPath.SetNext(buildFileSystem)

	filterSearchPath := &chain.FilterSearchPath{}
	filterSearchPath.SetNext(collectSearchPath)

	parseSearchPath := &chain.ParseSearchPath{}
	parseSearchPath.SetNext(filterSearchPath)

	parseGameInfo := &chain.ParseGameInfo{}
	parseGameInfo.SetNext(parseSearchPath)

	state := &chain.State{
		InstallationPath: installation,
		Dictionary:       dictionary,
	}

	f.chainState = state
	f.chainHandle = parseGameInfo.Handle

	f.updateFileSystem()
	f.startMonitoring()

	return f, nil
}

func (f *fileSystemManager) GetFileSystem() afero.Fs {
	return *f.fileSystem.Load()
}

func (f *fileSystemManager) Close() {
	f.cancel()
	f.wg.Wait()
}

func (f *fileSystemManager) updateFileSystem() {
	f.chainHandle(f.chainState)

	fs := f.chainState.BuildFileSystem.FileSystem
	f.fileSystem.Store(&fs)
}

func (f *fileSystemManager) startMonitoring() {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-f.ctx.Done():
				return
			case <-ticker.C:
				f.updateFileSystem()
			}
		}
	}()
}
