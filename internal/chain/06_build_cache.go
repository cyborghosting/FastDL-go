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

type CacheFsState struct {
	Updated bool
	Fs      afero.Fs

	cacheState CacheState
}

type BuildCacheFs struct {
	next Handler
}

func (c *BuildCacheFs) Handle(state *State) {
	if c.next != nil {
		defer c.next.Handle(state)
	}

	state.CacheFs.Updated = false

	if !state.OverlayFs.Updated && state.CacheFs.Fs != nil {
		return
	}

	state.CacheFs.Updated = true

	if state.CachePath == "" {
		state.CacheFs.Fs = state.OverlayFs.Fs
		return
	}

	baseFs := state.OverlayFs.Fs
	layerFs := afero.NewBasePathFs(afero.NewOsFs(), state.CachePath)
	unionFs := afero.NewCopyOnWriteFs(baseFs, layerFs)

	cacheFs := newCacheFs(unionFs, layerFs, &state.CacheFs.cacheState, state.CompressMaxSize)

	state.CacheFs.Fs = cacheFs
}

func (c *BuildCacheFs) SetNext(next Handler) {
	c.next = next
}

type cacheFs struct {
	union           afero.Fs
	layer           afero.Fs
	cacheState      *CacheState
	compressMaxSize int64
}

func newCacheFs(union afero.Fs, layer afero.Fs, cacheState *CacheState, compressMaxSize int64) afero.Fs {
	return &cacheFs{union: union, layer: layer, cacheState: cacheState, compressMaxSize: compressMaxSize}
}

func (c *cacheFs) tryBuildCache(name string) bool {
	if !strings.HasSuffix(name, ".bz2") {
		return false
	}

	base := strings.TrimSuffix(name, ".bz2")
	if s, err := c.union.Stat(base); err == nil {
		if s.Size() > c.compressMaxSize {
			return false
		}
		_ = c.cacheState.TryCompress(c.union, c.layer, base)
	}

	return true
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
	f, err := c.union.Open(name)
	if err == nil {
		return f, nil
	}
	if !c.tryBuildCache(name) {
		return nil, err
	}
	return c.union.Open(name)
}

func (c *cacheFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag&(os.O_WRONLY|syscall.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, syscall.EPERM
	}
	f, err := c.union.OpenFile(name, flag, perm)
	if err == nil {
		return f, nil
	}
	if !c.tryBuildCache(name) {
		return nil, err
	}
	return c.union.OpenFile(name, flag, perm)
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
	s, err := c.union.Stat(name)
	if err == nil {
		return s, nil
	}
	if !c.tryBuildCache(name) {
		return nil, err
	}
	return c.union.Stat(name)
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

func newCompressState() *compressState {
	state := &compressState{}
	state.cond = sync.NewCond(&state.mu)
	return state
}

type CacheState struct {
	registry sync.Map
}

func (c *CacheState) TryCompress(source afero.Fs, target afero.Fs, key string) error {
	value, loaded := c.registry.LoadOrStore(key, newCompressState())
	state := value.(*compressState)

	state.mu.Lock()
	state.waiters++
	defer func() {
		state.waiters--
		if state.waiters == 0 {
			c.registry.Delete(key)
		}
		state.mu.Unlock()
	}()

	if loaded {
		for !state.done {
			state.cond.Wait()
		}
		return state.err
	}

	state.mu.Unlock()
	err := compressFunction(source, target, key)
	state.mu.Lock()

	state.done = true
	state.err = err
	state.cond.Broadcast()
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
