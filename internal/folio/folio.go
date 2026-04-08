package folio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Post struct {
	Slug        string
	Title       string
	Author      string
	Date        time.Time
	DateDisplay string
	Tags        []string
	Draft       bool
	Markdown    string
	HTML        template.HTML
}

type SearchDoc struct {
	Title   string   `json:"title"`
	Slug    string   `json:"slug"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
}

type SEO struct {
	Description   string
	CanonicalURL  string
	OGType        string
	OGURL         string
	OGImage       string
	SiteName      string
	PublishedTime string
}

type AppConfig struct {
	SiteTitle          string        `json:"site_title"`
	SiteDescription    string        `json:"site_description"`
	SiteURL            string        `json:"site_url"`
	AuthorName         string        `json:"author_name"`
	AuthorGitHub       string        `json:"author_github"`
	Theme              string        `json:"theme"`
	DefaultDescription string        `json:"default_description"`
	DefaultOGImage     string        `json:"default_og_image"`
	DefaultOGType      string        `json:"default_og_type"`
	CommentsProvider   string        `json:"comments_provider"`
	CommentsRepo       string        `json:"comments_repo"`
	CommentsRepoID     string        `json:"comments_repo_id"`
	CommentsCategory   string        `json:"comments_category"`
	CommentsCategoryID string        `json:"comments_category_id"`
	CommentsMapping    string        `json:"comments_mapping"`
	CommentsTheme      string        `json:"comments_theme"`
	CommentsLang       string        `json:"comments_lang"`
	CommentsLabel      string        `json:"comments_label"`
	CommentsIssueTerm  string        `json:"comments_issue_term"`
	Plugins            PluginConfigs `json:"plugins"`
}

type PluginConfigs []PluginConfig

func (p *PluginConfigs) UnmarshalJSON(data []byte) error {
	var names []string
	if err := json.Unmarshal(data, &names); err == nil {
		out := make([]PluginConfig, 0, len(names))
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			out = append(out, PluginConfig{Name: name})
		}
		*p = out
		return nil
	}

	var configs []PluginConfig
	if err := json.Unmarshal(data, &configs); err == nil {
		*p = configs
		return nil
	}
	return fmt.Errorf("plugins must be an array of names or plugin objects")
}

type TagStat struct {
	Name  string
	Count int
	URL   string
}

type ArchiveGroup struct {
	Label string
	Posts []Post
}

var (
	reImage       = regexp.MustCompile(`!\[([^\]]*?)\]\(([^)\s]+)(?:\s+(?:&quot;|&#34;)(.*?)(?:&quot;|&#34;))?\)`)
	reLink        = regexp.MustCompile(`\[([^\]]+?)\]\(([^)\s]+)(?:\s+(?:&quot;|&#34;)(.*?)(?:&quot;|&#34;))?\)`)
	reBold        = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic      = regexp.MustCompile(`\*(.+?)\*`)
	reStrike      = regexp.MustCompile(`~~(.+?)~~`)
	reCode        = regexp.MustCompile("`([^`]+)`")
	reFootnoteDef = regexp.MustCompile(`^\[\^([^\]]+)\]:\s*(.*)$`)
	reFootnoteRef = regexp.MustCompile(`\[\^([^\]]+)\]`)
)

func DefaultConfig() AppConfig {
	return AppConfig{
		SiteTitle:          "Folio",
		SiteDescription:    "A lightweight blog powered by Go and file storage.",
		SiteURL:            "",
		AuthorName:         "Anonymous",
		AuthorGitHub:       "",
		Theme:              "default",
		DefaultDescription: "A lightweight blog powered by Go and file storage.",
		DefaultOGImage:     "",
		DefaultOGType:      "website",
		CommentsMapping:    "pathname",
		CommentsTheme:      "preferred_color_scheme",
		CommentsLang:       "zh-CN",
		CommentsIssueTerm:  "pathname",
	}
}

func (c *AppConfig) normalize() {
	d := DefaultConfig()
	if strings.TrimSpace(c.SiteTitle) == "" {
		c.SiteTitle = d.SiteTitle
	}
	if strings.TrimSpace(c.SiteDescription) == "" {
		c.SiteDescription = d.SiteDescription
	}
	c.SiteURL = strings.TrimRight(strings.TrimSpace(c.SiteURL), "/")
	if strings.TrimSpace(c.AuthorName) == "" {
		c.AuthorName = d.AuthorName
	}
	c.AuthorGitHub = strings.TrimSpace(c.AuthorGitHub)
	if c.AuthorGitHub != "" &&
		!strings.HasPrefix(c.AuthorGitHub, "http://") &&
		!strings.HasPrefix(c.AuthorGitHub, "https://") {
		c.AuthorGitHub = "https://github.com/" + strings.Trim(c.AuthorGitHub, "/")
	}
	c.Theme = NormalizeThemeName(c.Theme)
	if strings.TrimSpace(c.DefaultDescription) == "" {
		c.DefaultDescription = c.SiteDescription
	}
	if strings.TrimSpace(c.DefaultOGType) == "" {
		c.DefaultOGType = d.DefaultOGType
	}
	c.CommentsProvider = strings.ToLower(strings.TrimSpace(c.CommentsProvider))
	if strings.TrimSpace(c.CommentsMapping) == "" {
		c.CommentsMapping = d.CommentsMapping
	}
	if strings.TrimSpace(c.CommentsTheme) == "" {
		c.CommentsTheme = d.CommentsTheme
	}
	if strings.TrimSpace(c.CommentsLang) == "" {
		c.CommentsLang = d.CommentsLang
	}
	if strings.TrimSpace(c.CommentsIssueTerm) == "" {
		c.CommentsIssueTerm = d.CommentsIssueTerm
	}
}

func LoadConfig(path string) (AppConfig, error) {
	cfg := DefaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return cfg, nil
	}
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	cfg.normalize()
	return cfg, nil
}

