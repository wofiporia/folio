package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
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
	SiteTitle          string `json:"site_title"`
	SiteDescription    string `json:"site_description"`
	SiteURL            string `json:"site_url"`
	AuthorName         string `json:"author_name"`
	DefaultDescription string `json:"default_description"`
	DefaultOGImage     string `json:"default_og_image"`
	DefaultOGType      string `json:"default_og_type"`
}

type IndexData struct {
	Title           string
	BasePath        string
	SiteDescription string
	SEO             SEO
	Posts           []Post
}

type PostData struct {
	Title    string
	BasePath string
	SEO      SEO
	Post     Post
}

type TagStat struct {
	Name  string
	Count int
	URL   string
}

type TagsData struct {
	Title      string
	BasePath   string
	SEO        SEO
	CurrentTag string
	Tags       []TagStat
	Posts      []Post
}

type ArchiveGroup struct {
	Label string
	Posts []Post
}

type ArchivesData struct {
	Title    string
	BasePath string
	SEO      SEO
	Groups   []ArchiveGroup
}

type SearchPageData struct {
	Title    string
	BasePath string
	SEO      SEO
}

const (
	postDir  = "posts"
	cacheTTL = 5 * time.Second
)

var postsCache = struct {
	mu        sync.RWMutex
	posts     []Post
	expiresAt time.Time
}{}

var appConfig = defaultConfig()

var (
	reLink   = regexp.MustCompile(`\[(.+?)\]\(([^)\s]+)\)`)
	reBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic = regexp.MustCompile(`\*(.+?)\*`)
	reCode   = regexp.MustCompile("`([^`]+)`")
)

func defaultConfig() AppConfig {
	return AppConfig{
		SiteTitle:          "Folio",
		SiteDescription:    "一个基于 Go 和文件系统的轻量博客。",
		SiteURL:            "",
		AuthorName:         "Anonymous",
		DefaultDescription: "一个基于 Go 和文件系统的轻量博客。",
		DefaultOGImage:     "",
		DefaultOGType:      "website",
	}
}

func (c *AppConfig) normalize() {
	d := defaultConfig()
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
	if strings.TrimSpace(c.DefaultDescription) == "" {
		c.DefaultDescription = c.SiteDescription
	}
	if strings.TrimSpace(c.DefaultOGType) == "" {
		c.DefaultOGType = d.DefaultOGType
	}
}

func loadConfig(path string) (AppConfig, error) {
	cfg := defaultConfig()
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

func main() {
	cfg, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("load config error: %v", err)
	}
	appConfig = cfg

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/post/", postHandler)
	mux.HandleFunc("/tags", tagsHandler)
	mux.HandleFunc("/archives", archivesHandler)
	mux.HandleFunc("/search", searchHandler)
	mux.HandleFunc("/search-index.json", searchIndexHandler)

	addr := ":8080"
	log.Printf("folio listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func renderHTML(w http.ResponseWriter, tpl *template.Template, data any, page string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.Execute(w, data); err != nil {
		log.Printf("execute %s template error: %v", page, err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	posts, err := loadPosts(postDir)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		log.Printf("loadPosts error: %v", err)
		return
	}

	tpl, err := parseTemplate("", "dynamic", "templates/index.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse index template error: %v", err)
		return
	}

	data := IndexData{
		Title:           appConfig.SiteTitle,
		BasePath:        "",
		SiteDescription: appConfig.SiteDescription,
		SEO:             makeSEO(appConfig.SiteTitle, appConfig.SiteDescription, "/", "website", ""),
		Posts:           posts,
	}
	renderHTML(w, tpl, data, "index")
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/post/") {
		http.NotFound(w, r)
		return
	}

	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/post/"), "/")
	if slug == "" || !isValidSlug(slug) {
		http.NotFound(w, r)
		return
	}

	post, err := loadPostBySlug(postDir, slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to load post", http.StatusInternalServerError)
		log.Printf("loadPostBySlug error: %v", err)
		return
	}
	if post.Draft {
		http.NotFound(w, r)
		return
	}

	tpl, err := parseTemplate("", "dynamic", "templates/post.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse post template error: %v", err)
		return
	}

	data := PostData{
		Title:    post.Title,
		BasePath: "",
		SEO:      makeSEO(post.Title+" - "+appConfig.SiteTitle, excerpt(post.Markdown, 140), "/post/"+post.Slug, "article", post.Date.Format(time.RFC3339)),
		Post:     post,
	}
	renderHTML(w, tpl, data, "post")
}

func tagsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tags" {
		http.NotFound(w, r)
		return
	}

	posts, err := loadPosts(postDir)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		log.Printf("loadPosts error: %v", err)
		return
	}

	currentTag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if currentTag != "" && !isValidTag(currentTag) {
		http.NotFound(w, r)
		return
	}

	tagStats := buildTagStats(posts, "", "dynamic")
	filtered := posts
	if currentTag != "" {
		filtered = filterPostsByTag(posts, currentTag)
	}

	tpl, err := parseTemplate("", "dynamic", "templates/tags.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse tags template error: %v", err)
		return
	}

	title := "标签"
	if currentTag != "" {
		title = "标签: " + currentTag
	}
	data := TagsData{
		Title:      title,
		BasePath:   "",
		SEO:        makeSEO(title+" - "+appConfig.SiteTitle, "按标签浏览文章内容。", "/tags", "website", ""),
		CurrentTag: currentTag,
		Tags:       tagStats,
		Posts:      filtered,
	}
	renderHTML(w, tpl, data, "tags")
}

func archivesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/archives" {
		http.NotFound(w, r)
		return
	}

	posts, err := loadPosts(postDir)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		log.Printf("loadPosts error: %v", err)
		return
	}

	tpl, err := parseTemplate("", "dynamic", "templates/archives.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse archives template error: %v", err)
		return
	}

	data := ArchivesData{
		Title:    "归档",
		BasePath: "",
		SEO:      makeSEO("归档 - "+appConfig.SiteTitle, "按月份浏览历史文章。", "/archives", "website", ""),
		Groups:   buildArchiveGroups(posts),
	}
	renderHTML(w, tpl, data, "archives")
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/search" {
		http.NotFound(w, r)
		return
	}

	tpl, err := parseTemplate("", "dynamic", "templates/search.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse search template error: %v", err)
		return
	}

	data := SearchPageData{
		Title:    "搜索",
		BasePath: "",
		SEO:      makeSEO("搜索 - "+appConfig.SiteTitle, "在博客中搜索标题、标签和正文。", "/search", "website", ""),
	}
	renderHTML(w, tpl, data, "search")
}

func searchIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/search-index.json" {
		http.NotFound(w, r)
		return
	}

	posts, err := loadPosts(postDir)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		log.Printf("loadPosts error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(makeSearchDocs(posts)); err != nil {
		log.Printf("encode search index error: %v", err)
	}
}

func parseTemplate(basePath, tagMode string, files ...string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"tagURL": func(tag string) string {
			return buildTagURL(basePath, tag, tagMode)
		},
	}
	return template.New(filepath.Base(files[0])).Funcs(funcMap).ParseFiles(files...)
}

func withBase(basePath, path string) string {
	if basePath == "" {
		return path
	}
	base := "/" + strings.Trim(basePath, "/")
	if strings.HasPrefix(path, "/") {
		return base + path
	}
	return base + "/" + path
}

func canonicalURL(path string) string {
	if site := strings.TrimRight(strings.TrimSpace(appConfig.SiteURL), "/"); site != "" {
		return site + path
	}
	return path
}

func makeSEO(title, desc, path, ogType, publishedTime string) SEO {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		desc = appConfig.DefaultDescription
	}
	ogType = strings.TrimSpace(ogType)
	if ogType == "" {
		ogType = appConfig.DefaultOGType
	}
	url := canonicalURL(path)
	return SEO{
		Description:   desc,
		CanonicalURL:  url,
		OGType:        ogType,
		OGURL:         url,
		OGImage:       appConfig.DefaultOGImage,
		SiteName:      appConfig.SiteTitle,
		PublishedTime: publishedTime,
	}
}

func buildTagURL(basePath, tag, mode string) string {
	if mode == "static" {
		return withBase(basePath, "/tags/"+slugifyTag(tag)+"/")
	}
	return withBase(basePath, "/tags?tag="+url.QueryEscape(tag))
}

func excerpt(s string, n int) string {
	text := normalizeSearchText(s)
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n]) + "..."
}

