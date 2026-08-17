package main

import (
	"context"
	_ "embed"
	"log"
	"time"

	"lrss/internal/appdata"
	"lrss/internal/appsvc"
	"lrss/internal/db"
	"lrss/internal/desktop"
	"lrss/internal/job"
	"lrss/internal/llm"
	"lrss/internal/notify"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"
	"lrss/internal/web"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// assets is provided by assets_prod.go (//go:build production) or assets_stub.go.

//go:embed build/appicon.png
var trayIcon []byte

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
	log.Printf("profile=%s database ready: %s", appdata.DirName(), database.Path)

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

	// Load UI prefs early for hardware acceleration / quit cleanup.
	uiPrefs, err := store.LoadUIPrefs(ctx)
	if err != nil {
		log.Printf("load UI prefs at startup: %v", err)
		uiPrefs = settings.DefaultUIPrefs()
	}

	// Desktop notifications (Windows toast / macOS / Linux).
	ns := notifications.New()
	notifier := notify.New(ns, store)
	settingsAPI.SetNotifier(notifier)

	repos := repo.New(database.SQL, repo.WithEmbeddingEnabled(func(ctx context.Context) bool {
		cfg, err := store.LoadEmbeddingConfig(ctx)
		return err == nil && cfg.IsConfigured()
	}))
	library := service.NewLibraryFromRepos(repos, &rss.Client{})
	// Settings → Feeds "抓取全文": enqueue only; drain is paced separately from feed refresh.
	library.FullContentEnabled = func(c context.Context) bool {
		prefs, err := store.LoadUIPrefs(c)
		return err == nil && prefs.FetchFullContent
	}
	llmSvc := &llm.Service{Store: store, Cache: &llm.Cache{DB: database.SQL}}
	briefingWorker := service.NewBriefingWorker(store, repos.Briefings, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	keepWorker := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	keepWorker.SetKeepFolders(repos.KeepFolders)
	library.OnArticlesInserted = func(c context.Context, ids []string) {
		briefingWorker.Enqueue(c, ids)
		keepWorker.Enqueue(c, ids)
	}
	settingsAPI.SetLibrary(library)
	feedAPI := appsvc.NewFeedService(library)
	feedAPI.SetNotifier(notifier)
	feedAPI.SetBriefingWorker(briefingWorker)
	feedAPI.SetKeepWorker(keepWorker)
	articleAPI := appsvc.NewArticleService(library, store)
	aiAPI := appsvc.NewAI(store, library, database.SQL)
	briefingAPI := appsvc.NewBriefingService(briefingWorker)
	syncAPI := appsvc.NewSync(store, library)
	// Keep in sync with frontend/src/lib/appMeta.ts APP_VERSION and git tags.
	updateAPI := appsvc.NewUpdate("0.1.13")

	// Optional browser access (same SPA; reader tools + reading assistant + star/read; no settings UI).
	webServer := web.New(web.APIDeps{
		Library:  library,
		Store:    store,
		Search:   searchSvc,
		AI:       appsvc.NewWebAI(aiAPI),
		Briefing: briefingWorker,
		Keep:     keepWorker,
	}, assets)
	settingsAPI.SetWebServer(webServer)
	defer func() {
		if err := webServer.Stop(context.Background()); err != nil {
			log.Printf("web access stop: %v", err)
		}
	}()
	if webCfg, err := store.LoadWebAccessConfig(ctx); err != nil {
		log.Printf("load web access config: %v", err)
	} else if webCfg.Enabled {
		if st, err := webServer.Apply(ctx, webCfg); err != nil {
			log.Printf("web access start: %v", err)
		} else {
			log.Printf("web access enabled: %s", st.URL)
		}
	}

	// Background auto-refresh (reads LibraryConfig each tick).
	go runAutoRefresh(ctx, library, store, notifier, briefingWorker)
	go runBriefingDrain(ctx, briefingWorker)
	go runKeepDrain(ctx, keepWorker)
	// Delayed retention purge so startup is not contending on SQLite.
	go runStartupPurge(ctx, library, store)

	appOpts := application.Options{
		Name:        appdata.AppName(),
		Description: "Local-first RSS reader with optional vector search",
		Services: []application.Service{
			application.NewService(settingsAPI),
			application.NewService(searchAPI),
			application.NewService(feedAPI),
			application.NewService(articleAPI),
			application.NewService(aiAPI),
			application.NewService(briefingAPI),
			application.NewService(syncAPI),
			application.NewService(updateAPI),
			application.NewService(ns),
			application.NewService(&GreetService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	}
	// Hardware acceleration off → disable GPU compositing (takes effect this launch).
	if !uiPrefs.HardwareAcceleration {
		appOpts.Windows.AdditionalBrowserArgs = append(
			appOpts.Windows.AdditionalBrowserArgs,
			"--disable-gpu",
			"--disable-gpu-compositing",
		)
		log.Printf("hardware acceleration disabled (WebView2 --disable-gpu)")
	}
	if wvDir, err := appdata.WebViewUserDataDir(); err != nil {
		log.Printf("webview user data dir: %v", err)
	} else if wvDir != "" {
		appOpts.Windows.WebviewUserDataPath = wvDir
		log.Printf("webview user data: %s", wvDir)
	}
	app := application.New(appOpts)

	winOpts := application.WebviewWindowOptions{
		Title:  appdata.DisplayName(),
		Width:  1280,
		Height: 800,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(246, 247, 249),
		URL:              "/",
	}
	// Win11 22H2+: DWM Mica behind a translucent WebView. Needs GPU compositing.
	if desktop.ApplyMica(&winOpts, uiPrefs.HardwareAcceleration, uiPrefs.MicaBackdrop, uiPrefs.Theme) {
		log.Printf("windows mica backdrop enabled")
	}
	window := app.Window.NewWithOptions(winOpts)
	settingsAPI.SetOnUIPrefs(func(prefs settings.UIPrefs) {
		desktop.ApplyWindowMicaFrom(window, prefs.MicaBackdrop && prefs.HardwareAcceleration)
	})

	beginQuit := desktop.Setup(app, window, trayIcon, desktop.Hooks{
		RefreshAll: func() error {
			_, err := feedAPI.RefreshAll()
			return err
		},
		SetWebAccess: func(enabled bool) error {
			ctx := context.Background()
			cfg, err := store.LoadWebAccessConfig(ctx)
			if err != nil {
				return err
			}
			cfg.Enabled = enabled
			saved, err := store.SetWebAccessConfig(ctx, cfg)
			if err != nil {
				return err
			}
			_, err = webServer.Apply(ctx, saved)
			return err
		},
		WebAccessEnabled: func() bool {
			if webServer.Status().Running {
				return true
			}
			cfg, err := store.LoadWebAccessConfig(context.Background())
			return err == nil && cfg.Enabled
		},
		Locale: func() string {
			prefs, err := store.LoadUIPrefs(context.Background())
			if err != nil || prefs.Locale == "" {
				return "zh-CN"
			}
			return prefs.Locale
		},
	})
	updateAPI.SetQuit(func() {
		beginQuit()
		app.Quit()
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

	// After UI exits: optional AI cache cleanup (Settings → Advanced).
	quitPrefs, err := store.LoadUIPrefs(context.Background())
	if err != nil {
		log.Printf("quit: load UI prefs: %v", err)
		return
	}
	if quitPrefs.ClearCacheOnQuit {
		cache := &llm.Cache{DB: database.SQL}
		n, cerr := cache.Clear(context.Background())
		if cerr != nil {
			log.Printf("quit: clear LLM cache: %v", cerr)
		} else {
			log.Printf("quit: cleared LLM cache (%d rows)", n)
		}
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
func runAutoRefresh(ctx context.Context, library *service.Library, store *settings.Store, notifier *notify.Sender, briefing *service.BriefingWorker) {
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
			log.Printf("auto-refresh: error load config: %v", err)
			timer.Reset(30 * time.Second)
			continue
		}
		// Manual "Refresh All" uses the same paced queue even when auto-refresh is off.
		// When auto is on, also refresh interval-due feeds (staggered + capped).
		includeDue := cfg.AutoRefresh
		hasForce := library.ForceQueueLen() > 0
		hasFulltext := library.FulltextQueueLen() > 0
		if !includeDue && !hasForce && !hasFulltext {
			timer.Reset(30 * time.Second)
			continue
		}

		if includeDue || hasForce {
			hadForce := hasForce
			// Tick: short intervals (e.g. 5m) and force-queue batches progress.
			res, ok, err := library.TryRefreshWork(ctx, cfg.RefreshIntervalMinutes, includeDue)
			if err != nil {
				log.Printf("auto-refresh: error: %v", err)
			} else if !ok {
				log.Printf("auto-refresh: skipped (refresh already in progress)")
			} else if res.FeedsOK > 0 || res.FeedsErr > 0 || res.FeedsPending > 0 {
				log.Printf("auto-refresh: ok=%d err=%d added=%d pending=%d",
					res.FeedsOK, res.FeedsErr, res.ArticlesAdded, res.FeedsPending)
				if notifier != nil {
					notifier.AfterRefresh(ctx, res.ArticlesAdded)
				}
				if briefing != nil && hadForce && library.ForceQueueLen() == 0 {
					briefing.NotifyForceQueueEmpty()
				}
			}
		}

		// Full-content drain: independent budget; never holds the multi-feed batch lock.
		if library.FulltextQueueLen() > 0 {
			ui, uerr := store.LoadUIPrefs(ctx)
			if uerr == nil && ui.FetchFullContent {
				okN, failN, pending := library.TryDrainFulltext(ctx, 0)
				if okN > 0 || failN > 0 || pending > 0 {
					log.Printf("fulltext-queue: ok=%d err=%d pending=%d", okN, failN, pending)
				}
			}
		}

		// Drain force / fulltext faster when work remains.
		if library.ForceQueueLen() > 0 || library.FulltextQueueLen() > 0 {
			timer.Reset(15 * time.Second)
		} else {
			timer.Reset(time.Minute)
		}
	}
}

func runKeepDrain(ctx context.Context, w *service.KeepWorker) {
	if w == nil {
		return
	}
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			did, err := w.TryJudge(ctx)
			if err != nil {
				log.Printf("smart filter: %v", err)
			} else if did {
				log.Printf("smart filter: judged a batch")
			}
		}
	}
}

func runBriefingDrain(ctx context.Context, w *service.BriefingWorker) {
	if w == nil {
		return
	}
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			did, err := w.TryGenerate(ctx)
			if err != nil {
				log.Printf("briefing generate: %v", err)
			} else if did {
				log.Printf("briefing: generated")
			}
		}
	}
}
