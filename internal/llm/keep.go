package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	StrictnessLoose    = "loose"
	StrictnessStandard = "standard"
	StrictnessStrict   = "strict"
)

const keepSystemPromptBase = `You are a senior RSS desk editor. Decide which items belong in a personal "worth reading" folder.

Two gates, BOTH required unless the interest profile is empty:
1) Quality — would opening this teach the reader one concrete new fact or a real argument? Reject ads, affiliate/soft-promo, SEO listicles, empty teasers, and duplicate coverage of the same event (keep only the most informative item in a batch).
2) Fit — match the reader's interest profile (affirmatives AND negatives). If the profile is empty, quality alone is enough.

Be conservative: unsure → keep=false. False positives pollute the folder; misses still appear in Unread.

Reply with ONLY valid JSON (no markdown fences required, but fences are ok).`

// KeepItem is one numbered article for FeatureKeep batch judgment.
type KeepItem struct {
	Index     int
	ID        string
	Title     string
	Feed      string
	Published string
	Summary   string
	Body      string
}

// KeepFolderRef is an existing 精选 subfolder the model may route into.
type KeepFolderRef struct {
	ID   string
	Name string
	Hint string
}

// KeepVerdict is the model (then threshold-adjusted) decision for one article.
// FolderID/FolderName empty means the 精选 root. Unknown names are cleared after match.
type KeepVerdict struct {
	ArticleID  string
	Keep       bool
	Confidence float64
	Reason     string
	Topics     []string
	FolderID   string
	FolderName string
	// ThresholdRejected is true when the model said keep but confidence was below the bar.
	ThresholdRejected bool
}

// MatchKeepFolder maps a model folder name onto an existing ref.
// Exact, case-insensitive; first match wins. Empty or unknown → "", "".
func MatchKeepFolder(name string, folders []KeepFolderRef) (id, matchedName string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	for _, f := range folders {
		n := strings.TrimSpace(f.Name)
		if n == "" {
			continue
		}
		if strings.EqualFold(n, name) {
			return f.ID, n
		}
	}
	return "", ""
}

// NormalizeKeepStrictness maps UI / stored values to loose|standard|strict.
func NormalizeKeepStrictness(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case StrictnessLoose, "宽松":
		return StrictnessLoose
	case StrictnessStrict, "严格":
		return StrictnessStrict
	default:
		return StrictnessStandard
	}
}

// KeepConfidenceThreshold is the minimum confidence to honor keep=true.
func KeepConfidenceThreshold(strictness string) float64 {
	switch NormalizeKeepStrictness(strictness) {
	case StrictnessLoose:
		return 0.55
	case StrictnessStrict:
		return 0.85
	default:
		return 0.70
	}
}

func keepItemNumber(it KeepItem, i int) int {
	if it.Index > 0 {
		return it.Index
	}
	return i + 1
}