func CanonicalURL(siteURL, path string) string {
	site := strings.TrimRight(strings.TrimSpace(siteURL), "/")
	if site == "" {
		return path
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Avoid duplicated base segment, e.g. site=/folio + path=/folio/post/x.
	if u, err := url.Parse(site); err == nil {
		base := strings.TrimRight(u.Path, "/")
		if base != "" {
			if path == base {
				path = "/"
			} else if strings.HasPrefix(path, base+"/") {
				path = strings.TrimPrefix(path, base)
			}
		}
	}
	return site + path
}

func MakeSEO(cfg AppConfig, title, desc, path, ogType, publishedTime string) SEO {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		desc = cfg.DefaultDescription
	}
	ogType = strings.TrimSpace(ogType)
	if ogType == "" {
		ogType = cfg.DefaultOGType
	}
	url := CanonicalURL(cfg.SiteURL, path)
	return SEO{
		Description:   desc,
		CanonicalURL:  url,
		OGType:        ogType,
		OGURL:         url,
		OGImage:       cfg.DefaultOGImage,
		SiteName:      cfg.SiteTitle,
		PublishedTime: publishedTime,
	}
}

func WithBase(basePath, path string) string {
	if basePath == "" {
		return path
	}
	base := "/" + strings.Trim(basePath, "/")
	if strings.HasPrefix(path, "/") {
		return base + path
	}
	return base + "/" + path
}

func NormalizeBasePath(basePath string) string {
	if strings.TrimSpace(basePath) == "" {
		return ""
	}
	return "/" + strings.Trim(basePath, "/")
}

func BuildTagURL(basePath, tag, mode string) string {
	if mode == "static" {
		return WithBase(basePath, "/tags/"+SlugifyTag(tag)+"/")
	}
	return WithBase(basePath, "/tags?tag="+url.QueryEscape(tag))
}

func BuildTagStats(posts []Post, basePath, mode string) []TagStat {
	counts := map[string]int{}
	for _, post := range posts {
		for _, tag := range post.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			counts[tag]++
		}
	}

	stats := make([]TagStat, 0, len(counts))
	for name, count := range counts {
		stats = append(stats, TagStat{Name: name, Count: count, URL: BuildTagURL(basePath, name, mode)})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})
	return stats
}

func BuildStaticTagStats(posts []Post, basePath string) ([]TagStat, map[string]string, map[string]string) {
	counts := map[string]int{}
	for _, post := range posts {
		for _, tag := range post.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			counts[tag]++
		}
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	used := map[string]bool{}
	tagSlugs := map[string]string{}
	tagURLs := map[string]string{}
	stats := make([]TagStat, 0, len(names))

	for _, name := range names {
		baseSlug := SlugifyTag(name)
		slug := baseSlug
		i := 2
		for used[slug] {
			slug = fmt.Sprintf("%s-%d", baseSlug, i)
			i++
		}
		used[slug] = true
		tagSlugs[name] = slug
		tagURLs[name] = WithBase(basePath, "/tags/"+slug+"/")
		stats = append(stats, TagStat{Name: name, Count: counts[name], URL: tagURLs[name]})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})
	return stats, tagSlugs, tagURLs
}