func loadPosts(dir string) ([]Post, error) {
	now := time.Now()
	postsCache.mu.RLock()
	if now.Before(postsCache.expiresAt) && postsCache.posts != nil {
		cached := make([]Post, len(postsCache.posts))
		copy(cached, postsCache.posts)
		postsCache.mu.RUnlock()
		return cached, nil
	}
	postsCache.mu.RUnlock()

	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}

	posts := make([]Post, 0, len(files))
	for _, path := range files {
		post, err := loadPost(path)
		if err != nil {
			log.Printf("skip post %s: %v", path, err)
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

	postsCache.mu.Lock()
	postsCache.posts = make([]Post, len(posts))
	copy(postsCache.posts, posts)
	postsCache.expiresAt = time.Now().Add(cacheTTL)
	postsCache.mu.Unlock()

	return posts, nil
}

func loadPostBySlug(dir, slug string) (Post, error) {
	path := filepath.Join(dir, slug+".md")
	if _, err := os.Stat(path); err != nil {
		return Post{}, err
	}
	return loadPost(path)
}

func loadPost(path string) (Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Post{}, err
	}

	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fm, body := splitFrontMatter(string(b))

	post := Post{
		Slug:     slug,
		Title:    fallbackTitle(fm["title"], slug),
		Author:   fallbackAuthor(fm["author"], appConfig.AuthorName),
		Tags:     parseList(fm["tags"]),
		Draft:    strings.EqualFold(strings.TrimSpace(fm["draft"]), "true"),
		Markdown: strings.TrimSpace(body),
	}

	post.Date = parseDateOrNow(fm["date"])
	post.DateDisplay = post.Date.Format("2006-01-02")
	post.HTML = template.HTML(renderMarkdown(post.Markdown))
	return post, nil
}

func buildTagStats(posts []Post, basePath, mode string) []TagStat {
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
		stats = append(stats, TagStat{
			Name:  name,
			Count: count,
			URL:   buildTagURL(basePath, name, mode),
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})
	return stats
}

func filterPostsByTag(posts []Post, target string) []Post {
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

func buildArchiveGroups(posts []Post) []ArchiveGroup {
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

func makeSearchDocs(posts []Post) []SearchDoc {
	docs := make([]SearchDoc, 0, len(posts))
	for _, post := range posts {
		docs = append(docs, SearchDoc{
			Title:   post.Title,
			Slug:    post.Slug,
			Date:    post.DateDisplay,
			Tags:    post.Tags,
			Content: normalizeSearchText(post.Markdown),
		})
	}
	return docs
}

func splitFrontMatter(content string) (map[string]string, string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
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

	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
	}
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

func fallbackAuthor(author, fallback string) string {
	author = strings.TrimSpace(author)
	if author != "" {
		return author
	}
	return strings.TrimSpace(fallback)
}

func renderMarkdown(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(input))
	var out strings.Builder
	inCode := false
	listTag := ""
	var paragraph []string

	closeList := func() {
		if listTag != "" {
			out.WriteString("</" + listTag + ">\n")
			listTag = ""
		}
	}
	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		text := formatInline(strings.Join(paragraph, " "))
		out.WriteString("<p>" + text + "</p>\n")
		paragraph = paragraph[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
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

		if strings.HasPrefix(trimmed, "#") {
			flushParagraph()
			closeList()
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 6 {
				level = 6
			}
			text := strings.TrimSpace(trimmed[level:])
			out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, formatInline(text), level))
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			flushParagraph()
			closeList()
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			out.WriteString("<blockquote><p>" + formatInline(text) + "</p></blockquote>\n")
			continue
		}

		if item, tag, ok := parseListItem(trimmed); ok {
			flushParagraph()
			if listTag != tag {
				closeList()
				listTag = tag
				out.WriteString("<" + listTag + ">\n")
			}
			out.WriteString("<li>" + formatInline(item) + "</li>\n")
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
	return out.String()
}

func parseListItem(line string) (item, tag string, ok bool) {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(line[2:]), "ul", true
	}

	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(line) && line[i] == '.' && line[i+1] == ' ' {
		return strings.TrimSpace(line[i+2:]), "ol", true
	}
	return "", "", false
}

func formatInline(s string) string {
	out := template.HTMLEscapeString(s)
	out = reLink.ReplaceAllString(out, `<a href="$2">$1</a>`)
	out = reBold.ReplaceAllString(out, `<strong>$1</strong>`)
	out = reItalic.ReplaceAllString(out, `<em>$1</em>`)
	out = reCode.ReplaceAllString(out, `<code>$1</code>`)
	return out
}

func normalizeSearchText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	replacer := strings.NewReplacer(
		"#", " ", "*", " ", "`", " ", ">", " ", "[", " ", "]", " ",
		"(", " ", ")", " ", "-", " ", "_", " ", "\n", " ", "\t", " ",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func isValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for _, r := range slug {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if !isAlphaNum && r != '-' {
			return false
		}
	}
	return true
}

func isValidTag(tag string) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" || len(tag) > 64 {
		return false
	}
	for _, r := range tag {
		if r == '<' || r == '>' || r == '"' || r == '\'' {
			return false
		}
	}
	return true
}

func slugifyTag(s string) string {
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
