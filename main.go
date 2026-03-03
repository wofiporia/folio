package main

import (
	"bufio"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Post struct {
	Slug        string
	Title       string
	Date        time.Time
	DateDisplay string
	Tags        []string
	Draft       bool
	Markdown    string
	HTML        template.HTML
}

type IndexData struct {
	Title    string
	BasePath string
	Posts    []Post
}

type PostData struct {
	Title    string
	BasePath string
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
	Groups   []ArchiveGroup
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

func main() {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/post/", postHandler)
	mux.HandleFunc("/tags", tagsHandler)
	mux.HandleFunc("/archives", archivesHandler)

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

	data := IndexData{Title: "Folio", BasePath: "", Posts: posts}
	if err := tpl.Execute(w, data); err != nil {
		log.Printf("execute index template error: %v", err)
	}
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

	data := PostData{Title: post.Title, BasePath: "", Post: post}
	if err := tpl.Execute(w, data); err != nil {
		log.Printf("execute post template error: %v", err)
	}
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
		CurrentTag: currentTag,
		Tags:       tagStats,
		Posts:      filtered,
	}
	if err := tpl.Execute(w, data); err != nil {
		log.Printf("execute tags template error: %v", err)
	}
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
		Groups:   buildArchiveGroups(posts),
	}
	if err := tpl.Execute(w, data); err != nil {
		log.Printf("execute archives template error: %v", err)
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

func buildTagURL(basePath, tag, mode string) string {
	if mode == "static" {
		return withBase(basePath, "/tags/"+slugifyTag(tag)+"/")
	}
	return withBase(basePath, "/tags?tag="+url.QueryEscape(tag))
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

func renderMarkdown(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(input))
	var out strings.Builder
	inCode := false
	var paragraph []string

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		text := template.HTMLEscapeString(strings.Join(paragraph, " "))
		out.WriteString("<p>" + text + "</p>\n")
		paragraph = paragraph[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
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
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			flushParagraph()
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 6 {
				level = 6
			}
			text := strings.TrimSpace(trimmed[level:])
			out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, template.HTMLEscapeString(text), level))
			continue
		}

		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	if inCode {
		out.WriteString("</code></pre>\n")
	}
	return out.String()
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
