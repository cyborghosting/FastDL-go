package internal

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/dsnet/compress/bzip2"
	"github.com/gin-gonic/gin"
)

type Predicate func(string) bool

type FileHandler struct {
	fsManager       FileSystemManager
	share           string
	predicate       Predicate
	compressMaxSize int64
}

func NewFileHandler(fsManager FileSystemManager, share string, predicate Predicate, compressMaxSize int64) *FileHandler {
	return &FileHandler{
		fsManager:       fsManager,
		share:           share,
		predicate:       predicate,
		compressMaxSize: compressMaxSize,
	}
}

func Suffix(suffixes ...string) Predicate {
	allSuffixes := make([]string, 0, len(suffixes)*2)

	// Add bz2 variants
	for _, suffix := range suffixes {
		allSuffixes = append(allSuffixes, suffix+".bz2")
	}
	allSuffixes = append(allSuffixes, suffixes...)

	return func(path string) bool {
		for _, suffix := range allSuffixes {
			if strings.HasSuffix(path, suffix) {
				return true
			}
		}
		return false
	}
}

type SubRoute struct {
	Path      string
	Share     string
	Predicate Predicate
}

var subRoutes = []SubRoute{
	{"/maps", "/maps", Suffix(".bsp", ".nav", ".ain")},
	{"/materials", "/materials", Suffix(".vmt", ".vtf")},
	{"/models", "/models", Suffix(".mdl", ".phy", ".vmt", ".vtf", ".vtx", ".vvd")},
	{"/resource/fonts", "/resource/fonts", Suffix(".ttf")},
	{"/scripts/items", "/scripts/items", Suffix(".txt")},
	{"/shaders", "/shaders", Suffix(".vcs")},
	{"/sound", "/sound", Suffix(".mp3", ".wav")},
}

func AssignRoutes(routerGroup *gin.RouterGroup, fsManager FileSystemManager, compressMaxSize int64) {
	for _, subRoute := range subRoutes {
		handler := NewFileHandler(fsManager, subRoute.Share, subRoute.Predicate, compressMaxSize)
		routerGroup.HEAD(subRoute.Path+"/*path", handler.HandleHEAD)
		routerGroup.GET(subRoute.Path+"/*path", handler.HandleGET)
	}
}

func (fh *FileHandler) HandleHEAD(c *gin.Context) {
	filepath := path.Join(fh.share, c.Param("path"))

	if !fh.predicate(filepath) {
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	fs := fh.fsManager.GetFileSystem()

	if stat, err := fs.Stat(filepath); err == nil && stat.Mode().IsRegular() {
		lastModified := getLastModified(stat)
		etag := getETag(stat)
		if handleIfNoneMatch(c, lastModified, etag) {
			return
		}
		c.Status(http.StatusOK)
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Last-Modified", lastModified)
		c.Header("ETag", etag)
		return
	}

	if strings.HasSuffix(filepath, ".bz2") {
		uncompressedPath := strings.TrimSuffix(filepath, ".bz2")
		if stat, err := fs.Stat(uncompressedPath); err == nil && stat.Mode().IsRegular() {
			lastModified := getLastModified(stat)
			etag := getETag(stat)
			if handleIfNoneMatch(c, lastModified, etag) {
				return
			}
			c.Status(http.StatusOK)
			c.Header("Content-Type", "application/octet-stream")
			c.Header("Last-Modified", lastModified)
			c.Header("ETag", etag)
			return
		}
	}

	c.Status(http.StatusNotFound)
}

func (fh *FileHandler) HandleGET(c *gin.Context) {
	filepath := path.Join(fh.share, c.Param("path"))

	if !fh.predicate(filepath) {
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	fs := fh.fsManager.GetFileSystem()

	if f, err := fs.Open(filepath); err == nil {
		defer f.Close()
		stat, err := f.Stat()
		if err != nil || !stat.Mode().IsRegular() {
			c.Status(http.StatusInternalServerError)
			return
		}
		lastModified := getLastModified(stat)
		etag := getETag(stat)
		if handleIfNoneMatch(c, lastModified, etag) {
			return
		}
		c.DataFromReader(http.StatusOK, stat.Size(), "application/octet-stream", f, map[string]string{
			"Last-Modified": lastModified,
			"ETag":          etag,
		})
		return
	}

	if strings.HasSuffix(filepath, ".bz2") {
		uncompressedPath := strings.TrimSuffix(filepath, ".bz2")
		if f, err := fs.Open(uncompressedPath); err == nil {
			defer f.Close()
			stat, err := f.Stat()
			if err != nil || !stat.Mode().IsRegular() {
				c.Status(http.StatusInternalServerError)
				return
			}
			if stat.Size() > fh.compressMaxSize {
				c.Status(http.StatusNotFound)
				return
			}
			lastModified := getLastModified(stat)
			etag := getETag(stat)
			if handleIfNoneMatch(c, lastModified, etag) {
				return
			}
			b, err := compress(f)
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
			c.DataFromReader(http.StatusOK, int64(len(b)), "application/octet-stream", bytes.NewReader(b), map[string]string{
				"Last-Modified": lastModified,
				"ETag":          etag,
			})
			return
		}
	}

	c.Status(http.StatusNotFound)
}

func getLastModified(stat os.FileInfo) string {
	return stat.ModTime().UTC().Format(http.TimeFormat)
}
func getETag(stat os.FileInfo) string {
	etagBase := fmt.Sprintf("%d-%d", stat.ModTime().UnixNano(), stat.Size())
	return fmt.Sprintf("\"%x\"", md5.Sum([]byte(etagBase)))
}
func handleIfNoneMatch(c *gin.Context, lastModified string, etag string) bool {
	if c.Request.Header.Get("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		c.Header("Last-Modified", lastModified)
		c.Header("ETag", etag)
		return true
	}
	return false
}

func compress(r io.Reader) ([]byte, error) {
	var buffer bytes.Buffer
	w, err := bzip2.NewWriter(&buffer, &bzip2.WriterConfig{Level: bzip2.BestSpeed})
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(w, r)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