func FilterPostsByTag(posts []Post, target string) []Post {
	target = strings.TrimSpace(target)
	if target == "" {
		return posts
	}
	out := make([]Post, 0)
	for _, post := range posts {
		for _, tag := range post.Tags {
			if strings.EqualFold(strings.TrimSpace(tag), target) {
				out = append(out, post)
				break
			}
		}
	}
	return out
}

func BuildArchiveGroups(posts []Post) []ArchiveGroup {
	keys := make([]string, 0)
	groups := map[string][]Post{}
	for _, post := range posts {
		key := post.Date.Format("2006-01")
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], post)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	out := make([]ArchiveGroup, 0, len(keys))
	for _, key := range keys {
		out = append(out, ArchiveGroup{Label: key, Posts: groups[key]})
	}
	return out
}

func BuildCommentConfig(cfg AppConfig, post Post) CommentConfig {
	provider := strings.ToLower(strings.TrimSpace(cfg.CommentsProvider))
	if provider == "" {
		return CommentConfig{}
	}

	switch provider {
	case "giscus":
		repo := strings.TrimSpace(cfg.CommentsRepo)
		repoID := strings.TrimSpace(cfg.CommentsRepoID)
		category := strings.TrimSpace(cfg.CommentsCategory)
		categoryID := strings.TrimSpace(cfg.CommentsCategoryID)
		if repo == "" || repoID == "" || category == "" || categoryID == "" {
			return CommentConfig{}
		}
		return CommentConfig{
			Enabled:    true,
			Provider:   "giscus",
			Repo:       repo,
			RepoID:     repoID,
			Category:   category,
			CategoryID: categoryID,
			Mapping:    strings.TrimSpace(cfg.CommentsMapping),
			Theme:      strings.TrimSpace(cfg.CommentsTheme),
			Lang:       strings.TrimSpace(cfg.CommentsLang),
		}
	case "utterances":
		repo := strings.TrimSpace(cfg.CommentsRepo)
		if repo == "" {
			return CommentConfig{}
		}
		issueTerm := strings.TrimSpace(cfg.CommentsIssueTerm)
		if issueTerm == "slug" {
			issueTerm = post.Slug
		}
		return CommentConfig{
			Enabled:   true,
			Provider:  "utterances",
			Repo:      repo,
			IssueTerm: issueTerm,
			Label:     strings.TrimSpace(cfg.CommentsLabel),
			Theme:     strings.TrimSpace(cfg.CommentsTheme),
		}
	default:
		return CommentConfig{}
	}
}

func PaginatePosts(posts []Post, page, perPage int) ([]Post, int, int) {
	if perPage <= 0 {
		perPage = 10
	}
	totalPages := 1
	if len(posts) > 0 {
		totalPages = (len(posts) + perPage - 1) / perPage
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	end := start + perPage
	if start >= len(posts) {
		return []Post{}, totalPages, page
	}
	if end > len(posts) {
		end = len(posts)
	}
	return posts[start:end], totalPages, page
}

func MakeSearchDocs(posts []Post) []SearchDoc {
	docs := make([]SearchDoc, 0, len(posts))
	for _, post := range posts {
		docs = append(docs, SearchDoc{
			Title:   post.Title,
			Slug:    post.Slug,
			Date:    post.DateDisplay,
			Tags:    post.Tags,
			Content: NormalizeSearchText(post.Markdown),
		})
	}
	return docs
}

func LoadPosts(dir, fallbackAuthor string) ([]Post, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}

	posts := make([]Post, 0, len(files))
	var loadErr error
	for _, path := range files {
		post, err := LoadPost(path, fallbackAuthor)
		if err != nil {
			loadErr = errors.Join(loadErr, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if post.Draft {
			continue
		}
		posts = append(posts, post)
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return posts, nil
}

func LoadPostBySlug(dir, slug, fallbackAuthor string) (Post, error) {
	path := filepath.Join(dir, slug+".md")
	if _, err := os.Stat(path); err != nil {
		return Post{}, err
	}
	return LoadPost(path, fallbackAuthor)
}

func LoadPost(path, fallbackAuthor string) (Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Post{}, err
	}

	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fm, body := splitFrontMatter(string(b))

	post := Post{
		Slug:     slug,
		Title:    fallbackTitle(fm["title"], slug),
		Author:   fallbackAuthorName(fm["author"], fallbackAuthor),
		Tags:     parseList(fm["tags"]),
		Draft:    strings.EqualFold(strings.TrimSpace(fm["draft"]), "true"),
		Markdown: strings.TrimSpace(body),
	}

	post.Date = parseDateOrNow(fm["date"])
	post.DateDisplay = post.Date.Format("2006-01-02")
	post.HTML = template.HTML(renderMarkdown(post.Markdown))
	return post, nil
}

func Excerpt(s string, n int) string {
	text := NormalizeSearchText(s)
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n]) + "..."
}

