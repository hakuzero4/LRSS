package main

import (
	"context"
	"embed"
	"log"
	"time"

	"lrss/internal/appsvc"
	"lrss/internal/db"
	"lrss/internal/job"
	"lrss/internal/notify"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[string]("time")
}

func main() {
	ctx := context.Background()
	database, err := db.Open(ctx, db.Options{})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()
	log.Printf("database ready: %s", database.Path)

	vs := db.VectorInfo()
	if vs.Loaded {
		log.Printf("sqlite-vector loaded: version=%s backend=%s path=%s", vs.Version, vs.Backend, vs.Path)
	} else {
		log.Printf("sqlite-vector not loaded (FTS-only / brute-force vector): %s", vs.Error)
	}

	store := settings.NewStore(database.SQL)
	searchSvc := search.New(database.SQL, store)
	embedWorker := job.NewEmbedWorker(database.SQL, store)
	settingsAPI := appsvc.NewSettings(store, searchSvc, embedWorker)
	searchAPI := appsvc.NewSearch(searchSvc)

	// Desktop notifications (Windows toast / macOS / Linux).
	ns := notifications.New()
	notifier := notify.New(ns, store)
	settingsAPI.SetNotifier(notifier)

	repos := repo.New(database.SQL, repo.WithEmbeddingEnabled(func(ctx context.Context) bool {
		cfg, err := store.LoadEmbeddingConfig(ctx)
		return err == nil && cfg.IsConfigured()
	}))
	library := service.NewLibraryFromRepos(repos, &rss.Client{})
	settingsAPI.SetLibrary(library)
	feedAPI := appsvc.NewFeedService(library)
	feedAPI.SetNotifier(notifier)
	articleAPI := appsvc.NewArticleService(library)

	// Background auto-refresh (reads LibraryConfig each tick).
	go runAutoRefresh(ctx, library, store, notifier)
	// Delayed retention purge so startup is not contending on SQLite.
	go runStartupPurge(ctx, library, store)

	app := application.New(application.Options{
		Name:        "LRSS",
		Description: "Local-first RSS reader with optional vector search",
		Services: []application.Service{
			application.NewService(settingsAPI),
			application.NewService(searchAPI),
			application.NewService(feedAPI),
			application.NewService(articleAPI),
			application.NewService(ns),
			application.NewService(&GreetService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "LRSS",
		Width:  1280,
		Height: 800,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(246, 247, 249),
		URL:              "/",
	})

	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// runStartupPurge waits ~20s then deletes articles older than keepArticlesDays.
func runStartupPurge(ctx context.Context, library *service.Library, store *settings.Store) {
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	prefs, err := store.LoadUIPrefs(ctx)
	if err != nil {
		log.Printf("startup purge: load UI prefs: %v", err)
		return
	}
	n, err := library.PurgeOldArticles(ctx, prefs.KeepArticlesDays)
	if err != nil {
		log.Printf("startup purge: %v", err)
		return
	}
	log.Printf("startup purge: deleted %d articles (keep=%d days)", n, prefs.KeepArticlesDays)
}

// runAutoRefresh periodically refreshes all feeds when enabled in settings.
// On each iteration it reloads config so SetLibraryConfig takes effect without restart.
func runAutoRefresh(ctx context.Context, library *service.Library, store *settings.Store, notifier *notify.Sender) {
	// Short initial delay so UI/startup is not contending on SQLite.
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		cfg, err := store.LoadLibraryConfig(ctx)
		if err != nil {
			log.Printf("auto-refresh: load config: %v", err)
			timer.Reset(30 * time.Second)
			continue
		}
		if !cfg.AutoRefresh {
			timer.Reset(30 * time.Second)
			continue
		}

		res, ok, err := library.TryRefreshAll(ctx)
		if err != nil {
			log.Printf("auto-refresh: error: %v", err)
		} else if !ok {
			log.Printf("auto-refresh: skipped (refresh already in progress)")
		} else {
			log.Printf("auto-refresh: ok=%d err=%d added=%d", res.FeedsOK, res.FeedsErr, res.ArticlesAdded)
			if notifier != nil {
				notifier.AfterRefresh(ctx, res.ArticlesAdded)
			}
		}

		interval := time.Duration(cfg.RefreshIntervalMinutes) * time.Minute
		if interval < 5*time.Minute {
			interval = 5 * time.Minute
		}
		timer.Reset(interval)
	}
}
