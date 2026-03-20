package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	core "folio/internal/folio"
)

const (
	postDir  = "posts"
	cacheTTL = 5 * time.Second
	pageSize = 10
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

	mux := newMux()

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

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/static/", staticHandler)
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/post/", postHandler)
	mux.HandleFunc("/tags", tagsHandler)
	mux.HandleFunc("/archives", archivesHandler)
	mux.HandleFunc("/search", searchHandler)
	mux.HandleFunc("/search-index.json", searchIndexHandler)
	return mux
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func renderHTML(w http.ResponseWriter, tpl *template.Template, data any, page string) {
	renderHTMLWithStatus(w, tpl, data, page, http.StatusOK)
}

func renderHTMLWithStatus(w http.ResponseWriter, tpl *template.Template, data any, page string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(status)
	if err := tpl.Execute(w, data); err != nil {
		log.Printf("execute %s template error: %v", page, err)
	}
}

func renderNotFound(w http.ResponseWriter, r *http.Request) {
	tpl, err := parseTemplate("", "dynamic", "404.html")
	if err != nil {
		http.NotFound(w, r)
		log.Printf("parse 404 template error: %v", err)
		return
	}
	stylePath, faviconPath := currentAssetPaths("")
	data := core.NotFoundPageData{
		Title:        "页面不存在",
		BasePath:     "",
		AuthorGitHub: appConfig.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          core.MakeSEO(appConfig, "页面不存在 - "+appConfig.SiteTitle, "你访问的页面不存在或已移动。", r.URL.Path, "website", ""),
		Message:      "你访问的页面不存在或已移动。",
	}
	renderHTMLWithStatus(w, tpl, data, "404", http.StatusNotFound)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		renderNotFound(w, r)
		return
	}

	posts, err := loadPosts(postDir)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		log.Printf("loadPosts error: %v", err)
		return
	}
	currentPage := core.ParsePositiveIntOrDefault(r.URL.Query().Get("page"), 1)
	visiblePosts, totalPages, currentPage := core.PaginatePosts(posts, currentPage, pageSize)

	tpl, err := parseTemplate("", "dynamic", "index.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse index template error: %v", err)
		return
	}

	title := appConfig.SiteTitle
	pathForSEO := "/"
	if currentPage > 1 {
		title = fmt.Sprintf("%s - 第 %d 页", appConfig.SiteTitle, currentPage)
		pathForSEO = core.DynamicIndexPageURL("", currentPage)
	}

	stylePath, faviconPath := currentAssetPaths("")
	data := core.IndexPageData{
		Title:           title,
		BasePath:        "",
		AuthorGitHub:    appConfig.AuthorGitHub,
		StylePath:       stylePath,
		FaviconPath:     faviconPath,
		SiteDescription: appConfig.SiteDescription,
		SEO:             core.MakeSEO(appConfig, title, appConfig.SiteDescription, pathForSEO, "website", ""),
		Posts:           visiblePosts,
		Pagination:      core.BuildDynamicPagination("", currentPage, totalPages),
	}
	renderHTML(w, tpl, data, "index")
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/post/") {
		renderNotFound(w, r)
		return
	}

	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/post/"), "/")
	if slug == "" || !core.IsValidSlug(slug) {
		renderNotFound(w, r)
		return
	}

	post, err := core.LoadPostBySlug(postDir, slug, appConfig.AuthorName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			renderNotFound(w, r)
			return
		}
		http.Error(w, "failed to load post", http.StatusInternalServerError)
		log.Printf("loadPostBySlug error: %v", err)
		return
	}
	if post.Draft {
		renderNotFound(w, r)
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
		Title:        post.Title,
		BasePath:     "",
		AuthorGitHub: appConfig.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO: core.MakeSEO(
			appConfig,
			post.Title+" - "+appConfig.SiteTitle,
			core.Excerpt(post.Markdown, 140),
			"/post/"+post.Slug,
			"article",
			post.Date.Format(time.RFC3339),
		),
		Post:     post,
		Comments: core.BuildCommentConfig(appConfig, post),
	}
	renderHTML(w, tpl, data, "post")
}

func tagsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tags" {
		renderNotFound(w, r)
		return
	}

	posts, err := loadPosts(postDir)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		log.Printf("loadPosts error: %v", err)
		return
	}

	currentTag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if currentTag != "" && !core.IsValidTag(currentTag) {
		renderNotFound(w, r)
		return
	}

	tagStats := core.BuildTagStats(posts, "", "dynamic")
	filtered := posts
	if currentTag != "" {
		filtered = core.FilterPostsByTag(posts, currentTag)
	}
	currentPage := core.ParsePositiveIntOrDefault(r.URL.Query().Get("page"), 1)
	visiblePosts, totalPages, currentPage := core.PaginatePosts(filtered, currentPage, pageSize)

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
	if currentPage > 1 {
		title = fmt.Sprintf("%s - 第 %d 页", title, currentPage)
	}
	pathForSEO := core.DynamicTagsPageURL("", currentTag, currentPage)

	stylePath, faviconPath := currentAssetPaths("")
	data := core.TagsPageData{
		Title:        title,
		BasePath:     "",
		AuthorGitHub: appConfig.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          core.MakeSEO(appConfig, title+" - "+appConfig.SiteTitle, "按标签浏览文章内容。", pathForSEO, "website", ""),
		CurrentTag:   currentTag,
		Tags:         tagStats,
		Posts:        visiblePosts,
		Pagination:   core.BuildDynamicTagsPagination("", currentTag, currentPage, totalPages),
	}
	renderHTML(w, tpl, data, "tags")
}
func archivesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/archives" {
		renderNotFound(w, r)
		return
	}

	posts, err := loadPosts(postDir)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		log.Printf("loadPosts error: %v", err)
		return
	}
	currentPage := core.ParsePositiveIntOrDefault(r.URL.Query().Get("page"), 1)
	visiblePosts, totalPages, currentPage := core.PaginatePosts(posts, currentPage, pageSize)

	tpl, err := parseTemplate("", "dynamic", "archives.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Printf("parse archives template error: %v", err)
		return
	}

	title := "归档"
	if currentPage > 1 {
		title = fmt.Sprintf("归档 - 第 %d 页", currentPage)
	}
	stylePath, faviconPath := currentAssetPaths("")
	data := core.ArchivesPageData{
		Title:        title,
		BasePath:     "",
		AuthorGitHub: appConfig.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          core.MakeSEO(appConfig, title+" - "+appConfig.SiteTitle, "按月份浏览历史文章。", core.DynamicArchivesPageURL("", currentPage), "website", ""),
		Groups:       core.BuildArchiveGroups(visiblePosts),
		Pagination:   core.BuildDynamicArchivesPagination("", currentPage, totalPages),
	}
	renderHTML(w, tpl, data, "archives")
}
func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/search" {
		renderNotFound(w, r)
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
		Title:        "搜索",
		BasePath:     "",
		AuthorGitHub: appConfig.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          core.MakeSEO(appConfig, "搜索 - "+appConfig.SiteTitle, "在博客中搜索标题、标签和正文。", "/search", "website", ""),
	}
	renderHTML(w, tpl, data, "search")
}

func searchIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/search-index.json" {
		renderNotFound(w, r)
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
		renderNotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(r.URL.Path, "/static/")), "/")
	if rel == "" || rel == "." {
		renderNotFound(w, r)
		return
	}
	p := core.ResolveStaticPath(appConfig.Theme, rel)
	if p == "" {
		renderNotFound(w, r)
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
	b, err := os.ReadFile(p)
	if err != nil {
		return publicURL
	}
	sum := sha256.Sum256(b)
	return publicURL + "?v=" + fmt.Sprintf("%x", sum[:])[:8]
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