func NormalizeSearchText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	replacer := strings.NewReplacer(
		"#", " ", "*", " ", "`", " ", ">", " ", "[", " ", "]", " ",
		"(", " ", ")", " ", "-", " ", "_", " ", "\n", " ", "\t", " ",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func SlugifyTag(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "tag"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "tag"
	}
	return out
}

func NormalizeThemeName(theme string) string {
	theme = strings.ToLower(strings.TrimSpace(theme))
	if theme == "" {
		return "default"
	}
	for _, r := range theme {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return "default"
		}
	}
	return theme
}

func ResolveTemplatePath(theme, rel string) string {
	rel = strings.TrimLeft(strings.ReplaceAll(rel, "\\", "/"), "/")
	t := NormalizeThemeName(theme)
	candidates := []string{
		filepath.Join("themes", t, "templates", filepath.FromSlash(rel)),
		filepath.Join("themes", "default", "templates", filepath.FromSlash(rel)),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func ResolveStaticPath(theme, rel string) string {
	rel = strings.TrimLeft(strings.ReplaceAll(rel, "\\", "/"), "/")
	t := NormalizeThemeName(theme)
	candidates := []string{
		filepath.Join("themes", t, "static", filepath.FromSlash(rel)),
		filepath.Join("themes", "default", "static", filepath.FromSlash(rel)),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func splitFrontMatter(content string) (map[string]string, string) {
	content = strings.TrimPrefix(content, "\uFEFF")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimPrefix(lines[0], "\uFEFF")) != "---" {
		return map[string]string{}, content
	}

	meta := map[string]string{}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
		k, v, ok := strings.Cut(lines[i], ":")
		if !ok {
			continue
		}
		meta[strings.TrimSpace(strings.ToLower(k))] = strings.Trim(strings.TrimSpace(v), "\"")
	}
	if end == -1 {
		return map[string]string{}, content
	}
	return meta, strings.Join(lines[end+1:], "\n")
}

func parseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "\"")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseDateOrNow(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now()
	}

	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Now()
}

func fallbackTitle(title, slug string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}
	parts := strings.Split(strings.ReplaceAll(slug, "-", " "), " ")
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func fallbackAuthorName(author, fallback string) string {
	author = strings.TrimSpace(author)
	if author != "" {
		return author
	}
	return strings.TrimSpace(fallback)
}

type headingItem struct {
	Level int
	ID    string
	Text  string
}

type footnoteState struct {
	Defs  map[string]string
	Order []string
	Used  map[string]int
}

type markdownTable struct {
	Header []string
	Rows   [][]string
}

