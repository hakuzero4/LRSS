package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"lrss/internal/htmltext"
)

// Feature identifiers for cache keys and prompts.
const (
	FeatureSummarize = "summarize"
	FeatureTranslate = "translate"
	FeatureAsk       = "ask"
	FeatureDigest    = "digest"
	FeatureSuggest   = "suggest"
	FeatureClassify  = "classify"
)

// Default budget ≈ 6k tokens × 4 chars (heuristic).
const DefaultMaxInputChars = 24000

// ArticleInput is the article payload for feature prompts.
type ArticleInput struct {
	ID      string
	Title   string
	Summary string
	Body    string // plain text preferred; HTML is stripped if needed
	URL     string
	Author  string
}

// DigestItem is one article line for daily digest.
type DigestItem struct {
	Title   string
	Summary string
	URL     string
}

// BudgetText truncates s to maxChars on a rune boundary, appending an ellipsis notice.
func BudgetText(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if maxChars <= 0 {
		maxChars = DefaultMaxInputChars
	}
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	runes := []rune(s)
	if maxChars < 4 {
		return string(runes[:maxChars])
	}
	cut := maxChars - 1
	return string(runes[:cut]) + "…"
}

// PlainBody prefers contentText; falls back to stripping HTML.
func PlainBody(contentText, contentHTML string) string {
	t := strings.TrimSpace(contentText)
	if t != "" {
		return t
	}
	return htmltext.ToText(contentHTML)
}

