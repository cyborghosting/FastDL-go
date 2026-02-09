package chain

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dsnet/compress/bzip2"
	"github.com/spf13/afero"
)

type CacheFileSystemState struct {
	Updated bool

	cacheState CacheState

	FileSystem afero.Fs
}

type CacheFileSystem struct {
	next Handler
}

func (c *CacheFileSystem) Handle(state *State) {
	if c.next != nil {
		defer c.next.Handle(state)
	}

	state.CacheFileSystem.Updated = false

	if !state.BuildFileSystem.Updated && state.CacheFileSystem.FileSystem != nil {
		return
	}

	state.CacheFileSystem.Updated = true

	if state.CachePath == "" {
		state.CacheFileSystem.FileSystem = state.BuildFileSystem.FileSystem
		return
	}

	cacheFs := afero.NewBasePathFs(afero.NewOsFs(), state.CachePath)

	unionFs := afero.NewCopyOnWriteFs(state.BuildFileSystem.FileSystem, cacheFs)

	state.CacheFileSystem.FileSystem = newCacheFs(unionFs, cacheFs, &state.CacheFileSystem.cacheState)
}

func (c *CacheFileSystem) SetNext(next Handler) {
	c.next = next
}

type cacheFs struct {
	source     afero.Fs
	target     afero.Fs
	cacheState *CacheState
}

func newCacheFs(source afero.Fs, target afero.Fs, cacheState *CacheState) afero.Fs {
	return &cacheFs{source: source, target: target, cacheState: cacheState}
}

func (c *cacheFs) Create(name string) (afero.File, error) {
	return nil, syscall.EPERM
}

func (c *cacheFs) Mkdir(name string, perm os.FileMode) error {
	return syscall.EPERM
}

func (c *cacheFs) MkdirAll(path string, perm os.FileMode) error {
	return syscall.EPERM
}

func (c *cacheFs) Open(name string) (afero.File, error) {
	f, err := c.source.Open(name)
	if err == nil {
		return f, nil
	}
	if !strings.HasSuffix(name, ".bz2") {
		return nil, err
	}
	if _, err := c.source.Stat(name[:len(name)-4]); err == nil {
		c.cacheState.TryCompress(c.source, c.target, name[:len(name)-4])
	}
	return c.source.Open(name)
}

func (c *cacheFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag&(os.O_WRONLY|syscall.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, syscall.EPERM
	}
	f, err := c.source.OpenFile(name, flag, perm)
	if err == nil {
		return f, nil
	}
	if !strings.HasSuffix(name, ".bz2") {
		return nil, err
	}
	if _, err := c.source.Stat(name[:len(name)-4]); err == nil {
		c.cacheState.TryCompress(c.source, c.target, name[:len(name)-4])
	}
	return c.source.OpenFile(name, flag, perm)
}

func (c *cacheFs) Remove(name string) error {
	return syscall.EPERM
}

func (c *cacheFs) RemoveAll(p string) error {
	return syscall.EPERM
}

func (c *cacheFs) Rename(o, n string) error {
	return syscall.EPERM
}

func (c *cacheFs) Stat(name string) (os.FileInfo, error) {
	s, err := c.source.Stat(name)
	if err == nil {
		return s, nil
	}
	if !strings.HasSuffix(name, ".bz2") {
		return nil, err
	}
	if _, err := c.source.Stat(name[:len(name)-4]); err == nil {
		c.cacheState.TryCompress(c.source, c.target, name[:len(name)-4])
	}
	return c.source.Stat(name)
}

func (c *cacheFs) Name() string {
	return "CacheFs"
}

func (c *cacheFs) Chmod(name string, mode os.FileMode) error {
	return syscall.EPERM
}

func (c *cacheFs) Chown(name string, uid, gid int) error {
	return syscall.EPERM
}

func (c *cacheFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return syscall.EPERM
}

type compressState struct {
	mu      sync.Mutex
	cond    *sync.Cond
	done    bool
	err     error
	waiters int
}

type CacheState struct {
	registry sync.Map
}

func (c *CacheState) TryCompress(source afero.Fs, target afero.Fs, key string) error {
	// Initialize a new compressState
	state := &compressState{}
	state.cond = sync.NewCond(&state.mu)

	// Try to load or store the state
	value, loaded := c.registry.LoadOrStore(key, state)
	state = value.(*compressState)

	state.mu.Lock()
	state.waiters++
	// If it was loaded, let it wait
	if loaded {
		for !state.done {
			state.cond.Wait()
		}
		err := state.err
		state.waiters--
		if state.waiters == 0 {
			c.registry.Delete(key)
		}
		state.mu.Unlock()
		return err
	}

	// Not loaded, so this goroutine is responsible for compressing
	state.mu.Unlock()

	err := compressFunction(source, target, key)

	state.mu.Lock()
	state.done = true
	state.err = err
	state.cond.Broadcast()
	state.waiters--
	if state.waiters == 0 {
		c.registry.Delete(key)
	}
	state.mu.Unlock()
	return err
}

func compressFunction(source afero.Fs, target afero.Fs, key string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compression panic: %v", r)
		}
	}()

	err = target.MkdirAll(filepath.Dir(key), 0755)
	if err != nil {
		return
	}

	rf, err := source.Open(key)
	if err != nil {
		return
	}
	defer rf.Close()

	wf, err := afero.TempFile(target, "/", "compress-*.bz2")
	if err != nil {
		return
	}

	name := wf.Name()

	err = compressStream(rf, wf)
	if err != nil {
		wf.Close()
		target.Remove(name)
		return
	}

	wf.Close()

	err = target.Rename(name, key+".bz2")
	if err != nil {
		target.Remove(name)
		return
	}

	err = target.Chmod(key+".bz2", 0644)
	if err != nil {
		return
	}

	return nil
}

func compressStream(r io.Reader, w io.Writer) error {
	bw, err := bzip2.NewWriter(w, &bzip2.WriterConfig{Level: bzip2.BestCompression})
	if err != nil {
		return err
	}
	_, err = io.Copy(bw, r)
	if err != nil {
		return err
	}
	err = bw.Close()
	if err != nil {
		return err
	}
	return nil
}
