package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"
)

// APIDeps are deps for the browser API (management CRUD still omitted).
type APIDeps struct {
	Library *service.Library
	Store   *settings.Store
	Search  *search.Service
	// AI powers reader toolbar: summarize / translate / ask / fulltext helpers.
	AI AI
	// Briefing is optional (desktop worker). Web is read + star/read only.
	Briefing *service.BriefingWorker
	// Keep is optional (desktop smart-filter worker).
	Keep *service.KeepWorker
}

func (d APIDeps) excludeNsfw(ctx context.Context) bool {
	if d.Store == nil {
		return false
	}
	prefs, err := d.Store.LoadUIPrefs(ctx)
	if err != nil {
		return false
	}
	return !prefs.NsfwMode
}

func isSmartCollection(collection string) bool {
	c := strings.TrimSpace(collection)
	if strings.HasPrefix(c, "kept:") {
		return true
	}
	switch c {
	case "", "unread", "today", "starred", "all", "recent", "kept":
		return true
	default:
		return false
	}
}

func (s *Server) mountAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("GET /api/activity", s.handleJobActivity)
	mux.HandleFunc("GET /api/folders", s.handleFolders)
	mux.HandleFunc("GET /api/feeds", s.handleFeeds)
	mux.HandleFunc("GET /api/articles", s.handleListArticles)
	mux.HandleFunc("GET /api/articles/{id}", s.handleGetArticle)
	mux.HandleFunc("GET /api/smart-counts", s.handleSmartCounts)
	mux.HandleFunc("GET /api/ui-prefs", s.handleUIPrefs)
	mux.HandleFunc("GET /api/library-config", s.handleLibraryConfig)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("POST /api/articles/{id}/read", s.handleSetRead)
	mux.HandleFunc("POST /api/articles/{id}/star", s.handleSetStarred)
	mux.HandleFunc("POST /api/articles/{id}/keep", s.handleSetKeep)
	mux.HandleFunc("POST /api/articles/{id}/keep-folder", s.handleSetKeepFolder)
	mux.HandleFunc("POST /api/articles/scan-keep", s.handleScanKeep)
	mux.HandleFunc("GET /api/keep-folders", s.handleListKeepFolders)
	mux.HandleFunc("POST /api/keep-folders", s.handleCreateKeepFolder)
	mux.HandleFunc("POST /api/keep-folders/{id}/rename", s.handleRenameKeepFolder)
	mux.HandleFunc("POST /api/keep-folders/{id}/delete", s.handleDeleteKeepFolder)
	mux.HandleFunc("POST /api/articles/{id}/opened", s.handleRecordOpened)
	mux.HandleFunc("POST /api/articles/mark-all-read", s.handleMarkAllRead)
	mux.HandleFunc("GET /api/briefings", s.handleListBriefings)
	mux.HandleFunc("GET /api/briefings/{id}", s.handleGetBriefing)
	mux.HandleFunc("POST /api/briefings/{id}/read", s.handleSetBriefingRead)
	mux.HandleFunc("POST /api/briefings/{id}/star", s.handleSetBriefingStar)
	mux.HandleFunc("POST /api/briefings/{id}/retry", s.handleRetryBriefing)
	mux.HandleFunc("POST /api/briefings/clear-unstarred", s.handleClearUnstarredBriefings)
	mux.HandleFunc("DELETE /api/briefings/{id}", s.handleDeleteBriefing)
	mux.HandleFunc("POST /api/briefings/{id}/delete", s.handleDeleteBriefing)

	// Reader toolbar tools (mirror desktop appsvc when enabled in UIPrefs.readerToolbar)
	mux.HandleFunc("POST /api/articles/{id}/fetch-full", s.handleFetchFull)
	mux.HandleFunc("GET /api/ai/configured", s.handleAIConfigured)
	mux.HandleFunc("POST /api/ai/summarize", s.handleAISummarize)
	mux.HandleFunc("POST /api/ai/translate", s.handleAITranslate)
	mux.HandleFunc("POST /api/ai/translate-selection", s.handleAITranslateSelection)
	mux.HandleFunc("POST /api/ai/ask", s.handleAIAsk)
	mux.HandleFunc("POST /api/ai/suggest-folders", s.handleAISuggest)
	mux.HandleFunc("POST /api/ai/classify", s.handleAIClassify)
	mux.HandleFunc("POST /api/ai/detect-fullness", s.handleAIDetectFullness)
	mux.HandleFunc("POST /api/ai/ensure-full", s.handleAIEnsureFull)
	mux.HandleFunc("POST /api/ai/clear-translation", s.handleAIClearTranslation)
	mux.HandleFunc("POST /api/ai/apply-folder", s.handleAIApplyFolder)
	mux.HandleFunc("GET /api/ai/chat", s.handleAIChatHistory)
	mux.HandleFunc("POST /api/ai/chat", s.handleAIChatSend)
	mux.HandleFunc("POST /api/ai/chat/clear", s.handleAIChatClear)
	mux.HandleFunc("POST /api/ai/chat/cancel", s.handleAIChatCancel)
	mux.HandleFunc("GET /api/ai/stream", s.handleAIStream)
}

