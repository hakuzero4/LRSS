package main

import (
	"context"
	"embed"
	"log"
	"time"

	"lrss/internal/appsvc"
	"lrss/internal/db"
	"lrss/internal/job"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
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

	repos := repo.New(database.SQL, repo.WithEmbeddingEnabled(func(ctx context.Context) bool {
		cfg, err := store.LoadEmbeddingConfig(ctx)
		return err == nil && cfg.IsConfigured()
	}))
	library := service.NewLibraryFromRepos(repos, &rss.Client{})
	feedAPI := appsvc.NewFeedService(library)
	articleAPI := appsvc.NewArticleService(library)

	app := application.New(application.Options{
		Name:        "LRSS",
		Description: "Local-first RSS reader with optional vector search",
		Services: []application.Service{
			application.NewService(settingsAPI),
			application.NewService(searchAPI),
			application.NewService(feedAPI),
			application.NewService(articleAPI),
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