// UserPromptKeep builds the numbered batch prompt for FeatureKeep.
func UserPromptKeep(items []KeepItem, profile, strictness, locale string, folders []KeepFolderRef) string {
	strictness = NormalizeKeepStrictness(strictness)
	thr := KeepConfidenceThreshold(strictness)
	var b strings.Builder
	b.WriteString("Locale: ")
	b.WriteString(NormalizeUILocale(locale))
	b.WriteString("\nCount: ")
	b.WriteString(fmt.Sprintf("%d", len(items)))
	b.WriteString("\nStrictness: ")
	b.WriteString(strictness)
	b.WriteString(fmt.Sprintf(" (threshold %.2f)", thr))
	b.WriteByte('\n')
	if p := strings.TrimSpace(profile); p == "" {
		b.WriteString("Interest profile: EMPTY PROFILE = quality-only\n")
	} else {
		b.WriteString("Interest profile:\n")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	b.WriteString("\nItems:\n")
	for i, it := range items {
		n := keepItemNumber(it, i)
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
		b.WriteString(BudgetText(it.Summary, 220))
		b.WriteByte('\n')
		b.WriteString("Body: ")
		b.WriteString(BudgetText(it.Body, 800))
		b.WriteString("\n\n")
	}
	if len(folders) == 0 {
		b.WriteString(`Task: JSON only:
{"items":[{"n":1,"keep":true,"confidence":0.82,"reason":"one short clause","topics":["tag"]}]}

Rules:
- reason ≤ 80 chars, no title paraphrase
- confidence 0..1
- strict: promo / unclear quality → keep false; fit must cite which profile clause
- standard: both gates explicit
- loose: quality pass + not clearly off-profile
- Same event in one batch: only the best item keep=true
`)
	} else {
		b.WriteString("Subfolders (pick at most one existing name, or leave folder empty for root):\n")
		for _, f := range folders {
			name := strings.TrimSpace(f.Name)
			if name == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(name)
			if hint := strings.TrimSpace(f.Hint); hint != "" {
				b.WriteString(" — ")
				b.WriteString(hint)
			}
			b.WriteByte('\n')
		}
		b.WriteString(`
Task: JSON only:
{"items":[{"n":1,"keep":true,"confidence":0.82,"reason":"...","topics":["t"],"folder":"Name or empty"}]}

Rules:
- reason ≤ 80 chars, no title paraphrase
- confidence 0..1
- strict: promo / unclear quality → keep false; fit must cite which profile clause
- standard: both gates explicit
- loose: quality pass + not clearly off-profile
- Same event in one batch: only the best item keep=true
- folder must be one of the listed names or empty/omit
- NEVER invent a folder name
- if unsure which folder, empty → root
- keep=false → omit folder
`)
	}
	b.WriteString(OutputLanguageInstruction(locale))
	return b.String()
}

// ParseKeepBatch maps model JSON onto verdicts. Unknown n is dropped.
// Invalid JSON or a missing items array yields an empty slice (not an error).
// Threshold is applied by JudgeKeepBatch, not here.
func ParseKeepBatch(raw string, byIndex map[int]KeepItem) []KeepVerdict {
	raw = extractJSONObject(raw)
	if raw == "" {
		return nil
	}
	var root struct {
		Items json.RawMessage `json:"items"`
	}
	if err := unmarshalBriefingJSON(raw, &root); err != nil {
		return nil
	}
	itemsRaw := bytes.TrimSpace(root.Items)
	if len(itemsRaw) == 0 || string(itemsRaw) == "null" {
		return nil
	}
	var elems []json.RawMessage
	switch itemsRaw[0] {
	case '[':
		if err := json.Unmarshal(itemsRaw, &elems); err != nil {
			soft := stripTrailingCommas(string(itemsRaw))
			if err2 := json.Unmarshal([]byte(soft), &elems); err2 != nil {
				return nil
			}
		}
	case '{':
		elems = []json.RawMessage{itemsRaw}
	default:
		return nil
	}

	seen := map[int]bool{}
	var out []KeepVerdict
	for _, el := range elems {
		n, v, ok := parseKeepElem(el)
		if !ok || n <= 0 || seen[n] {
			continue
		}
		it, known := byIndex[n]
		if !known {
			continue
		}
		seen[n] = true
		v.ArticleID = it.ID
		out = append(out, v)
	}
	return out
}

func parseKeepElem(raw json.RawMessage) (n int, v KeepVerdict, ok bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return 0, KeepVerdict{}, false
	}
	var obj struct {
		N          json.RawMessage `json:"n"`
		Keep       json.RawMessage `json:"keep"`
		Confidence json.RawMessage `json:"confidence"`
		Reason     string          `json:"reason"`
		Topics     json.RawMessage `json:"topics"`
		Folder     json.RawMessage `json:"folder"`
		FolderName json.RawMessage `json:"folderName"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		soft := stripTrailingCommas(string(raw))
		if err2 := json.Unmarshal([]byte(soft), &obj); err2 != nil {
			return 0, KeepVerdict{}, false
		}
	}
	n, ok = parseKeepN(obj.N)
	if !ok {
		return 0, KeepVerdict{}, false
	}
	keep := parseKeepBool(obj.Keep)
	reason := strings.TrimSpace(obj.Reason)
	if keep && reason == "" {
		reason = "worth reading"
	}
	folder := parseKeepString(obj.Folder)
	if folder == "" {
		folder = parseKeepString(obj.FolderName)
	}
	return n, KeepVerdict{
		Keep:       keep,
		Confidence: parseKeepConfidence(obj.Confidence),
		Reason:     reason,
		Topics:     parseKeepTopics(obj.Topics),
		FolderName: folder,
	}, true
}

func parseKeepString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func parseKeepN(raw json.RawMessage) (int, bool) {
	for _, n := range parseNField(raw) {
		if n > 0 {
			return n, true
		}
	}
	return 0, false
}

func parseKeepBool(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f != 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "yes", "keep", "1":
			return true
		}
	}
	return false
}

func parseKeepConfidence(raw json.RawMessage) float64 {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return clamp01(f)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err == nil {
			return clamp01(f)
		}
	}
	return 0
}

func clamp01(f float64) float64 {
	if f != f || f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func parseKeepTopics(raw json.RawMessage) []string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return []string{s}
	}
	if raw[0] != '[' {
		return nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil
	}
	var out []string
	for _, el := range elems {
		el = bytes.TrimSpace(el)
		if len(el) == 0 || string(el) == "null" {
			continue
		}
		var s string
		if err := json.Unmarshal(el, &s); err != nil {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// JudgeKeepBatch asks the model which items belong in the kept collection.
func (s *Service) JudgeKeepBatch(ctx context.Context, items []KeepItem, profile, strictness, locale string, folders []KeepFolderRef) ([]KeepVerdict, error) {
	if len(items) == 0 {
		return nil, nil
	}
	cfg, err := s.loadCfg(ctx)
	if err != nil {
		return nil, err
	}
	strictness = NormalizeKeepStrictness(strictness)
	req := ChatRequest{
		Messages: []Message{
			{Role: "system", Content: SystemPromptFor(FeatureKeep, locale)},
			{Role: "user", Content: UserPromptKeep(items, profile, strictness, locale, folders)},
		},
	}
	// Tests inject NewChatter (Chat only). Production streams with a long
	// timeout — same reason as briefing: non-stream waits for the whole JSON
	// and often dies on "timeout awaiting response headers".
	var res ChatResponse
	if s.NewChatter != nil {
		chat, err := s.NewChatter(cfg)
		if err != nil {
			return nil, err
		}
		res, err = chat.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
	} else {
		client, err := NewClientWithTimeout(cfg, 4*time.Minute)
		if err != nil {
			return nil, err
		}
		res, err = client.ChatStream(ctx, req, nil)
		if err != nil {
			return nil, err
		}
	}
	if err := RejectIfIncomplete(res.FinishReason); err != nil {
		return nil, err
	}

	byIndex := make(map[int]KeepItem, len(items))
	for i, it := range items {
		byIndex[keepItemNumber(it, i)] = it
	}
	parsed := ParseKeepBatch(res.Content, byIndex)
	thr := KeepConfidenceThreshold(strictness)
	out := applyKeepThreshold(items, parsed, thr)
	applyKeepFolders(out, folders)
	return out, nil
}

// applyKeepFolders resolves FolderName against existing folders after thresholding.
// !Keep or empty/unknown names become root (empty FolderID/FolderName).
func applyKeepFolders(verdicts []KeepVerdict, folders []KeepFolderRef) {
	for i := range verdicts {
		if !verdicts[i].Keep || len(folders) == 0 {
			verdicts[i].FolderID = ""
			verdicts[i].FolderName = ""
			continue
		}
		id, name := MatchKeepFolder(verdicts[i].FolderName, folders)
		verdicts[i].FolderID = id
		verdicts[i].FolderName = name
	}
}

func applyKeepThreshold(items []KeepItem, parsed []KeepVerdict, thr float64) []KeepVerdict {
	used := make([]bool, len(parsed))
	out := make([]KeepVerdict, len(items))
	for i, it := range items {
		idx := -1
		for j, v := range parsed {
			if used[j] {
				continue
			}
			if v.ArticleID == it.ID {
				idx = j
				break
			}
		}
		if idx < 0 {
			out[i] = KeepVerdict{ArticleID: it.ID}
			continue
		}
		used[idx] = true
		v := parsed[idx]
		if v.Keep && v.Confidence < thr {
			v.Keep = false
			v.ThresholdRejected = true
		}
		v.ArticleID = it.ID
		out[i] = v
	}
	return out
}
