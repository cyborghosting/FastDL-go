package main

import (
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/cyborghosting/fastdl/config"
	"github.com/cyborghosting/fastdl/internal"
	"github.com/cyborghosting/fastdl/utils"
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

	fileSystems := make([]internal.FileSystemManager, 0, len(config.GetServers()))
	defer func() {
		// Cleanup all file systems
		for _, fs := range fileSystems {
			fs.Close()
		}
	}()

	for _, server := range config.GetServers() {
		if server.GetInstallationPath() == "" {
			log.Printf("Skipping server %s: no root directory specified", server.GetRoute())
			continue
		}
		_, ok := server.GetDictionary()["gameinfo_path"]
		if !ok {
			log.Printf("Skipping server %s: no gameinfo_path specified", server.GetRoute())
			continue
		}
		_, err := os.Stat(path.Join(server.GetInstallationPath(), server.GetDictionary()["gameinfo_path"], "gameinfo.txt"))
		if err != nil {
			log.Printf("Skipping server %s: gameinfo.txt not found: %v", server.GetRoute(), err)
			continue
		}

		fsManager, err := internal.NewFileSystemManager(server)
		if err != nil {
			log.Printf("Skipping server %s: failed to create filesystem manager: %v", server.GetRoute(), err)
			continue
		}
		fileSystems = append(fileSystems, fsManager)

		internal.AssignRoutes(r.Group(server.GetRoute()), fsManager)
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
	_, err := utils.TrySetNoFile(4096)
	if err != nil {
		log.Printf("Failed to set nofile limit: %v", err)
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
