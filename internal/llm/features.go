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

// NormalizeUILocale maps app UI locales to a short code used in prompts/cache.
// zh* → "zh", everything else → "en".
func NormalizeUILocale(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	if l == "" {
		return "en"
	}
	if strings.HasPrefix(l, "zh") {
		return "zh"
	}
	return "en"
}

// OutputLanguageInstruction tells the model which language to write in.
func OutputLanguageInstruction(locale string) string {
	switch NormalizeUILocale(locale) {
	case "zh":
		return "Write the entire reply in Simplified Chinese (简体中文). Do not use English except for proper nouns, code, or URLs."
	default:
		return "Write the entire reply in English."
	}
}

// SystemPromptFor returns the feature system prompt for the UI language.
func SystemPromptFor(feature, locale string) string {
	lang := OutputLanguageInstruction(locale)
	var base string
	switch feature {
	case FeatureSummarize:
		base = "You are an RSS reading assistant. Write concise deck/standfirst summaries for the reader UI (plain text + optional • bullets). Be faithful and avoid markdown headings or bold."
	case FeatureTranslate:
		base = "You are a precise literary translator for RSS articles. Preserve meaning, names, and tone. Output only bilingual segment pairs in the required marker format—no commentary."
	case FeatureAsk:
		base = "You are a careful reading assistant. Answer only from the provided article context. If unknown, say so. Prefer short Markdown answers."
	case FeatureDigest:
		base = "You are a news desk editor. Produce a compact daily unread digest in Markdown with short bullets and optional themes. Do not invent articles."
	case FeatureSuggest:
		base = "You suggest tags and folder placement for RSS articles. Reply in compact Markdown with clear sections. Prefer short tags."
	case FeatureClassify:
		base = "You classify RSS items as organic editorial content vs ads/soft-promo/sponsored. Be conservative: only flag clear promotional content. Reply in Markdown with a clear verdict line."
	default:
		base = "You are a helpful RSS reading assistant. Reply in Markdown."
	}
	// Translate already targets an explicit language; still keep a short style line.
	if feature == FeatureTranslate {
		return base
	}
	return base + " " + lang
}

// UserPromptSummarize builds a deck-style summary for the reader (above the body).
// Plain prose + optional • bullets — no markdown headings/bold (shown as standfirst).
func UserPromptSummarize(bundle, locale string) string {
	if NormalizeUILocale(locale) == "zh" {
		return "为 RSS 阅读器写一段「文首摘要」（替换文章原摘要，显示在正文上方）。\n" +
			"要求：\n" +
			"- 先写 2–4 句连贯概述（纯文本，不要标题、不要 **加粗**）\n" +
			"- 空一行后，用 3–5 行要点，每行以「• 」开头\n" +
			"- 忠实原文，简洁好读，不要「本文介绍了」套话\n" +
			"- 不要输出「概述」「要点列表」等小标题\n\n" +
			OutputLanguageInstruction(locale) + "\n\n" + bundle
	}
	return "Write a standfirst / deck summary for an RSS reader (replaces the feed summary above the body).\n" +
		"Requirements:\n" +
		"- 2–4 continuous sentences of plain prose (no headings, no **bold**)\n" +
		"- Then a blank line and 3–5 bullet lines each starting with \"• \"\n" +
		"- Faithful and concise; no meta phrases like \"This article discusses\"\n" +
		"- Do not label sections as Overview / Key points\n\n" +
		OutputLanguageInstruction(locale) + "\n\n" + bundle
}