func (s *Server) handleJobActivity(w http.ResponseWriter, r *http.Request) {
	var a service.JobActivity
	if s.deps.Library != nil {
		id, title, pending, queuedIDs := s.deps.Library.RefreshSnapshot()
		a.FeedID = id
		a.FeedTitle = title
		a.Pending = pending
		a.Refreshing = id != "" || pending > 0
		if len(queuedIDs) > 0 {
			titles := make([]string, 0, len(queuedIDs))
			for _, qid := range queuedIDs {
				f, err := s.deps.Library.Feeds.Get(r.Context(), qid)
				if err != nil {
					continue
				}
				t := strings.TrimSpace(f.Title)
				if t == "" {
					t = f.FeedURL
				}
				if t != "" {
					titles = append(titles, t)
				}
			}
			a.QueuedTitles = titles
		}
	}
	if s.deps.Briefing != nil {
		a.BriefingState, a.BriefingPending, a.BriefingArticles = s.deps.Briefing.Snapshot()
	}
	if s.deps.Keep != nil {
		a.KeepState, a.KeepPending, a.KeepLast = s.deps.Keep.Snapshot()
	}
	if s.deps.Library != nil {
		a.ArticlesAdded = s.deps.Library.InsertedTotal()
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":     "web",
		"readOnly": false, // star + read allowed; management blocked
		"web":      true,
		// managementReadOnly signals frontend to hide settings/feed CRUD
		"managementReadOnly": true,
	})
}

