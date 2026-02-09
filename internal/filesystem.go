package internal

import (
	"context"
	"time"

	"sync"
	"sync/atomic"

	"github.com/spf13/afero"

	"github.com/cyborghosting/fastdl/config"
	"github.com/cyborghosting/fastdl/internal/chain"
)

// FileSystemManager manages the filesystem with automatic updates

type FileSystemManager interface {
	GetFileSystem() afero.Fs
	Close()
}

type fileSystemManager struct {
	chainState  *chain.State
	chainHandle func(*chain.State)

	fileSystem atomic.Pointer[afero.Fs]

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewFileSystemManager(server config.Server) (FileSystemManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	f := &fileSystemManager{
		ctx:    ctx,
		cancel: cancel,
	}

	cacheFileSystem := &chain.CacheFileSystem{}

	buildFileSystem := &chain.BuildFileSystem{}
	buildFileSystem.SetNext(cacheFileSystem)

	collectSearchPath := &chain.CollectSearchPath{}
	collectSearchPath.SetNext(buildFileSystem)

	filterSearchPath := &chain.FilterSearchPath{}
	filterSearchPath.SetNext(collectSearchPath)

	parseSearchPath := &chain.ParseSearchPath{}
	parseSearchPath.SetNext(filterSearchPath)

	parseGameInfo := &chain.ParseGameInfo{}
	parseGameInfo.SetNext(parseSearchPath)

	state := &chain.State{
		InstallationPath: server.GetInstallationPath(),
		Dictionary:       server.GetDictionary(),
		CachePath:        server.GetCachePath(),
		CompressMaxSize:  server.GetCompressMaxSize(),
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

	if !f.chainState.CacheFileSystem.Updated {
		return
	}

	fs := f.chainState.CacheFileSystem.FileSystem
	f.fileSystem.Store(&fs)
}

func (f *fileSystemManager) startMonitoring() {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
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