// UserPromptTranslate builds a bilingual interlinear translation prompt.
// Output uses <<o>> / <<t>> markers so the reader can render source + target pairs
// (like quote cards: original line, then translation under it).
func UserPromptTranslate(bundle, targetLang string) string {
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = "zh-CN"
	}
	label := targetLang
	switch NormalizeUILocale(targetLang) {
	case "zh":
		label = "Simplified Chinese (简体中文)"
	default:
		if strings.HasPrefix(strings.ToLower(targetLang), "en") {
			label = "English"
		}
	}
	return fmt.Sprintf(`Translate the article into %s for an elegant bilingual reading view.

Output format ONLY (repeat for each segment; no intro/outro):
<<o>> original sentence or short paragraph (source language, keep as-is)
<<t>> translation into %s

Rules:
- Split into natural reading segments (usually one sentence, or a short paragraph).
- Keep every meaningful sentence; do not skip content.
- Do not wrap in markdown code fences; do not use **bold** or headings.
- Proper names and code may stay in the original language inside <<t>> when natural.

Article:
%s
`, label, label, bundle)
}

// BilingualPair is one original + translation unit for the reader UI.
type BilingualPair struct {
	Original    string `json:"original"`
	Translation string `json:"translation"`
}

// ParseBilingualPairs extracts <<o>>/<<t>> pairs from model output.
// Also accepts a loose fallback: alternating non-empty lines when markers are missing.
// Uses Go RE2-safe patterns (no lookaheads).
func ParseBilingualPairs(raw string) []BilingualPair {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Split on <<o>> (case-insensitive via lowercase scan of tags).
	// Normalize tags first.
	norm := regexp.MustCompile(`(?i)<<\s*o\s*>>`).ReplaceAllString(raw, "\n<<O>>\n")
	norm = regexp.MustCompile(`(?i)<<\s*t\s*>>`).ReplaceAllString(norm, "\n<<T>>\n")

	parts := strings.Split(norm, "<<O>>")
	var out []BilingualPair
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Each part after <<O>> should be: original <<T>> translation
		idx := strings.Index(part, "<<T>>")
		if idx < 0 {
			continue
		}
		o := strings.TrimSpace(part[:idx])
		tr := strings.TrimSpace(part[idx+len("<<T>>"):])
		// Drop accidental next-tag leftovers.
		if cut := strings.Index(tr, "<<O>>"); cut >= 0 {
			tr = strings.TrimSpace(tr[:cut])
		}
		if o == "" && tr == "" {
			continue
		}
		out = append(out, BilingualPair{Original: o, Translation: tr})
	}
	if len(out) > 0 {
		return out
	}

	// Fallback: split on blank lines into blocks of 2 lines.
	blocks := regexp.MustCompile(`\n\s*\n+`).Split(raw, -1)
	for _, b := range blocks {
		lines := nonEmptyLines(b)
		if len(lines) >= 2 {
			out = append(out, BilingualPair{Original: lines[0], Translation: lines[1]})
			continue
		}
		if len(lines) == 1 && len(out) > 0 && out[len(out)-1].Translation == "" {
			out[len(out)-1].Translation = lines[0]
		}
	}
	if len(out) > 0 {
		return out
	}

	// Last resort: whole text as one translation-only block.
	return []BilingualPair{{Original: "", Translation: raw}}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip accidental bullets / quotes.
		line = strings.TrimPrefix(line, "> ")
		line = strings.TrimPrefix(line, "- ")
		out = append(out, line)
	}
	return out
}

// TranslatedBodyFromPairs builds HTML + plain text that fully replaces the article body
// with the target-language translation (one <p> per segment).
func TranslatedBodyFromPairs(pairs []BilingualPair) (htmlBody, plain string) {
	if len(pairs) == 0 {
		return "", ""
	}
	var html, text strings.Builder
	for _, p := range pairs {
		seg := strings.TrimSpace(p.Translation)
		if seg == "" {
			seg = strings.TrimSpace(p.Original)
		}
		if seg == "" {
			continue
		}
		// Multi-line segment → multiple paragraphs.
		for _, line := range strings.Split(seg, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			html.WriteString("<p>")
			html.WriteString(htmlEscapeText(line))
			html.WriteString("</p>\n")
			if text.Len() > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(line)
		}
	}
	return strings.TrimSpace(html.String()), strings.TrimSpace(text.String())
}