func (s *Server) handleFolders(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	list, err := s.deps.Library.ListFolders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []model.Folder{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleFeeds(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	list, err := s.deps.Library.ListFeeds(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []model.Feed{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListArticles(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	q := r.URL.Query()
	collection := q.Get("collection")
	if collection == "" {
		collection = "unread"
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	exclude := s.deps.excludeNsfw(r.Context()) && isSmartCollection(collection)
	list, err := s.deps.Library.ListArticles(r.Context(), collection, limit, offset, exclude)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []model.Article{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetArticle(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	art, err := s.deps.Library.GetArticle(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (s *Server) handleSmartCounts(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	counts, err := s.deps.Library.SmartCounts(r.Context(), s.deps.excludeNsfw(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, counts)
}

func (s *Server) handleUIPrefs(w http.ResponseWriter, r *http.Request) {
	if s.deps.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "store unavailable")
		return
	}
	prefs, err := s.deps.Store.LoadUIPrefs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleLibraryConfig(w http.ResponseWriter, r *http.Request) {
	if s.deps.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "store unavailable")
		return
	}
	cfg, err := s.deps.Store.LoadLibraryConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.deps.Search == nil {
		writeError(w, http.StatusServiceUnavailable, "search unavailable")
		return
	}
	q := r.URL.Query()
	query := q.Get("q")
	mode := q.Get("mode")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	res, err := s.deps.Search.Search(r.Context(), query, search.Options{
		Mode:        mode,
		Limit:       limit,
		ExcludeNsfw: s.deps.excludeNsfw(r.Context()),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRecordOpened(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	keep := 50
	if s.deps.Store != nil {
		if prefs, err := s.deps.Store.LoadUIPrefs(r.Context()); err == nil {
			keep = prefs.RecentReadLimit
		}
	}
	if err := s.deps.Library.RecordOpened(r.Context(), id, keep); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListBriefings(w http.ResponseWriter, r *http.Request) {
	if s.deps.Briefing == nil {
		writeJSON(w, http.StatusOK, []model.Briefing{})
		return
	}
	list, err := s.deps.Briefing.List(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []model.Briefing{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetBriefing(w http.ResponseWriter, r *http.Request) {
	if s.deps.Briefing == nil {
		writeError(w, http.StatusServiceUnavailable, "briefings unavailable")
		return
	}
	id := r.PathValue("id")
	b, err := s.deps.Briefing.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleSetBriefingRead(w http.ResponseWriter, r *http.Request) {
	if s.deps.Briefing == nil {
		writeError(w, http.StatusServiceUnavailable, "briefings unavailable")
		return
	}
	var body struct {
		Read bool `json:"read"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.deps.Briefing.SetRead(r.Context(), r.PathValue("id"), body.Read); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetBriefingStar(w http.ResponseWriter, r *http.Request) {
	if s.deps.Briefing == nil {
		writeError(w, http.StatusServiceUnavailable, "briefings unavailable")
		return
	}
	var body struct {
		Starred bool `json:"starred"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.deps.Briefing.SetStarred(r.Context(), r.PathValue("id"), body.Starred); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteBriefing(w http.ResponseWriter, r *http.Request) {
	if s.deps.Briefing == nil {
		writeError(w, http.StatusServiceUnavailable, "briefings unavailable")
		return
	}
	id := r.PathValue("id")
	if err := s.deps.Briefing.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repo.ErrStarredProtected) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleClearUnstarredBriefings(w http.ResponseWriter, r *http.Request) {
	if s.deps.Briefing == nil {
		writeError(w, http.StatusServiceUnavailable, "briefings unavailable")
		return
	}
	n, err := s.deps.Briefing.DeleteUnstarred(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
}

func (s *Server) handleRetryBriefing(w http.ResponseWriter, r *http.Request) {
	if s.deps.Briefing == nil {
		writeError(w, http.StatusServiceUnavailable, "briefings unavailable")
		return
	}
	id := r.PathValue("id")
	if err := s.deps.Briefing.Retry(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	b, err := s.deps.Briefing.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleSetRead(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Read bool `json:"read"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.deps.Library.SetRead(r.Context(), id, body.Read); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetKeep(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil || s.deps.Library.Articles == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Keep bool `json:"keep"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var err error
	if body.Keep {
		err = s.deps.Library.Articles.Keep(r.Context(), id, "", "manual", 1, nil)
	} else {
		err = s.deps.Library.Articles.Unkeep(r.Context(), id)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetKeepFolder(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil || s.deps.Library.Articles == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	id := r.PathValue("id")
	var body struct {
		FolderID string `json:"folderId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.deps.Library.Articles.SetKeepFolder(r.Context(), id, body.FolderID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListKeepFolders(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil || s.deps.Library.KeepFolders == nil {
		writeJSON(w, http.StatusOK, []model.KeepFolder{})
		return
	}
	list, err := s.deps.Library.KeepFolders.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []model.KeepFolder{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateKeepFolder(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil || s.deps.Library.KeepFolders == nil {
		writeError(w, http.StatusServiceUnavailable, "keep folders unavailable")
		return
	}
	var body struct {
		Name     string `json:"name"`
		ParentID string `json:"parentId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	f, err := s.deps.Library.KeepFolders.Create(r.Context(), body.Name, body.ParentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleRenameKeepFolder(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil || s.deps.Library.KeepFolders == nil {
		writeError(w, http.StatusServiceUnavailable, "keep folders unavailable")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.deps.Library.KeepFolders.Rename(r.Context(), id, body.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteKeepFolder(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil || s.deps.Library.KeepFolders == nil {
		writeError(w, http.StatusServiceUnavailable, "keep folders unavailable")
		return
	}
	id := r.PathValue("id")
	if err := s.deps.Library.KeepFolders.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleScanKeep(w http.ResponseWriter, r *http.Request) {
	if s.deps.Keep == nil {
		writeError(w, http.StatusServiceUnavailable, "smart filter unavailable")
		return
	}
	n, err := s.deps.Keep.EnqueueUnread(r.Context(), 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queued": n})
}

func (s *Server) handleSetStarred(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Starred bool `json:"starred"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.deps.Library.SetStarred(r.Context(), id, body.Starred); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	var body struct {
		Collection string `json:"collection"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Collection == "" {
		body.Collection = "unread"
	}
	exclude := s.deps.excludeNsfw(r.Context())
	if err := s.deps.Library.MarkAllRead(r.Context(), body.Collection, exclude); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleFetchFull(w http.ResponseWriter, r *http.Request) {
	if s.deps.Library == nil {
		writeError(w, http.StatusServiceUnavailable, "library unavailable")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	// Long timeout for remote page fetch (desktop appsvc uses 50s).
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Second)
	defer cancel()
	art, err := s.deps.Library.FetchFullContent(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, art)
}

func (s *Server) requireAI(w http.ResponseWriter) AI {
	if s.deps.AI == nil {
		writeError(w, http.StatusServiceUnavailable, "ai unavailable")
		return nil
	}
	return s.deps.AI
}

func (s *Server) handleAIConfigured(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	ok, err := ai.IsLLMConfigured()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": ok})
}

func (s *Server) handleAISummarize(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
		Locale    string `json:"locale"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	res, err := ai.Summarize(body.ArticleID, body.Locale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAITranslate(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID  string `json:"articleId"`
		TargetLang string `json:"targetLang"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	res, err := ai.Translate(body.ArticleID, body.TargetLang)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAITranslateSelection(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		Text       string `json:"text"`
		TargetLang string `json:"targetLang"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Text) == "" {
		writeError(w, http.StatusBadRequest, "text required")
		return
	}
	res, err := ai.TranslateSelection(body.Text, body.TargetLang)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAIAsk(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
		Question  string `json:"question"`
		Locale    string `json:"locale"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	res, err := ai.Ask(body.ArticleID, body.Question, body.Locale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAISuggest(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
		Locale    string `json:"locale"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	res, err := ai.SuggestFolders(body.ArticleID, body.Locale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAIClassify(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
		Locale    string `json:"locale"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	res, err := ai.ClassifyPromo(body.ArticleID, body.Locale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAIDetectFullness(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	res, err := ai.DetectContentFullness(body.ArticleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAIEnsureFull(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	res, err := ai.EnsureFullContent(body.ArticleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAIClearTranslation(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	if err := ai.ClearTranslation(body.ArticleID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAIApplyFolder(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
		FolderID  string `json:"folderId"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.ArticleID) == "" {
		writeError(w, http.StatusBadRequest, "articleId required")
		return
	}
	if err := ai.ApplySuggestedFolder(body.ArticleID, body.FolderID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
