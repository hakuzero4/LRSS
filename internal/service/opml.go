package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"lrss/internal/model"
	"lrss/internal/opml"
)

const maxOPMLImportErrors = 20

// OPMLImportResult summarizes an OPML import batch.
type OPMLImportResult struct {
	FoldersCreated int      `json:"foldersCreated"`
	FeedsAdded     int      `json:"feedsAdded"`
	FeedsUpdated   int      `json:"feedsUpdated"` // existing URL: folder/title/site synced from OPML
	FeedsSkipped   int      `json:"feedsSkipped"` // existing URL unchanged
	FeedsFailed    int      `json:"feedsFailed"`
	Errors         []string `json:"errors"`
	// AddedFeedIDs are newly inserted feed IDs (not skipped). Callers can refresh
	// them asynchronously for progress UI; empty when none added.
	AddedFeedIDs []string `json:"addedFeedIds"`
}

// ImportOPML parses OPML XML, creates folders/feeds, optionally refreshes new feeds.
// Existing feed URLs are merged: folder placement (and unlocked title / empty site URL)
// are updated from the OPML outline. Folders with the same name under the same
// parent are reused (case-insensitive) so re-import does not duplicate the tree.
//
// Prefer fetch=false from the UI, then refresh AddedFeedIDs with progress. When
// fetch=true, every new feed is refreshed in-process (can take a long time).
func (lib *Library) ImportOPML(ctx context.Context, xml string, fetch bool) (OPMLImportResult, error) {
	doc, err := opml.Parse([]byte(xml))
	if err != nil {
		return OPMLImportResult{}, fmt.Errorf("import opml: %w", err)
	}

	var res OPMLImportResult
	res.Errors = []string{}
	res.AddedFeedIDs = []string{}

	// Collect newly added feed IDs for optional refresh.
	var addedIDs []string

	for i := range doc.Outlines {
		lib.walkImportOutline(ctx, &doc.Outlines[i], nil, &res, &addedIDs)
	}

	res.AddedFeedIDs = addedIDs

	if fetch && len(addedIDs) > 0 {
		// Avoid overlapping with auto-refresh / RefreshAll.
		lib.refreshMu.Lock()
		defer lib.refreshMu.Unlock()

		for _, id := range addedIDs {
			if ctx.Err() != nil {
				res.FeedsFailed++
				res.appendError(fmt.Sprintf("refresh cancelled: %v", ctx.Err()))
				break
			}
			if _, err := lib.refreshOneByID(ctx, id); err != nil {
				res.FeedsFailed++
				res.appendError(fmt.Sprintf("refresh %s: %v", id, err))
			}
		}
	}

	return res, nil
}

func (lib *Library) walkImportOutline(
	ctx context.Context,
	o *opml.Outline,
	parentFolderID *string,
	res *OPMLImportResult,
	addedIDs *[]string,
) {
	if o == nil {
		return
	}

	// Feed outline.
	if strings.TrimSpace(o.XMLURL) != "" {
		lib.importFeedOutline(ctx, o, parentFolderID, res, addedIDs)
		return
	}

	// Folder: has children (parser already drops empty folders).
	if len(o.Children) == 0 {
		return
	}

	name := strings.TrimSpace(o.Text)
	if name == "" {
		name = strings.TrimSpace(o.Title)
	}
	if name == "" {
		name = "Folder"
	}

	// Reuse existing folder with same name under same parent (OPML re-import).
	folder, err := lib.Folders.FindByNameAndParent(ctx, name, parentFolderID)
	if err != nil && err != sql.ErrNoRows {
		res.appendError(fmt.Sprintf("lookup folder %q: %v", name, err))
		for i := range o.Children {
			lib.walkImportOutline(ctx, &o.Children[i], parentFolderID, res, addedIDs)
		}
		return
	}
	if err == sql.ErrNoRows {
		folder, err = lib.CreateFolder(ctx, name, parentFolderID)
		if err != nil {
			res.appendError(fmt.Sprintf("create folder %q: %v", name, err))
			// Still try children under the parent so import continues.
			for i := range o.Children {
				lib.walkImportOutline(ctx, &o.Children[i], parentFolderID, res, addedIDs)
			}
			return
		}
		res.FoldersCreated++
	}
	fid := folder.ID
	for i := range o.Children {
		lib.walkImportOutline(ctx, &o.Children[i], &fid, res, addedIDs)
	}
}

func (lib *Library) importFeedOutline(
	ctx context.Context,
	o *opml.Outline,
	folderID *string,
	res *OPMLImportResult,
	addedIDs *[]string,
) {
	feedURL := strings.TrimSpace(o.XMLURL)
	if err := validateFeedURL(feedURL); err != nil {
		res.FeedsFailed++
		res.appendError(fmt.Sprintf("invalid feed url %q: %v", feedURL, err))
		return
	}

	title := strings.TrimSpace(o.Text)
	if title == "" {
		title = strings.TrimSpace(o.Title)
	}
	if title == "" {
		title = feedURL
	}

	var siteURL *string
	if s := strings.TrimSpace(o.HTMLURL); s != "" {
		siteURL = &s
	}
	var folder *string
	if folderID != nil && strings.TrimSpace(*folderID) != "" {
		folder = folderID
	}

	existing, err := lib.Feeds.GetByURL(ctx, feedURL)
	if err == nil {
		if lib.mergeExistingFeedFromOPML(ctx, existing, folder, title, siteURL, res) {
			res.FeedsUpdated++
		} else {
			res.FeedsSkipped++
		}
		return
	}
	if err != sql.ErrNoRows {
		res.FeedsFailed++
		res.appendError(fmt.Sprintf("lookup %s: %v", feedURL, err))
		return
	}

	feed := &model.Feed{
		FolderID: folder,
		Title:    title,
		SiteURL:  siteURL,
		FeedURL:  feedURL,
		IsPaused: false,
	}
	if err := lib.Feeds.Insert(ctx, feed); err != nil {
		res.FeedsFailed++
		res.appendError(fmt.Sprintf("insert %s: %v", feedURL, err))
		return
	}
	res.FeedsAdded++
	*addedIDs = append(*addedIDs, feed.ID)
}

