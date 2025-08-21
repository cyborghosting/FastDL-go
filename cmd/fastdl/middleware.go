package main

import (
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

func Sanitizer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if p, np := c.Request.URL.Path, cleanPath(c.Request.URL.Path); p != np {
			c.Abort()
			c.Redirect(http.StatusTemporaryRedirect, np)
		}
	}
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		// Fast path for common case of p being the string we want:
		if len(p) == len(np)+1 && strings.HasPrefix(p, np) {
			np = p
		} else {
			np += "/"
		}
	}
	return np
}
