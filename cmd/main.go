package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/cyborghosting/fastdl/config"
	"github.com/cyborghosting/fastdl/internal"
)

func main() {
	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	setResourceLimits()

	r := gin.New()

	RegisterMiddlewares(r)

	addTrustedProxies(r)

	addHealthCheckEndpoint(r)

	config, err := config.RunConfigurationLoader()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fileSystems := make([]internal.FileSystemManager, 0, len(config.Servers))
	defer func() {
		// Cleanup all file systems
		for _, fs := range fileSystems {
			fs.Close()
		}
	}()

	for _, server := range config.Servers {
		if server.RootDir == "" {
			log.Printf("Skipping server %s: no root directory specified", server.Route)
			continue
		}
		_, ok := server.BaseDirs["gameinfo_path"]
		if !ok {
			log.Printf("Skipping server %s: no gameinfo_path specified", server.Route)
			continue
		}

		fsManager, err := internal.NewFileSystemManager(server.RootDir, server.BaseDirs)
		if err != nil {
			log.Printf("Skipping server %s: failed to create filesystem manager: %v", server.Route, err)
			continue
		}
		fileSystems = append(fileSystems, fsManager)

		internal.AssignRoutes(r.Group(server.Route), fsManager, server.CompressMaxSize)
	}

	// Start server in a goroutine
	go func() {
		if err := r.Run(port()); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down server...")
}

func setResourceLimits() {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Printf("Failed to get ulimit: %v", err)
		return
	}
	rLimit.Cur = max(rLimit.Cur, 4096)
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Printf("Failed to set ulimit: %v", err)
		return
	}
}

func addTrustedProxies(r *gin.Engine) {
	CIDRs, ok := os.LookupEnv("FORWARDED_ALLOW_IPS")
	if !ok {
		r.SetTrustedProxies(nil)
		return
	}
	CIDRList := strings.Split(CIDRs, ",")
	for i := range CIDRList {
		CIDRList[i] = strings.TrimSpace(CIDRList[i])
	}
	r.SetTrustedProxies(CIDRList)
}

func addHealthCheckEndpoint(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
}

func port() string {
	port, ok := os.LookupEnv("PORT")
	if !ok || port == "" {
		port = "8080"
	}
	return ":" + port
}
