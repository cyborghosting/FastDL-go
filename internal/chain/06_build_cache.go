package chain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

	cacheFs := newCacheFs(baseFs, layerFs, &state.CacheFs.cacheState, state.CompressMaxSize)

	state.CacheFs.Fs = cacheFs
}

func (c *BuildCacheFs) SetNext(next Handler) {
	c.next = next
}

type cacheFs struct {
	base  afero.Fs
	layer afero.Fs

	cacheState      *CacheState
	compressMaxSize int64
}

type openFunc func(fs afero.Fs, name string) (afero.File, error)

func newCacheFs(base afero.Fs, layer afero.Fs, cacheState *CacheState, compressMaxSize int64) afero.Fs {
	return &cacheFs{base: base, layer: layer, cacheState: cacheState, compressMaxSize: compressMaxSize}
}

func (c *cacheFs) procedure(open openFunc, name string) (afero.File, error) {
	if !strings.HasSuffix(name, ".bz2") {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	basename := strings.TrimSuffix(name, ".bz2")

	rawFile, err := c.base.Open(basename)
	if err != nil {
		return nil, err
	}
	defer rawFile.Close()

	rawStat, err := rawFile.Stat()
	if err != nil {
		return nil, err
	}

	compFile, err := open(c.layer, name)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		compStat, err := compFile.Stat()
		if err != nil {
			compFile.Close()
			return nil, err
		} else if !compStat.ModTime().Before(rawStat.ModTime()) {
			return compFile, nil
		} else {
			compFile.Close()
		}
	}

	if rawStat.Size() > c.compressMaxSize {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}

	err = c.cacheState.TryCompress(basename, rawFile, rawStat, c.layer)
	if err != nil {
		return nil, err
	}

	return open(c.layer, name)
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
	f, err := c.base.Open(name)
	if err == nil {
		return f, nil
	}

	open := func(fs afero.Fs, name string) (afero.File, error) {
		return fs.Open(name)
	}

	if f, err := c.procedure(open, name); err == nil {
		return f, nil
	}

	return f, err
}

func (c *cacheFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.EPERM}
	}

	f, err := c.base.OpenFile(name, flag, perm)
	if err == nil {
		return f, nil
	}

	open := func(fs afero.Fs, name string) (afero.File, error) {
		return fs.OpenFile(name, flag, perm)
	}

	if f, err := c.procedure(open, name); err == nil {
		return f, nil
	}

	return nil, err
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
	s, err := c.base.Stat(name)
	if err == nil {
		return s, nil
	}

	open := func(fs afero.Fs, name string) (afero.File, error) {
		return fs.Open(name)
	}

	if f, err := c.procedure(open, name); err == nil {
		defer f.Close()
		return f.Stat()
	}

	return nil, err
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
	waiters int

	mu   sync.Mutex
	cond *sync.Cond

	done bool
	err  error

	modTime time.Time

	ctx    context.Context
	cancel context.CancelFunc
}

func newCompressState() *compressState {
	state := &compressState{}
	state.cond = sync.NewCond(&state.mu)
	return state
}

type CacheState struct {
	registry sync.Map
}

func (c *CacheState) TryCompress(key string, f afero.File, s os.FileInfo, fs afero.Fs) error {
	var state *compressState
	for {
		value, _ := c.registry.LoadOrStore(key, newCompressState())
		state = value.(*compressState)

		state.mu.Lock()
		current, ok := c.registry.Load(key)
		if ok && current == value {
			state.waiters++
			break
		}
		state.mu.Unlock()
	}

	defer func() {
		state.waiters--
		if state.waiters == 0 {
			c.registry.Delete(key)
		}
		state.mu.Unlock()
	}()

	modTime := s.ModTime()

	switch {
	case state.ctx != nil && state.modTime.Before(modTime):
		state.cancel()
		fallthrough
	case state.ctx == nil:
		ctx, cancel := context.WithCancel(context.Background())

		state.done = false
		state.err = nil
		state.modTime = modTime
		state.ctx = ctx
		state.cancel = cancel

		state.mu.Unlock()
		tmp, err := compressFunction(ctx, key, f, s, fs)
		state.mu.Lock()

		if ctx.Err() != nil {
			if err == nil {
				fs.Remove(tmp)
			}
			for !state.done {
				state.cond.Wait()
			}
			return state.err
		}

		if err == nil {
			err = rename(fs, tmp, key+".bz2")
			if err != nil {
				fs.Remove(tmp)
			}
		}

		state.done = true
		state.err = err
		state.cond.Broadcast()
		return err
	}

	for !state.done {
		state.cond.Wait()
	}
	return state.err
}

func compressFunction(ctx context.Context, key string, r afero.File, s os.FileInfo, fs afero.Fs) (name string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compression panic: %v", r)
		}
	}()

	err = fs.MkdirAll(filepath.Dir(key), 0755)
	if err != nil {
		return
	}

	w, err := afero.TempFile(fs, "/", "compress-*.bz2")
	if err != nil {
		return
	}
	name = w.Name()

	err = compressStream(ctx, w, r)
	if err != nil {
		w.Close()
		fs.Remove(name)
		return
	}
	w.Close()

	err = fs.Chtimes(name, time.Now(), s.ModTime())
	if err != nil {
		fs.Remove(name)
		return
	}

	err = fs.Chmod(name, 0644)
	if err != nil {
		fs.Remove(name)
		return
	}

	return
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *contextReader) Read(p []byte) (n int, err error) {
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
		return c.r.Read(p)
	}
}

func compressStream(ctx context.Context, w io.Writer, r io.Reader) error {
	b, err := bzip2.NewWriter(w, &bzip2.WriterConfig{Level: bzip2.BestCompression})
	if err != nil {
		return err
	}
	_, err = io.Copy(b, &contextReader{ctx: ctx, r: r})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := b.Close(); err != nil {
		return err
	}
	return nil
}

func rename(fs afero.Fs, oldname string, newname string) error {
	err := fs.Rename(oldname, newname)
	if err == nil {
		return nil
	}

	if runtime.GOOS != "windows" {
		return err
	}

	err = fs.Remove(newname)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return fs.Rename(oldname, newname)
}
