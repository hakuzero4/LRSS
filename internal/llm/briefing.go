package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"

	"lrss/internal/model"
)

const briefingSystemPromptBase = `You are a senior news editor writing a desk note for a busy reader. Your job is NOT to recap articles. Your job is to answer: "In this batch, what is actually valuable to know?"

What "valuable" means:
- A concrete change (shipped, leaked, banned, funded, broken, reversed)
- A number, date, name, or decision that would change what the reader does or believes
- A connection or contradiction across items (same event, two sources; claim vs counter-claim)
- A risk or deadline worth tracking

Forbidden:
- Restating or lightly paraphrasing titles
- One bullet per article ("catalog mode")
- "文章介绍了 / This article discusses / 指出 / 强调了重要性" with no new fact
- Repeating the same insight under two themes
- Starting with "本汇报 / This briefing"

Rules:
- overview = 4–7 sentences of synthesis: the shift in the batch, not a table of contents.
- 3–6 themes max. Each theme is a development, not a folder name for leftover titles.
- 8–12 bullets total across the whole briefing. Merge coverage of the same event into ONE bullet and cite every relevant [n].
- Each point must stand alone without the title: fact + so-what. If you delete the source title, the point must still make sense.
- Cite only with input numbers like [3] or [3][7]. Never invent numbers. Never output URLs or article IDs.
- Skip TOC/sponsor/listicles/duplicates with no extra fact.
- watch = array of objects {n, point} only. Never a string. Never an array of strings. Empty [] is fine.
- Output ONLY valid JSON. No markdown fences.`

// BriefingItem is one numbered input article for the briefing prompt.
type BriefingItem struct {
	Index     int
	Title     string
	Feed      string
	Published string
	Summary   string
}

type briefingBulletJSON struct {
	N     []int
	Point string
}

type briefingThemeJSON struct {
	Title   string
	Bullets []briefingBulletJSON
}

type briefingModelJSON struct {
	Overview string
	Themes   []briefingThemeJSON
	Watch    []briefingBulletJSON
}

var (
	citeBracketRe = regexp.MustCompile(`\[(\d+)\]`)
	citeCjkRe     = regexp.MustCompile(`【(\d+)】`)
)

