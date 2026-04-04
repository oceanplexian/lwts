package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/oceanplexian/lwts/server/internal/auth"
	boardhandler "github.com/oceanplexian/lwts/server/internal/board"
	cardhandler "github.com/oceanplexian/lwts/server/internal/card"
	commenthandler "github.com/oceanplexian/lwts/server/internal/comment"
	discordnotifier "github.com/oceanplexian/lwts/server/internal/discord"
	settingshandler "github.com/oceanplexian/lwts/server/internal/settings"
	webhookhandler "github.com/oceanplexian/lwts/server/internal/webhook"
	"github.com/oceanplexian/lwts/server/internal/config"
	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/middleware"
	"github.com/oceanplexian/lwts/server/internal/repo"
	"github.com/oceanplexian/lwts/server/internal/sse"
	"github.com/oceanplexian/lwts/server/migrations"
)

var version = "dev"
var commit = "unknown"
var buildDate = "unknown"

func main() {
	// Handle CLI subcommands that don't need full config
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			runMigrate()
			return
		case "seed":
			runSeed()
			return
		case "reset-password":
			runResetPassword()
			return
		case "users":
			runUsers()
			return
		case "user-create":
			runUserCreate()
			return
		case "user-delete":
			runUserDelete()
			return
		case "boards":
			runBoards()
			return
		case "board-create":
			runBoardCreate()
			return
		case "board-delete":
			runBoardDelete()
			return
		case "cards":
			runCards()
			return
		case "card-show":
			runCardShow()
			return
		case "card-delete":
			runCardDelete()
			return
		case "reseed":
			runReseed()
			return
		case "seed-test":
			runSeedTest()
			return
		case "stats":
			runStats()
			return
		case "backup":
			runBackup()
			return
		case "restore":
			runRestore()
			return
		case "api-key":
			runAPIKey()
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// Set up slog
	var handler slog.Handler
	level := parseLogLevel(cfg.LogLevel)
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Create router
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"version":   version,
			"commit":    commit,
			"buildDate": buildDate,
		})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		// TODO: check DB connectivity once datasource is wired
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Database — auto-setup tables on startup
	ctx := context.Background()
	ds := getDS(ctx)
	defer ds.Close()
	if err := db.Migrate(ctx, ds, migrations.FS); err != nil {
		logger.Error("database setup failed", "error", err)
		os.Exit(1)
	}

	// SSE hub
	sseHub := sse.NewHub()
	go sseHub.Run()

	// Repositories
	userRepo := repo.NewUserRepository(ds)
	boardRepo := repo.NewBoardRepository(ds)
	cardRepo := repo.NewCardRepository(ds)
	commentRepo := repo.NewCommentRepository(ds)

	// Auth
	userAdapter := auth.NewUserRepoAdapter(userRepo)
	tokenStore := auth.NewDBTokenStore(ds)
	authHandler := auth.NewHandler(userAdapter, tokenStore, cfg.JWTSecret, logger)
	authHandler.SetDatasource(ds)
	authHandler.RegisterRoutes(mux)

	// Auth middleware (JWT for browser, lwts_sk_ for API keys)
	authMW := auth.RequireAuth(cfg.JWTSecret, userAdapter, ds)
	memberMW := func(next http.Handler) http.Handler {
		return authMW(auth.RequireRole("member")(next))
	}

	// Discord notifier
	discordN := discordnotifier.NewNotifier(ds, userRepo, logger)
	go discordN.Run()

	// Board routes
	bh := boardhandler.NewHandler(boardRepo, cardRepo, commentRepo, sseHub)
	bh.RegisterRoutes(mux, authMW)

	// Card routes
	ch := cardhandler.NewHandler(cardRepo, boardRepo, commentRepo, sseHub)
	ch.SetDiscord(discordN)
	ch.RegisterRoutes(mux, authMW, memberMW)

	// Comment routes
	cmh := commenthandler.NewHandler(commentRepo, cardRepo, sseHub)
	cmh.SetBoards(boardRepo)
	cmh.SetDiscord(discordN)
	cmh.RegisterRoutes(mux, authMW, memberMW)

	// Webhook routes
	whStore := webhookhandler.NewStore(ds)
	whDispatcher := webhookhandler.NewDispatcher(whStore, logger)
	go whDispatcher.Run()
	wh := webhookhandler.NewHandler(whStore, whDispatcher)
	wh.RegisterRoutes(mux)

	// Search
	sh := boardhandler.NewSearchHandler(ds)
	sh.RegisterRoutes(mux, authMW)

	// Users list
	mux.Handle("GET /api/v1/users", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users, err := userRepo.List(r.Context())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
			return
		}
		if users == nil {
			users = []repo.User{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})))

	// Seed function — creates demo board+cards for a user
	seedFunc := func(ctx context.Context, ownerID string) error {
		return repo.SeedDemo(ctx, ds, ownerID)
	}

	// Settings routes
	stg := settingshandler.NewHandler(ds, userRepo, boardRepo, cardRepo, commentRepo)
	stg.SetSeedFunc(settingshandler.SeedFunc(seedFunc))
	adminMW := func(next http.Handler) http.Handler {
		return authMW(auth.RequireRole("admin")(next))
	}
	stg.RegisterRoutes(mux, authMW, adminMW)
	stg.RegisterDiscordRoutes(mux, adminMW)
	authHandler.SetRegistrationChecker(stg)
	authHandler.SetSeedFunc(auth.SeedFunc(seedFunc))

	// SSE routes
	mux.HandleFunc("GET /api/v1/boards/{id}/stream", sse.StreamHandler(sseHub, cfg.JWTSecret))
	mux.HandleFunc("GET /api/v1/boards/{id}/presence", sse.PresenceHandler(sseHub, cfg.JWTSecret))

	// Static files
	if cfg.DevMode {
		// Dev mode: no-cache headers, serves from web/ then web/public/ (mimics Vite publicDir)
		webFS := http.FileServer(http.Dir("web"))
		publicFS := http.FileServer(http.Dir("web/public"))
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".html") || strings.HasSuffix(r.URL.Path, ".css") || strings.HasSuffix(r.URL.Path, ".js") || r.URL.Path == "/" {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			if _, err := os.Stat("web" + r.URL.Path); err == nil || r.URL.Path == "/" {
				webFS.ServeHTTP(w, r)
			} else {
				publicFS.ServeHTTP(w, r)
			}
		}))
	} else {
		// Production: serve Vite build output from STATIC_DIR, SPA fallback to index.html
		staticFS := http.Dir(cfg.StaticDir)
		fileServer := http.FileServer(staticFS)
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: serve index.html for missing paths that aren't API routes or files
			if _, err := os.Stat(cfg.StaticDir + r.URL.Path); os.IsNotExist(err) && !strings.HasPrefix(r.URL.Path, "/api/") && !strings.Contains(r.URL.Path, ".") {
				http.ServeFile(w, r, cfg.StaticDir+"/index.html")
				return
			}
			fileServer.ServeHTTP(w, r)
		}))
	}

	// Apply middleware chain
	var h http.Handler = mux
	if !cfg.DevMode {
		h = middleware.BodyLimit(cfg.MaxUploadSize)(h)
		h = middleware.RateLimit(100, 100)(h)
		h = middleware.CORS(cfg.CORSOrigins)(h)
		h = middleware.SecurityHeaders(h)
	}
	h = middleware.Recovery(logger)(h)
	h = middleware.Logger(logger)(h)
	h = middleware.RequestID(h)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "port", cfg.Port, "version", version, "dev", cfg.DevMode)
		var listenErr error
		if srv.TLSConfig != nil {
			listenErr = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			listenErr = srv.ListenAndServe()
		}
		if listenErr != nil && listenErr != http.ErrServerClosed {
			logger.Error("server failed", "error", listenErr)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
	sseHub.Stop()
	discordN.Stop()
	logger.Info("server stopped")
}


func getDS(ctx context.Context) db.Datasource {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://lwts:lwts@localhost:5432/lwts?sslmode=disable"
	}
	ds, err := db.NewDatasource(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	return ds
}

func runMigrate() {
	ctx := context.Background()
	ds := getDS(ctx)
	defer ds.Close()
	if err := db.Migrate(ctx, ds, migrations.FS); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("migrations applied")
}

func runSeed() {
	ctx := context.Background()
	ds := getDS(ctx)
	defer ds.Close()
	// Run migrations first
	if err := db.Migrate(ctx, ds, migrations.FS); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}

	if err := repo.Seed(ctx, ds); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("seed data applied")
}


func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
