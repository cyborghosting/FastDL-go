package internal

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

type Predicate func(string) bool

type FileHandler struct {
	fsManager FileSystemManager
	share     string
	predicate Predicate
}

func NewFileHandler(fsManager FileSystemManager, share string, predicate Predicate) *FileHandler {
	return &FileHandler{
		fsManager: fsManager,
		share:     share,
		predicate: predicate,
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
	{"/models", "/models", Suffix(".mdl", ".phy", ".vmt", ".vtf", ".vtx", ".vvd", ".ani")},
	{"/resource/fonts", "/resource/fonts", Suffix(".ttf")},
	{"/scripts/items", "/scripts/items", Suffix(".txt")},
	{"/shaders", "/shaders", Suffix(".vcs")},
	{"/sound", "/sound", Suffix(".mp3", ".wav")},
}

func AssignRoutes(routerGroup *gin.RouterGroup, fsManager FileSystemManager) {
	for _, subRoute := range subRoutes {
		handler := NewFileHandler(fsManager, subRoute.Share, subRoute.Predicate)
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

	s, err := fs.Stat(filepath)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			log.Printf("Permission Denied: %s", filepath)
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNotFound)
		return
	}

	if !s.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}

	lastModified := getLastModified(s)
	etag := getETag(s)

	if c.Request.Header.Get("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		c.Header("Last-Modified", lastModified)
		c.Header("ETag", etag)
		return
	}

	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Last-Modified", lastModified)
	c.Header("ETag", etag)
}

func (fh *FileHandler) HandleGET(c *gin.Context) {
	filepath := path.Join(fh.share, c.Param("path"))

	if !fh.predicate(filepath) {
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	fs := fh.fsManager.GetFileSystem()

	f, err := fs.Open(filepath)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			log.Printf("Permission Denied: %s", filepath)
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNotFound)
		return
	}
	defer f.Close()

	s, err := f.Stat()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	if !s.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}

	lastModified := getLastModified(s)
	etag := getETag(s)

	if c.Request.Header.Get("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		c.Header("Last-Modified", lastModified)
		c.Header("ETag", etag)
		return
	}

	c.DataFromReader(http.StatusOK, s.Size(), "application/octet-stream", f, map[string]string{
		"Last-Modified": lastModified,
		"ETag":          etag,
	})
}

func getLastModified(stat os.FileInfo) string {
	return stat.ModTime().UTC().Format(http.TimeFormat)
}

func getETag(stat os.FileInfo) string {
	etagBase := fmt.Sprintf("%d-%d", stat.ModTime().UnixNano(), stat.Size())
	return fmt.Sprintf("\"%x\"", md5.Sum([]byte(etagBase)))
}
