package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/phngkhuongduy/shortlink/internal/httpapi"
	"github.com/phngkhuongduy/shortlink/internal/repository"
	"github.com/phngkhuongduy/shortlink/internal/shortener"
)

func main() {
	cfg := loadConfig()

	repo, err := repository.NewSQLite(cfg.dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer repo.Close()

	gen := shortener.NewRandomGenerator(cfg.codeLength)
	svc := shortener.NewService(repo, gen, cfg.maxRetries)
	handler := httpapi.NewHandler(svc, cfg.baseURL)

	gin.SetMode(gin.ReleaseMode)
	srv := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run the server and wait for an interrupt to shut down gracefully.
	go func() {
		log.Printf("shortlink listening on :%s (db=%s)", cfg.port, cfg.dbPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

type config struct {
	port       string
	dbPath     string
	baseURL    string
	codeLength int
	maxRetries int
}

func loadConfig() config {
	cfg := config{
		port:       getenv("PORT", "8080"),
		dbPath:     getenv("DB_PATH", "shortlink.db"),
		baseURL:    getenv("BASE_URL", ""),
		codeLength: getenvInt("CODE_LENGTH", 7),
		maxRetries: getenvInt("MAX_RETRIES", 5),
	}
	if cfg.baseURL == "" {
		cfg.baseURL = "http://localhost:" + cfg.port
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