// BuildArticleBundle joins title/summary/body with a size budget for the model.
func BuildArticleBundle(a ArticleInput, maxChars int) string {
	if maxChars <= 0 {
		maxChars = DefaultMaxInputChars
	}
	var b strings.Builder
	title := strings.TrimSpace(a.Title)
	if title == "" {
		title = "(untitled)"
	}
	b.WriteString("Title: ")
	b.WriteString(title)
	b.WriteByte('\n')
	if a.Author != "" {
		b.WriteString("Author: ")
		b.WriteString(strings.TrimSpace(a.Author))
		b.WriteByte('\n')
	}
	if a.URL != "" {
		b.WriteString("URL: ")
		b.WriteString(strings.TrimSpace(a.URL))
		b.WriteByte('\n')
	}
	if s := strings.TrimSpace(a.Summary); s != "" {
		b.WriteString("Summary: ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	body := strings.TrimSpace(a.Body)
	if body == "" {
		body = strings.TrimSpace(a.Summary)
	}
	header := b.String()
	remain := maxChars - utf8.RuneCountInString(header) - 8
	if remain < 200 {
		remain = 200
	}
	b.WriteString("Body:\n")
	b.WriteString(BudgetText(body, remain))
	return b.String()
}

// ContentFingerprint hashes the article fields that affect generation.
func ContentFingerprint(a ArticleInput) string {
	payload := strings.Join([]string{
		strings.TrimSpace(a.Title),
		strings.TrimSpace(a.Summary),
		strings.TrimSpace(a.Body),
		strings.TrimSpace(a.URL),
	}, "\n|\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16]) // 32 hex chars
}

// DigestFingerprint hashes the digest item set.
func DigestFingerprint(items []DigestItem, topN int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "n=%d\n", topN)
	for i, it := range items {
		fmt.Fprintf(&b, "%d|%s|%s|%s\n", i, it.Title, it.Summary, it.URL)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}

// CacheKey builds a stable cache key for (article|digest scope) + feature + model + content hash + extra.
func CacheKey(articleID, feature, model, contentHash, extra string) string {
	parts := []string{
		strings.TrimSpace(articleID),
		strings.TrimSpace(feature),
		strings.TrimSpace(model),
		strings.TrimSpace(contentHash),
		strings.TrimSpace(extra),
	}
	raw := strings.Join(parts, "::")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// SystemPromptFor returns the feature system prompt (overrides user default for quality).
func SystemPromptFor(feature string) string {
	switch feature {
	case FeatureSummarize:
		return "You are an RSS reading assistant. Summarize articles clearly in Markdown. Be concise and faithful to the source. Use the same language as the article unless asked otherwise."
	case FeatureTranslate:
		return "You are a precise translator for RSS articles. Preserve meaning, names, and structure. Output Markdown only."
	case FeatureAsk:
		return "You are a careful reading assistant. Answer only from the provided article context. If unknown, say so. Prefer short Markdown answers."
	case FeatureDigest:
		return "You are a news desk editor. Produce a compact daily unread digest in Markdown with short bullets and optional themes. Do not invent articles."
	case FeatureSuggest:
		return "You suggest tags and folder placement for RSS articles. Reply in compact Markdown with clear sections. Prefer short tags."
	case FeatureClassify:
		return "You classify RSS items as organic editorial content vs ads/soft-promo/sponsored. Be conservative: only flag clear promotional content. Reply in Markdown with a clear verdict line."
	default:
		return "You are a helpful RSS reading assistant. Reply in Markdown."
	}
}

// UserPromptSummarize builds the user message for summarize.
func UserPromptSummarize(bundle string) string {
	return "Summarize this article in Markdown:\n1) 2–4 sentence overview\n2) Bullet key points (3–7)\n\n" + bundle
}

// UserPromptTranslate builds the user message for translation.
func UserPromptTranslate(bundle, targetLang string) string {
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = "zh-CN"
	}
	return fmt.Sprintf("Translate the following article into %s. Keep Markdown structure if useful. Output only the translation.\n\n%s", targetLang, bundle)
}

// UserPromptAsk builds the user message for Q&A.
func UserPromptAsk(bundle, question string) string {
	q := strings.TrimSpace(question)
	if q == "" {
		q = "What is this article about? List main claims and any risks or caveats."
	}
	return "Article context:\n" + bundle + "\n\nQuestion: " + q + "\n\nAnswer in Markdown."
}

// UserPromptDigest builds the digest user message.
func UserPromptDigest(items []DigestItem) string {
	var b strings.Builder
	b.WriteString("Create a daily unread digest from these articles (Top N). Group lightly by theme if possible.\n\n")
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s\n", i+1, strings.TrimSpace(it.Title))
		if s := strings.TrimSpace(it.Summary); s != "" {
			b.WriteString("   ")
			b.WriteString(BudgetText(s, 280))
			b.WriteByte('\n')
		}
		if u := strings.TrimSpace(it.URL); u != "" {
			b.WriteString("   ")
			b.WriteString(u)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\nOutput Markdown with a title, short intro, and bullets.")
	return b.String()
}

// UserPromptSuggest includes available folder names.
func UserPromptSuggest(bundle string, folderNames []string) string {
	var b strings.Builder
	b.WriteString("Suggest organization for this article.\n")
	b.WriteString("Output Markdown with:\n## Tags\n- tag1\n- tag2\n## Folder\nBest match folder name from the list (or Unfiled), with one-line reason.\n\n")
	if len(folderNames) > 0 {
		b.WriteString("Available folders: ")
		b.WriteString(strings.Join(folderNames, ", "))
		b.WriteString("\n\n")
	} else {
		b.WriteString("No folders exist yet; still suggest tags and say Unfiled for folder.\n\n")
	}
	b.WriteString(bundle)
	return b.String()
}

// UserPromptClassify builds classify prompt.
func UserPromptClassify(bundle string) string {
	return "Classify whether this item is primarily advertising / soft promo / sponsored content.\n" +
		"Output Markdown:\n**Verdict:** organic | promo | unclear\n**Confidence:** low|medium|high\n**Why:** 1–3 short bullets\n\n" +
		bundle
}

var tagWordRe = regexp.MustCompile(`(?i)\b([A-Za-z][A-Za-z0-9+#.]{1,24}|[\p{Han}]{2,8})\b`)

// LocalSuggestTags derives simple tags from title/summary without calling the LLM.
// Complementary first pass for smart suggestions.
func LocalSuggestTags(title, summary string, max int) []string {
	if max <= 0 {
		max = 6
	}
	text := title + " " + summary
	seen := map[string]struct{}{}
	var out []string
	// Prefer explicit hashtags.
	for _, m := range regexp.MustCompile(`#([\p{L}\p{N}_-]{2,24})`).FindAllStringSubmatch(text, -1) {
		t := strings.ToLower(m[1])
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) >= max {
			return out
		}
	}
	// Keyword stop-ish list
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "from": {}, "this": {}, "that": {},
		"are": {}, "was": {}, "were": {}, "have": {}, "has": {}, "will": {}, "your": {},
		"about": {}, "into": {}, "http": {}, "https": {}, "www": {}, "com": {},
		"一个": {}, "我们": {}, "他们": {}, "以及": {}, "如果": {}, "因为": {}, "但是": {},
	}
	for _, m := range tagWordRe.FindAllString(text, -1) {
		t := strings.ToLower(strings.TrimSpace(m))
		if len(t) < 2 {
			continue
		}
		if _, bad := stop[t]; bad {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) >= max {
			break
		}
	}
	return out
}

// FormatLocalSuggestMarkdown turns local tags into Markdown for immediate UI.
func FormatLocalSuggestMarkdown(tags []string, folderHint string) string {
	var b strings.Builder
	b.WriteString("## Tags\n")
	if len(tags) == 0 {
		b.WriteString("_No local tags extracted._\n")
	} else {
		for _, t := range tags {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n## Folder\n")
	if strings.TrimSpace(folderHint) == "" {
		folderHint = "Unfiled"
	}
	b.WriteString(folderHint)
	b.WriteString("\n\n_Local rules only — enable LLM for richer suggestions._\n")
	return b.String()
}

// MatchFolderName finds the best case-insensitive folder id by name contained in text.
func MatchFolderName(text string, folders []FolderRef) (id, name string) {
	low := strings.ToLower(text)
	// Prefer longer names to avoid partial false positives.
	bestLen := 0
	for _, f := range folders {
		n := strings.TrimSpace(f.Name)
		if n == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(n)) && len(n) > bestLen {
			bestLen = len(n)
			id, name = f.ID, n
		}
	}
	return id, name
}
