package main

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	core "folio/internal/folio"
)

const (
	postDir  = "posts"
	cacheTTL = 5 * time.Second
)

var postsCache = struct {
	mu        sync.RWMutex
	posts     []core.Post
	expiresAt time.Time
}{}

var appConfig = core.DefaultConfig()

func main() {
	cfg, err := core.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("load config error: %v", err)
	}
	appConfig = cfg

	mux := http.NewServeMux()
	mux.HandleFunc("/static/", staticHandler)
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/post/", postHandler)
	mux.HandleFunc("/tags", tagsHandler)
	mux.HandleFunc("/archives", archivesHandler)
	mux.HandleFunc("/search", searchHandler)
	mux.HandleFunc("/search-index.json", searchIndexHandler)

	addr := ":8080"
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			addr = port
		} else {
			addr = ":" + port
		}
	}
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
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
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

	tpl, err := parseTemplate("", "dynamic", "index.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse index template error: %v", err)
		return
	}

	stylePath, faviconPath := currentAssetPaths("")
	data := core.IndexPageData{
		Title:           appConfig.SiteTitle,
		BasePath:        "",
		StylePath:       stylePath,
		FaviconPath:     faviconPath,
		SiteDescription: appConfig.SiteDescription,
		SEO:             core.MakeSEO(appConfig, appConfig.SiteTitle, appConfig.SiteDescription, "/", "website", ""),
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

	post, err := core.LoadPostBySlug(postDir, slug, appConfig.AuthorName)
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

	tpl, err := parseTemplate("", "dynamic", "post.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse post template error: %v", err)
		return
	}

	stylePath, faviconPath := currentAssetPaths("")
	data := core.PostPageData{
		Title:       post.Title,
		BasePath:    "",
		StylePath:   stylePath,
		FaviconPath: faviconPath,
		SEO: core.MakeSEO(
			appConfig,
			post.Title+" - "+appConfig.SiteTitle,
			core.Excerpt(post.Markdown, 140),
			"/post/"+post.Slug,
			"article",
			post.Date.Format(time.RFC3339),
		),
		Post: post,
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

	tagStats := core.BuildTagStats(posts, "", "dynamic")
	filtered := posts
	if currentTag != "" {
		filtered = core.FilterPostsByTag(posts, currentTag)
	}

	tpl, err := parseTemplate("", "dynamic", "tags.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse tags template error: %v", err)
		return
	}

	title := "标签"
	if currentTag != "" {
		title = "标签: " + currentTag
	}
	stylePath, faviconPath := currentAssetPaths("")
	data := core.TagsPageData{
		Title:       title,
		BasePath:    "",
		StylePath:   stylePath,
		FaviconPath: faviconPath,
		SEO:         core.MakeSEO(appConfig, title+" - "+appConfig.SiteTitle, "按标签浏览文章内容。", "/tags", "website", ""),
		CurrentTag:  currentTag,
		Tags:        tagStats,
		Posts:       filtered,
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

	tpl, err := parseTemplate("", "dynamic", "archives.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse archives template error: %v", err)
		return
	}

	stylePath, faviconPath := currentAssetPaths("")
	data := core.ArchivesPageData{
		Title:       "归档",
		BasePath:    "",
		StylePath:   stylePath,
		FaviconPath: faviconPath,
		SEO:         core.MakeSEO(appConfig, "归档 - "+appConfig.SiteTitle, "按月份浏览历史文章。", "/archives", "website", ""),
		Groups:      core.BuildArchiveGroups(posts),
	}
	renderHTML(w, tpl, data, "archives")
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/search" {
		http.NotFound(w, r)
		return
	}

	tpl, err := parseTemplate("", "dynamic", "search.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse search template error: %v", err)
		return
	}

	stylePath, faviconPath := currentAssetPaths("")
	data := core.SearchPageData{
		Title:       "搜索",
		BasePath:    "",
		StylePath:   stylePath,
		FaviconPath: faviconPath,
		SEO:         core.MakeSEO(appConfig, "搜索 - "+appConfig.SiteTitle, "在博客中搜索标题、标签和正文。", "/search", "website", ""),
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
	if err := json.NewEncoder(w).Encode(core.MakeSearchDocs(posts)); err != nil {
		log.Printf("encode search index error: %v", err)
	}
}

func parseTemplate(basePath, tagMode, page string) (*template.Template, error) {
	return core.ParseTemplate(appConfig.Theme, page, func(tag string) string {
		return core.BuildTagURL(basePath, tag, tagMode)
	})
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/static/") {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(r.URL.Path, "/static/")), "/")
	if rel == "" || rel == "." {
		http.NotFound(w, r)
		return
	}
	p := core.ResolveStaticPath(appConfig.Theme, rel)
	if p == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, p)
}

func currentAssetPaths(basePath string) (string, string) {
	styleURL := core.WithBase(basePath, "/static/style.css")
	faviconURL := core.WithBase(basePath, "/static/favicon.png")
	return withAssetVersion("style.css", styleURL), withAssetVersion("favicon.png", faviconURL)
}

func withAssetVersion(rel, publicURL string) string {
	p := core.ResolveStaticPath(appConfig.Theme, rel)
	if p == "" {
		return publicURL
	}
	info, err := os.Stat(p)
	if err != nil {
		return publicURL
	}
	return publicURL + "?v=" + strconv.FormatInt(info.ModTime().Unix(), 10)
}

func loadPosts(dir string) ([]core.Post, error) {
	now := time.Now()
	postsCache.mu.RLock()
	if now.Before(postsCache.expiresAt) && postsCache.posts != nil {
		cached := make([]core.Post, len(postsCache.posts))
		copy(cached, postsCache.posts)
		postsCache.mu.RUnlock()
		return cached, nil
	}
	postsCache.mu.RUnlock()

	posts, err := core.LoadPosts(dir, appConfig.AuthorName)
	if err != nil {
		return nil, err
	}

	postsCache.mu.Lock()
	postsCache.posts = make([]core.Post, len(posts))
	copy(postsCache.posts, posts)
	postsCache.expiresAt = time.Now().Add(cacheTTL)
	postsCache.mu.Unlock()

	return posts, nil
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