func htmlEscapeText(s string) string {
	r := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

// UserPromptAsk builds the user message for Q&A.
func UserPromptAsk(bundle, question, locale string) string {
	q := strings.TrimSpace(question)
	if q == "" {
		if NormalizeUILocale(locale) == "zh" {
			q = "这篇文章在说什么？列出主要观点与任何风险或注意事项。"
		} else {
			q = "What is this article about? List main claims and any risks or caveats."
		}
	}
	if NormalizeUILocale(locale) == "zh" {
		return "文章内容：\n" + bundle + "\n\n问题：" + q + "\n\n请用 Markdown 回答。\n" + OutputLanguageInstruction(locale)
	}
	return "Article context:\n" + bundle + "\n\nQuestion: " + q + "\n\nAnswer in Markdown.\n" + OutputLanguageInstruction(locale)
}

// UserPromptDigest builds the digest user message.
func UserPromptDigest(items []DigestItem, locale string) string {
	var b strings.Builder
	if NormalizeUILocale(locale) == "zh" {
		b.WriteString("根据以下今日未读文章（Top N）写一份每日简报。可按主题轻量分组。\n\n")
	} else {
		b.WriteString("Create a daily unread digest from these articles (Top N). Group lightly by theme if possible.\n\n")
	}
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
	if NormalizeUILocale(locale) == "zh" {
		b.WriteString("\n输出 Markdown：标题、简短导语、要点列表。\n")
	} else {
		b.WriteString("\nOutput Markdown with a title, short intro, and bullets.\n")
	}
	b.WriteString(OutputLanguageInstruction(locale))
	return b.String()
}

// UserPromptSuggest includes available folder names.
func UserPromptSuggest(bundle string, folderNames []string, locale string) string {
	var b strings.Builder
	if NormalizeUILocale(locale) == "zh" {
		b.WriteString("为这篇文章建议组织方式。\n")
		b.WriteString("用 Markdown 输出：\n## 标签\n- 标签1\n- 标签2\n## 文件夹\n从列表中选最合适的文件夹名（或 Unfiled），并给一句理由。\n\n")
	} else {
		b.WriteString("Suggest organization for this article.\n")
		b.WriteString("Output Markdown with:\n## Tags\n- tag1\n- tag2\n## Folder\nBest match folder name from the list (or Unfiled), with one-line reason.\n\n")
	}
	if len(folderNames) > 0 {
		b.WriteString("Available folders: ")
		b.WriteString(strings.Join(folderNames, ", "))
		b.WriteString("\n\n")
	} else if NormalizeUILocale(locale) == "zh" {
		b.WriteString("尚无文件夹；仍请建议标签，文件夹写 Unfiled。\n\n")
	} else {
		b.WriteString("No folders exist yet; still suggest tags and say Unfiled for folder.\n\n")
	}
	b.WriteString(OutputLanguageInstruction(locale))
	b.WriteString("\n\n")
	b.WriteString(bundle)
	return b.String()
}

// UserPromptClassify builds classify prompt.
func UserPromptClassify(bundle, locale string) string {
	if NormalizeUILocale(locale) == "zh" {
		return "判断这篇文章是否主要为广告 / 软文 / 赞助内容。\n" +
			"用 Markdown 输出：\n**Verdict:** organic | promo | unclear\n**Confidence:** low|medium|high\n**Why:** 1–3 条简短理由\n" +
			"（Verdict 行请保留英文枚举值；说明可用中文。）\n\n" +
			OutputLanguageInstruction(locale) + "\n\n" + bundle
	}
	return "Classify whether this item is primarily advertising / soft promo / sponsored content.\n" +
		"Output Markdown:\n**Verdict:** organic | promo | unclear\n**Confidence:** low|medium|high\n**Why:** 1–3 short bullets\n\n" +
		OutputLanguageInstruction(locale) + "\n\n" + bundle
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