func renderMarkdown(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	input = strings.ReplaceAll(input, "\r\n", "\n")
	lines := strings.Split(input, "\n")
	bodyLines, footnoteDefs := extractFootnoteDefs(lines)
	headings := collectHeadings(bodyLines)
	headingIdx := 0
	footnotes := &footnoteState{
		Defs: footnoteDefs,
		Used: map[string]int{},
	}

	var out strings.Builder
	inCode := false
	var paragraph []string
	type listFrame struct {
		tag    string
		liOpen bool
	}
	listStack := make([]listFrame, 0)

	closeListItem := func() {
		if len(listStack) == 0 {
			return
		}
		top := &listStack[len(listStack)-1]
		if top.liOpen {
			out.WriteString("</li>\n")
			top.liOpen = false
		}
	}
	closeList := func() {
		closeListItem()
		for len(listStack) > 0 {
			top := listStack[len(listStack)-1]
			listStack = listStack[:len(listStack)-1]
			out.WriteString("</" + top.tag + ">\n")
			closeListItem()
		}
	}
	adjustList := func(depth int, tag string) {
		if depth < 0 {
			depth = 0
		}
		if len(listStack) == 0 {
			out.WriteString("<" + tag + ">\n")
			listStack = append(listStack, listFrame{tag: tag})
		}

		currentDepth := len(listStack) - 1
		if depth > currentDepth {
			for currentDepth < depth {
				parent := &listStack[len(listStack)-1]
				if !parent.liOpen {
					out.WriteString("<li>")
					parent.liOpen = true
				}
				out.WriteString("\n<" + tag + ">\n")
				listStack = append(listStack, listFrame{tag: tag})
				currentDepth++
			}
		} else if depth < currentDepth {
			closeListItem()
			for currentDepth > depth {
				top := listStack[len(listStack)-1]
				listStack = listStack[:len(listStack)-1]
				out.WriteString("</" + top.tag + ">\n")
				currentDepth--
				closeListItem()
			}
		}

		if len(listStack) == 0 {
			out.WriteString("<" + tag + ">\n")
			listStack = append(listStack, listFrame{tag: tag})
			return
		}
		top := &listStack[len(listStack)-1]
		if top.tag != tag {
			closeListItem()
			old := listStack[len(listStack)-1]
			listStack = listStack[:len(listStack)-1]
			out.WriteString("</" + old.tag + ">\n")
			out.WriteString("<" + tag + ">\n")
			listStack = append(listStack, listFrame{tag: tag})
		} else {
			closeListItem()
		}
	}
	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		text := formatInlineWithState(strings.Join(paragraph, " "), footnotes)
		out.WriteString("<p>" + text + "</p>\n")
		paragraph = paragraph[:0]
	}

	for i := 0; i < len(bodyLines); i++ {
		line := bodyLines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			closeList()
			if inCode {
				out.WriteString("</code></pre>\n")
			} else {
				out.WriteString("<pre><code>")
			}
			inCode = !inCode
			continue
		}

		if inCode {
			out.WriteString(template.HTMLEscapeString(line))
			out.WriteString("\n")
			continue
		}

		if trimmed == "" {
			flushParagraph()
			closeList()
			continue
		}

		if strings.EqualFold(trimmed, "[TOC]") {
			flushParagraph()
			closeList()
			out.WriteString(renderTOC(headings))
			continue
		}

		if isHorizontalRule(trimmed) {
			flushParagraph()
			closeList()
			out.WriteString("<hr>\n")
			continue
		}

		if tbl, consumed := parseTable(bodyLines, i); consumed > 0 {
			flushParagraph()
			closeList()
			out.WriteString(renderTable(tbl, footnotes))
			i += consumed - 1
			continue
		}

		if level, text, ok := parseHeading(trimmed); ok {
			flushParagraph()
			closeList()
			id := slugifyHeading(text)
			if headingIdx < len(headings) {
				id = headings[headingIdx].ID
			}
			headingIdx++
			fmt.Fprintf(&out, `<h%d id="%s">%s</h%d>`+"\n", level, id, formatInlineWithState(text, footnotes), level)
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			flushParagraph()
			closeList()
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			out.WriteString("<blockquote><p>" + formatInlineWithState(text, footnotes) + "</p></blockquote>\n")
			continue
		}

		if item, tag, indent, task, ok := parseListItem(line); ok {
			flushParagraph()
			adjustList(indent, tag)
			content := formatInlineWithState(item, footnotes)
			if task != nil {
				checked := ""
				if *task {
					checked = " checked"
				}
				content = `<input type="checkbox" disabled` + checked + `> ` + content
			}
			top := &listStack[len(listStack)-1]
			out.WriteString("<li>" + content)
			top.liOpen = true
			continue
		}

		closeList()
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	closeList()
	if inCode {
		out.WriteString("</code></pre>\n")
	}
	if len(footnotes.Order) > 0 {
		out.WriteString(`<section class="footnotes">` + "\n")
		out.WriteString(`<hr>` + "\n")
		out.WriteString(`<ol>` + "\n")
		for _, id := range footnotes.Order {
			aid := slugifyHeading(id)
			out.WriteString(`<li id="fn-` + aid + `">` + formatInline(footnotes.Defs[id]) + ` <a href="#fnref-` + aid + `">back</a></li>` + "\n")
		}
		out.WriteString(`</ol>` + "\n")
		out.WriteString(`</section>` + "\n")
	}
	return out.String()
}