// UserPromptBriefing builds the numbered item list for FeatureBriefing.
func UserPromptBriefing(items []BriefingItem, locale string) string {
	var b strings.Builder
	b.WriteString("Locale: ")
	b.WriteString(NormalizeUILocale(locale))
	b.WriteString("\nCount: ")
	b.WriteString(fmt.Sprintf("%d", len(items)))
	b.WriteString("\n\nItems:\n")
	for _, it := range items {
		n := it.Index
		if n <= 0 {
			n = 1
		}
		b.WriteString(fmt.Sprintf("[%d]\n", n))
		b.WriteString("Title: ")
		b.WriteString(strings.TrimSpace(it.Title))
		b.WriteByte('\n')
		b.WriteString("Feed: ")
		b.WriteString(strings.TrimSpace(it.Feed))
		b.WriteByte('\n')
		pub := strings.TrimSpace(it.Published)
		if pub == "" {
			pub = "unknown"
		}
		b.WriteString("Published: ")
		b.WriteString(pub)
		b.WriteByte('\n')
		b.WriteString("Summary: ")
		b.WriteString(truncateRunes(strings.TrimSpace(it.Summary), 220))
		b.WriteString("\n\n")
	}
	if NormalizeUILocale(locale) == "zh" {
		b.WriteString("任务：不要逐条复述标题。先判断这批里什么值得知道（变化、数字、冲突、该跟进的事），再合并重复报道。一条要点可引用多篇 [n]。禁止改写标题当要点。\n")
	} else {
		b.WriteString("Task: Do not recap titles. Decide what in THIS BATCH is worth knowing (changes, numbers, conflicts, follow-ups). Merge duplicate coverage. One point may cite several [n]. Never paraphrase a title as the point.\n")
	}
	b.WriteString(`JSON only:
{"overview":"synthesis, not a catalog","themes":[{"title":"development","bullets":[{"n":[1,4],"point":"fact + so-what"}]}],"watch":[{"n":[2],"point":"hearing Friday"}]}
watch must be [] or [{n,point},...]. Never "watch":"text" or "watch":["text"].
`)
	b.WriteString(OutputLanguageInstruction(locale))
	return b.String()
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

// StripJSONFence removes optional ```json ... ``` wrappers.
func StripJSONFence(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "json") {
		s = strings.TrimSpace(s[4:])
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// BriefingSource is the article a 1-based prompt index maps to.
type BriefingSource struct {
	ID, Title, FeedTitle string
}

// ParseAndMapBriefing maps model JSON onto BriefingPayload.
// Unknown n values are dropped. Empty themes after mapping is an error.
// Local models often emit watch/bullets as strings; those are coerced or
// dropped. A malformed watch field never fails the whole briefing.
func ParseAndMapBriefing(raw string, byIndex map[int]BriefingSource) (model.BriefingPayload, error) {
	raw = extractJSONObject(raw)
	if raw == "" {
		return model.BriefingPayload{}, fmt.Errorf("empty briefing json")
	}
	var loose struct {
		Overview json.RawMessage `json:"overview"`
		Themes   json.RawMessage `json:"themes"`
		Watch    json.RawMessage `json:"watch"`
	}
	if err := unmarshalBriefingJSON(raw, &loose); err != nil {
		return model.BriefingPayload{}, fmt.Errorf("briefing json: %w", err)
	}
	parsed := briefingModelJSON{
		Overview: parseOverview(loose.Overview),
		Themes:   parseThemes(loose.Themes),
		Watch:    parseBulletList(loose.Watch),
	}
	return mapParsedBriefing(parsed, func(n int) (BriefingSource, bool) {
		r, ok := byIndex[n]
		return r, ok
	})
}

func extractJSONObject(raw string) string {
	raw = StripJSONFence(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return strings.TrimSpace(raw[start : end+1])
	}
	return raw
}

func unmarshalBriefingJSON(raw string, dest any) error {
	if err := json.Unmarshal([]byte(raw), dest); err == nil {
		return nil
	} else {
		first := err
		if soft := stripTrailingCommas(raw); soft != raw {
			if err2 := json.Unmarshal([]byte(soft), dest); err2 == nil {
				return nil
			}
		}
		return first
	}
}

func stripTrailingCommas(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			b.WriteByte(c)
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\n' || s[j] == '\r' || s[j] == '\t') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func parseOverview(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	if len(raw) > 0 && raw[0] == '[' {
		var parts []string
		if err := json.Unmarshal(raw, &parts); err == nil {
			return strings.TrimSpace(strings.Join(parts, " "))
		}
	}
	return ""
}

func parseThemes(raw json.RawMessage) []briefingThemeJSON {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" || raw[0] != '[' {
		return nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil
	}
	var out []briefingThemeJSON
	for _, el := range elems {
		el = bytes.TrimSpace(el)
		if len(el) == 0 || el[0] != '{' {
			continue
		}
		var obj struct {
			Title   string          `json:"title"`
			Name    string          `json:"name"`
			Theme   string          `json:"theme"`
			Bullets json.RawMessage `json:"bullets"`
			Items   json.RawMessage `json:"items"`
			Points  json.RawMessage `json:"points"`
		}
		if err := json.Unmarshal(el, &obj); err != nil {
			continue
		}
		title := strings.TrimSpace(firstFilled(obj.Title, obj.Name, obj.Theme))
		if title == "" {
			continue
		}
		bullets := parseBulletList(obj.Bullets)
		if len(bullets) == 0 {
			bullets = parseBulletList(obj.Items)
		}
		if len(bullets) == 0 {
			bullets = parseBulletList(obj.Points)
		}
		out = append(out, briefingThemeJSON{Title: title, Bullets: bullets})
	}
	return out
}

func parseBulletList(raw json.RawMessage) []briefingBulletJSON {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []briefingBulletJSON
	switch raw[0] {
	case '[':
		var elems []json.RawMessage
		if err := json.Unmarshal(raw, &elems); err != nil {
			return nil
		}
		for _, el := range elems {
			n, point, ok := coerceBullet(el)
			if !ok {
				continue
			}
			out = append(out, briefingBulletJSON{N: n, Point: point})
		}
	case '{', '"':
		n, point, ok := coerceBullet(raw)
		if ok {
			out = append(out, briefingBulletJSON{N: n, Point: point})
		}
	}
	return out
}

func coerceBullet(raw json.RawMessage) (n []int, point string, ok bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, "", false
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, "", false
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, "", false
		}
		return extractCiteNs(s), s, true
	}
	if raw[0] != '{' {
		return nil, "", false
	}
	var obj struct {
		N     json.RawMessage `json:"n"`
		Ns    json.RawMessage `json:"ns"`
		Index json.RawMessage `json:"index"`
		Point string          `json:"point"`
		Text  string          `json:"text"`
		Item  string          `json:"item"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, "", false
	}
	point = strings.TrimSpace(firstFilled(obj.Point, obj.Text, obj.Item))
	ns := parseNField(obj.N)
	if len(ns) == 0 {
		ns = parseNField(obj.Ns)
	}
	if len(ns) == 0 {
		ns = parseNField(obj.Index)
	}
	if len(ns) == 0 {
		ns = extractCiteNs(point)
	}
	if point == "" {
		return nil, "", false
	}
	return ns, point, true
}

func parseNField(raw json.RawMessage) []int {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var ints []int
	if err := json.Unmarshal(raw, &ints); err == nil {
		return ints
	}
	if raw[0] == '[' {
		var elems []json.RawMessage
		if err := json.Unmarshal(raw, &elems); err == nil {
			var out []int
			for _, el := range elems {
				out = append(out, parseNField(el)...)
			}
			return out
		}
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return []int{n}
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return []int{int(f)}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if ns := extractCiteNs(s); len(ns) > 0 {
			return ns
		}
		return parseBareInts(s)
	}
	return nil
}

func extractCiteNs(s string) []int {
	var out []int
	seen := map[int]bool{}
	add := func(n int) {
		if n <= 0 || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	for _, m := range citeBracketRe.FindAllStringSubmatch(s, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			add(n)
		}
	}
	for _, m := range citeCjkRe.FindAllStringSubmatch(s, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			add(n)
		}
	}
	return out
}

func parseBareInts(s string) []int {
	var out []int
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == ' '
	}) {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func firstFilled(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func mapParsedBriefing(parsed briefingModelJSON, lookup func(int) (BriefingSource, bool)) (model.BriefingPayload, error) {
	scrub := bluemonday.StrictPolicy()
	out := model.BriefingPayload{
		Overview: strings.TrimSpace(scrub.Sanitize(parsed.Overview)),
		Themes:   make([]model.BriefingTheme, 0, len(parsed.Themes)),
		Watch:    nil,
	}
	for _, th := range parsed.Themes {
		theme := model.BriefingTheme{Title: strings.TrimSpace(scrub.Sanitize(th.Title))}
		if theme.Title == "" {
			continue
		}
		for _, bu := range th.Bullets {
			point := strings.TrimSpace(scrub.Sanitize(bu.Point))
			if point == "" {
				continue
			}
			refs := resolveRefs(bu.N, lookup)
			if len(refs) == 0 {
				continue
			}
			cites := make([]model.BriefingCite, 0, len(refs))
			for _, r := range refs {
				cites = append(cites, model.BriefingCite{ArticleID: r.ID, Title: r.Title, FeedTitle: r.FeedTitle})
			}
			theme.Bullets = append(theme.Bullets, model.BriefingBullet{
				Point:     point,
				ArticleID: refs[0].ID,
				Title:     refs[0].Title,
				FeedTitle: refs[0].FeedTitle,
				Cites:     cites,
			})
		}
		if len(theme.Bullets) > 0 {
			out.Themes = append(out.Themes, theme)
		}
	}
	for _, bu := range parsed.Watch {
		point := strings.TrimSpace(scrub.Sanitize(bu.Point))
		if point == "" {
			continue
		}
		refs := resolveRefs(bu.N, lookup)
		if len(refs) == 0 {
			continue
		}
		out.Watch = append(out.Watch, model.BriefingBullet{
			ArticleID: refs[0].ID,
			Title:     refs[0].Title,
			FeedTitle: refs[0].FeedTitle,
			Point:     point,
		})
	}
	if len(out.Themes) == 0 {
		return model.BriefingPayload{}, fmt.Errorf("briefing themes empty")
	}
	return out, nil
}

func resolveRefs(ns []int, lookup func(int) (BriefingSource, bool)) []BriefingSource {
	seen := map[int]bool{}
	var out []BriefingSource
	for _, n := range ns {
		if n <= 0 || seen[n] {
			continue
		}
		r, ok := lookup(n)
		if !ok || strings.TrimSpace(r.ID) == "" {
			continue
		}
		seen[n] = true
		out = append(out, r)
	}
	return out
}

// Brief asks the configured model for a batch digest. Not cached (each batch is unique).
func (s *Service) Brief(ctx context.Context, items []BriefingItem, locale string) (FeatureResult, error) {
	if len(items) == 0 {
		return FeatureResult{}, fmt.Errorf("no briefing items")
	}
	cfg, err := s.loadCfg(ctx)
	if err != nil {
		return FeatureResult{}, err
	}
	// Stream so proxies send headers on first token (non-stream waits for the
	// whole JSON and often hits "timeout awaiting response headers").
	client, err := NewClientWithTimeout(cfg, 4*time.Minute)
	if err != nil {
		return FeatureResult{}, err
	}
	res, err := client.ChatStream(ctx, ChatRequest{
		Messages: []Message{
			{Role: "system", Content: SystemPromptFor(FeatureBriefing, locale)},
			{Role: "user", Content: UserPromptBriefing(items, locale)},
		},
	}, nil)
	if err != nil {
		return FeatureResult{}, err
	}
	if err := RejectIfIncomplete(res.FinishReason); err != nil {
		return FeatureResult{}, err
	}
	md := strings.TrimSpace(res.Content)
	if md == "" {
		return FeatureResult{}, fmt.Errorf("llm returned empty content")
	}
	modelName := res.Model
	if modelName == "" {
		modelName = client.model
	}
	return FeatureResult{
		Markdown: md,
		Feature:  FeatureBriefing,
		Model:    modelName,
	}, nil
}
