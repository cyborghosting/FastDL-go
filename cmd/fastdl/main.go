package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	}), gin.Recovery())

	CIDRs, exist := os.LookupEnv("FORWARDED_ALLOW_IPS")
	if exist {
		CIDRList := strings.Split(CIDRs, ",")
		for i := range CIDRList {
			CIDRList[i] = strings.TrimSpace(CIDRList[i])
		}
		r.SetTrustedProxies(CIDRList)
	} else {
		r.SetTrustedProxies(nil)
	}
	r.Use(Sanitizer())

	// Health check endpoint for Docker
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "fastdl",
		})
	})

	config, err := runConfigurationLoader()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fileSystems := make([]FileSystemManager, 0, len(config.Servers))
	defer func() {
		// Cleanup all file systems
		for _, fs := range fileSystems {
			fs.Close()
		}
	}()

	for _, server := range config.Servers {
		fsManager, err := NewFileSystemManager(server.BasePath, server.Directories)
		if err != nil {
			log.Fatalf("Failed to create filesystem for server %s: %v", server.Route, err)
		}
		fileSystems = append(fileSystems, fsManager)

		AssignRoutes(r.Group(server.Route), fsManager, server.CompressMaxSize)
	}

	// Start server in a goroutine
	go func() {
		if err := r.Run(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down server...")
}