func parseListItem(line string) (item, tag string, indent int, task *bool, ok bool) {
	indent = leadingSpaces(line) / 2
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		item = strings.TrimSpace(trimmed[2:])
		tag = "ul"
		ok = true
	} else {
		i := 0
		for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
			i++
		}
		if i > 0 && i+1 < len(trimmed) && trimmed[i] == '.' && trimmed[i+1] == ' ' {
			item = strings.TrimSpace(trimmed[i+2:])
			tag = "ol"
			ok = true
		}
	}
	if !ok {
		return "", "", 0, nil, false
	}

	lower := strings.ToLower(item)
	if strings.HasPrefix(lower, "[ ] ") || strings.HasPrefix(lower, "[x] ") {
		checked := strings.HasPrefix(lower, "[x] ")
		task = &checked
		item = strings.TrimSpace(item[4:])
	}
	return item, tag, indent, task, true
}

func formatInline(s string) string {
	return formatInlineWithState(s, nil)
}

func formatInlineWithState(s string, footnotes *footnoteState) string {
	out := template.HTMLEscapeString(s)
	codeSpans := make([]string, 0)
	out = reCode.ReplaceAllStringFunc(out, func(m string) string {
		sub := reCode.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		token := fmt.Sprintf("{{CODE_SPAN_%d}}", len(codeSpans))
		codeSpans = append(codeSpans, `<code>`+sub[1]+`</code>`)
		return token
	})
	out = reImage.ReplaceAllStringFunc(out, func(m string) string {
		sub := reImage.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		alt := sub[1]
		src := sub[2]
		title := ""
		if len(sub) > 3 {
			title = sub[3]
		}
		tag := `<img src="` + src + `" alt="` + alt + `" loading="lazy"`
		if strings.TrimSpace(title) != "" {
			tag += ` title="` + title + `"`
		}
		tag += `>`
		return tag
	})
	out = reLink.ReplaceAllStringFunc(out, func(m string) string {
		sub := reLink.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		text := sub[1]
		href := sub[2]
		title := ""
		if len(sub) > 3 {
			title = sub[3]
		}
		attr := ` href="` + href + `"`
		if strings.TrimSpace(title) != "" {
			attr += ` title="` + title + `"`
		}
		rawHref := strings.TrimSpace(html.UnescapeString(href))
		if u, err := url.Parse(rawHref); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			attr += ` target="_blank" rel="noopener noreferrer"`
		}
		return `<a` + attr + `>` + text + `</a>`
	})
	if footnotes != nil {
		out = reFootnoteRef.ReplaceAllStringFunc(out, func(m string) string {
			sub := reFootnoteRef.FindStringSubmatch(m)
			if len(sub) < 2 {
				return m
			}
			id := sub[1]
			if _, ok := footnotes.Defs[id]; !ok {
				return m
			}
			if n, ok := footnotes.Used[id]; ok {
				aid := slugifyHeading(id)
				return fmt.Sprintf(`<sup id="fnref-%s"><a href="#fn-%s">[%d]</a></sup>`, aid, aid, n)
			}
			n := len(footnotes.Order) + 1
			footnotes.Order = append(footnotes.Order, id)
			footnotes.Used[id] = n
			aid := slugifyHeading(id)
			return fmt.Sprintf(`<sup id="fnref-%s"><a href="#fn-%s">[%d]</a></sup>`, aid, aid, n)
		})
	}
	out = reBold.ReplaceAllString(out, `<strong>$1</strong>`)
	out = reItalic.ReplaceAllString(out, `<em>$1</em>`)
	out = reStrike.ReplaceAllString(out, `<del>$1</del>`)
	for i, code := range codeSpans {
		token := fmt.Sprintf("{{CODE_SPAN_%d}}", i)
		out = strings.ReplaceAll(out, token, code)
	}
	return out
}

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func parseHeading(trimmed string) (level int, text string, ok bool) {
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	level = 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	text = strings.TrimSpace(trimmed[level:])
	if text == "" {
		return 0, "", false
	}
	return level, text, true
}