// mergeExistingFeedFromOPML syncs folder / unlocked title / empty site URL from an
// OPML outline onto an already-subscribed feed. Returns true when anything changed.
func (lib *Library) mergeExistingFeedFromOPML(
	ctx context.Context,
	existing model.Feed,
	folder *string,
	title string,
	siteURL *string,
	res *OPMLImportResult,
) bool {
	changed := false

	if !sameOptionalID(existing.FolderID, folder) {
		if err := lib.Feeds.SetFolder(ctx, existing.ID, folder); err != nil {
			res.appendError(fmt.Sprintf("update folder %s: %v", existing.FeedURL, err))
		} else {
			changed = true
		}
	}

	// Do not override a user-renamed title.
	if !existing.TitleUserSet && title != "" && title != existing.Title {
		if err := lib.Feeds.SetDisplayTitle(ctx, existing.ID, title); err != nil {
			res.appendError(fmt.Sprintf("update title %s: %v", existing.FeedURL, err))
		} else {
			changed = true
		}
	}

	// Fill site URL when local is empty and OPML provides htmlUrl.
	if siteURL != nil && strings.TrimSpace(*siteURL) != "" {
		if existing.SiteURL == nil || strings.TrimSpace(*existing.SiteURL) == "" {
			if err := lib.Feeds.SetSiteURL(ctx, existing.ID, *siteURL); err != nil {
				res.appendError(fmt.Sprintf("update site %s: %v", existing.FeedURL, err))
			} else {
				changed = true
			}
		}
	}

	return changed
}

func sameOptionalID(a, b *string) bool {
	as, bs := "", ""
	if a != nil {
		as = strings.TrimSpace(*a)
	}
	if b != nil {
		bs = strings.TrimSpace(*b)
	}
	return as == bs
}

func (res *OPMLImportResult) appendError(msg string) {
	if len(res.Errors) >= maxOPMLImportErrors {
		return
	}
	res.Errors = append(res.Errors, msg)
}

// ExportOPML builds an OPML document from folders + feeds and returns XML text.
func (lib *Library) ExportOPML(ctx context.Context) (string, error) {
	folders, err := lib.Folders.List(ctx)
	if err != nil {
		return "", fmt.Errorf("export opml: list folders: %w", err)
	}
	feeds, err := lib.Feeds.List(ctx)
	if err != nil {
		return "", fmt.Errorf("export opml: list feeds: %w", err)
	}

	doc := buildOPMLDocument(folders, feeds)
	data, err := opml.Export(doc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func buildOPMLDocument(folders []model.Folder, feeds []model.Feed) *opml.Document {
	// childrenByParent maps parent folder id ("" for root) → folder IDs
	childrenByParent := map[string][]string{}
	folderByID := make(map[string]model.Folder, len(folders))
	for _, f := range folders {
		folderByID[f.ID] = f
		parentKey := ""
		if f.ParentID != nil && *f.ParentID != "" {
			parentKey = *f.ParentID
		}
		childrenByParent[parentKey] = append(childrenByParent[parentKey], f.ID)
	}

	// feeds by folder
	feedsByFolder := map[string][]model.Feed{} // "" = unfiled
	for _, feed := range feeds {
		key := ""
		if feed.FolderID != nil && *feed.FolderID != "" {
			key = *feed.FolderID
		}
		feedsByFolder[key] = append(feedsByFolder[key], feed)
	}

	var buildFolder func(folderID string) opml.Outline
	buildFolder = func(folderID string) opml.Outline {
		f := folderByID[folderID]
		o := opml.Outline{
			Text:  f.Name,
			Title: f.Name,
		}
		// Nested folders first.
		for _, childID := range childrenByParent[folderID] {
			o.Children = append(o.Children, buildFolder(childID))
		}
		// Feeds in this folder.
		for _, feed := range feedsByFolder[folderID] {
			o.Children = append(o.Children, feedToOutline(feed))
		}
		return o
	}

	doc := &opml.Document{
		Title:    "LRSS Subscriptions",
		Outlines: make([]opml.Outline, 0),
	}
	// Root folders.
	for _, id := range childrenByParent[""] {
		doc.Outlines = append(doc.Outlines, buildFolder(id))
	}
	// Unfiled feeds at root.
	for _, feed := range feedsByFolder[""] {
		doc.Outlines = append(doc.Outlines, feedToOutline(feed))
	}
	return doc
}

func feedToOutline(f model.Feed) opml.Outline {
	o := opml.Outline{
		Text:   f.Title,
		Title:  f.Title,
		Type:   "rss",
		XMLURL: f.FeedURL,
	}
	if f.SiteURL != nil {
		o.HTMLURL = *f.SiteURL
	}
	return o
}

// refreshOneByID loads a feed and refreshes it (used after OPML import).
func (lib *Library) refreshOneByID(ctx context.Context, feedID string) (int, error) {
	feed, err := lib.Feeds.Get(ctx, feedID)
	if err != nil {
		return 0, err
	}
	if feed.IsPaused {
		return 0, nil
	}
	return lib.refreshOne(ctx, feed)
}