func isHorizontalRule(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	ch := trimmed[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	count := 0
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c == ' ' {
			continue
		}
		if c != ch {
			return false
		}
		count++
	}
	return count >= 3
}

func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func isTableSeparatorRow(line string) bool {
	cells := splitTableRow(line)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		s := strings.TrimSpace(c)
		s = strings.TrimPrefix(s, ":")
		s = strings.TrimSuffix(s, ":")
		if len(s) < 3 {
			return false
		}
		for i := 0; i < len(s); i++ {
			if s[i] != '-' {
				return false
			}
		}
	}
	return true
}

func parseTable(lines []string, start int) (markdownTable, int) {
	if start+1 >= len(lines) {
		return markdownTable{}, 0
	}
	headLine := strings.TrimSpace(lines[start])
	sepLine := strings.TrimSpace(lines[start+1])
	if !strings.Contains(headLine, "|") || !isTableSeparatorRow(sepLine) {
		return markdownTable{}, 0
	}
	tbl := markdownTable{Header: splitTableRow(headLine)}
	i := start + 2
	for i < len(lines) {
		l := strings.TrimSpace(lines[i])
		if l == "" || !strings.Contains(l, "|") {
			break
		}
		tbl.Rows = append(tbl.Rows, splitTableRow(l))
		i++
	}
	return tbl, i - start
}

func renderTable(tbl markdownTable, footnotes *footnoteState) string {
	var b strings.Builder
	b.WriteString("<table>\n<thead><tr>")
	for _, h := range tbl.Header {
		b.WriteString("<th>" + formatInlineWithState(h, footnotes) + "</th>")
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, row := range tbl.Rows {
		b.WriteString("<tr>")
		for _, cell := range row {
			b.WriteString("<td>" + formatInlineWithState(cell, footnotes) + "</td>")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>\n")
	return b.String()
}

func extractFootnoteDefs(lines []string) ([]string, map[string]string) {
	body := make([]string, 0, len(lines))
	defs := map[string]string{}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := reFootnoteDef.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) < 3 {
			body = append(body, line)
			continue
		}
		id := strings.TrimSpace(m[1])
		text := strings.TrimSpace(m[2])
		j := i + 1
		for j < len(lines) {
			next := lines[j]
			if strings.HasPrefix(next, "  ") || strings.HasPrefix(next, "\t") {
				text += " " + strings.TrimSpace(next)
				j++
				continue
			}
			break
		}
		defs[id] = text
		i = j - 1
	}
	return body, defs
}

func collectHeadings(lines []string) []headingItem {
	out := make([]headingItem, 0)
	used := map[string]int{}
	inCode := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		level, text, ok := parseHeading(trimmed)
		if !ok {
			continue
		}
		id := uniqueHeadingID(slugifyHeading(text), used)
		out = append(out, headingItem{Level: level, ID: id, Text: text})
	}
	return out
}

func uniqueHeadingID(base string, used map[string]int) string {
	if base == "" {
		base = "section"
	}
	n := used[base]
	used[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, n+1)
}

func slugifyHeading(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "section"
	}
	return out
}

func renderTOC(headings []headingItem) string {
	if len(headings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="toc" style="font-variant-numeric: tabular-nums;">` + "\n")
	b.WriteString(`<ol>` + "\n")

	prevLevel := 1
	first := true
	for _, h := range headings {
		lvl := h.Level
		if lvl < 1 {
			lvl = 1
		}
		if first {
			prevLevel = lvl
			first = false
		} else {
			if lvl > prevLevel {
				for i := prevLevel; i < lvl; i++ {
					b.WriteString("<ol>\n")
				}
			} else {
				b.WriteString("</li>\n")
				if lvl < prevLevel {
					for i := lvl; i < prevLevel; i++ {
						b.WriteString("</ol>\n")
						if i+1 <= prevLevel-1 {
							b.WriteString("</li>\n")
						}
					}
				}
			}
		}
		b.WriteString(`<li><a href="#` + h.ID + `">` + formatInline(h.Text) + `</a>`)
		prevLevel = lvl
	}
	b.WriteString("</li>\n")
	for prevLevel > 1 {
		b.WriteString("</ol>\n</li>\n")
		prevLevel--
	}
	b.WriteString(`</ol>` + "\n")
	b.WriteString(`</nav>` + "\n")
	return b.String()
}
