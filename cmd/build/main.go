package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	core "folio/internal/folio"
)

const pageSize = 10

func main() {
	outDir := flag.String("out", "dist", "output directory")
	basePath := flag.String("base-path", "", "base path prefix, e.g. /repo")
	configPath := flag.String("config", "config.json", "config file path")
	siteURL := flag.String("site-url", "", "absolute site url, e.g. https://example.com")
	flag.Parse()

	cfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if strings.TrimSpace(*siteURL) != "" {
		cfg.SiteURL = strings.TrimRight(strings.TrimSpace(*siteURL), "/")
	}

	posts, err := core.LoadPosts("posts", cfg.AuthorName)
	if err != nil {
		log.Fatal(err)
	}

	tagStats, tagSlugs, tagURLs := core.BuildStaticTagStats(posts, *basePath)

	if err := os.RemoveAll(*outDir); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, ".nojekyll"), []byte{}, 0644); err != nil {
		log.Fatal(err)
	}
	if err := copyDirIfExists(filepath.Join("themes", "default", "static"), filepath.Join(*outDir, "static")); err != nil {
		log.Fatal(err)
	}
	if err := copyDirIfExists(filepath.Join("themes", core.NormalizeThemeName(cfg.Theme), "static"), filepath.Join(*outDir, "static")); err != nil {
		log.Fatal(err)
	}

	assets, err := fingerprintAssets(filepath.Join(*outDir, "static"), []string{"style.css", "favicon.png"})
	if err != nil {
		log.Fatal(err)
	}

	base := core.NormalizeBasePath(*basePath)
	stylePath := core.WithBase(*basePath, "/static/style.css")
	if name := assets["style.css"]; name != "" {
		stylePath = core.WithBase(*basePath, "/static/"+name)
	}
	faviconPath := core.WithBase(*basePath, "/static/favicon.png")
	if name := assets["favicon.png"]; name != "" {
		faviconPath = core.WithBase(*basePath, "/static/"+name)
	}

	_, totalPages, _ := core.PaginatePosts(posts, 1, pageSize)
	for page := 1; page <= totalPages; page++ {
		pagePosts, _, currentPage := core.PaginatePosts(posts, page, pageSize)
		pageTitle := cfg.SiteTitle
		pagePathForSEO := core.WithBase(*basePath, "/")
		outPath := filepath.Join(*outDir, "index.html")
		if currentPage > 1 {
			pageTitle = fmt.Sprintf("%s - 第 %d 页", cfg.SiteTitle, currentPage)
			pagePathForSEO = core.WithBase(*basePath, fmt.Sprintf("/page/%d/", currentPage))
			outPath = filepath.Join(*outDir, "page", fmt.Sprintf("%d", currentPage), "index.html")
		}

		if err := renderToFile(outPath, "index.html", *basePath, tagURLs, cfg.Theme, core.IndexPageData{
			Title:           pageTitle,
			BasePath:        base,
			AuthorGitHub:    cfg.AuthorGitHub,
			StylePath:       stylePath,
			FaviconPath:     faviconPath,
			SiteDescription: cfg.SiteDescription,
			SEO:             core.MakeSEO(cfg, pageTitle, cfg.SiteDescription, pagePathForSEO, "website", ""),
			Posts:           pagePosts,
			Pagination:      buildStaticPagination(*basePath, currentPage, totalPages),
		}); err != nil {
			log.Fatal(err)
		}
	}

	_, totalArchivePages, _ := core.PaginatePosts(posts, 1, pageSize)
	for page := 1; page <= totalArchivePages; page++ {
		pagePosts, _, currentPage := core.PaginatePosts(posts, page, pageSize)
		pageTitle := "归档"
		if currentPage > 1 {
			pageTitle = fmt.Sprintf("归档 - 第 %d 页", currentPage)
		}

		if err := renderToFile(staticArchivesOutPath(*outDir, currentPage), "archives.html", *basePath, tagURLs, cfg.Theme, core.ArchivesPageData{
			Title:        pageTitle,
			BasePath:     base,
			AuthorGitHub: cfg.AuthorGitHub,
			StylePath:    stylePath,
			FaviconPath:  faviconPath,
			SEO:          core.MakeSEO(cfg, pageTitle+" - "+cfg.SiteTitle, "按月份浏览历史文章。", staticArchivesPageURL(*basePath, currentPage), "website", ""),
			Groups:       core.BuildArchiveGroups(pagePosts),
			Pagination:   buildStaticArchivesPagination(*basePath, currentPage, totalArchivePages),
		}); err != nil {
			log.Fatal(err)
		}
	}

	if err := renderToFile(filepath.Join(*outDir, "search", "index.html"), "search.html", *basePath, tagURLs, cfg.Theme, core.SearchPageData{
		Title:        "搜索",
		BasePath:     base,
		AuthorGitHub: cfg.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          core.MakeSEO(cfg, "搜索 - "+cfg.SiteTitle, "在博客中搜索标题、标签和正文。", core.WithBase(*basePath, "/search"), "website", ""),
	}); err != nil {
		log.Fatal(err)
	}

	if err := writeJSON(filepath.Join(*outDir, "search-index.json"), core.MakeSearchDocs(posts)); err != nil {
		log.Fatal(err)
	}

	_, totalTagPages, _ := core.PaginatePosts(posts, 1, pageSize)
	for page := 1; page <= totalTagPages; page++ {
		pagePosts, _, currentPage := core.PaginatePosts(posts, page, pageSize)
		pageTitle := "标签"
		if currentPage > 1 {
			pageTitle = fmt.Sprintf("标签 - 第 %d 页", currentPage)
		}

		if err := renderToFile(staticTagsOutPath(*outDir, "", currentPage), "tags.html", *basePath, tagURLs, cfg.Theme, core.TagsPageData{
			Title:        pageTitle,
			BasePath:     base,
			AuthorGitHub: cfg.AuthorGitHub,
			StylePath:    stylePath,
			FaviconPath:  faviconPath,
			SEO:          core.MakeSEO(cfg, pageTitle+" - "+cfg.SiteTitle, "按标签浏览文章内容。", staticTagsPageURL(*basePath, "", currentPage), "website", ""),
			CurrentTag:   "",
			Tags:         tagStats,
			Posts:        pagePosts,
			Pagination:   buildStaticTagsPagination(*basePath, "", currentPage, totalTagPages),
		}); err != nil {
			log.Fatal(err)
		}
	}

	for _, stat := range tagStats {
		filtered := core.FilterPostsByTag(posts, stat.Name)
		slug := tagSlugs[stat.Name]
		_, totalTagPagesForCurrent, _ := core.PaginatePosts(filtered, 1, pageSize)

		for page := 1; page <= totalTagPagesForCurrent; page++ {
			pagePosts, _, currentPage := core.PaginatePosts(filtered, page, pageSize)
			pageTitle := "标签: " + stat.Name
			if currentPage > 1 {
				pageTitle = fmt.Sprintf("标签: %s - 第 %d 页", stat.Name, currentPage)
			}

			if err := renderToFile(staticTagsOutPath(*outDir, slug, currentPage), "tags.html", *basePath, tagURLs, cfg.Theme, core.TagsPageData{
				Title:        pageTitle,
				BasePath:     base,
				AuthorGitHub: cfg.AuthorGitHub,
				StylePath:    stylePath,
				FaviconPath:  faviconPath,
				SEO:          core.MakeSEO(cfg, pageTitle+" - "+cfg.SiteTitle, "按标签浏览文章内容。", staticTagsPageURL(*basePath, slug, currentPage), "website", ""),
				CurrentTag:   stat.Name,
				Tags:         tagStats,
				Posts:        pagePosts,
				Pagination:   buildStaticTagsPagination(*basePath, slug, currentPage, totalTagPagesForCurrent),
			}); err != nil {
				log.Fatal(err)
			}
		}
	}
	for _, post := range posts {
		if err := renderToFile(filepath.Join(*outDir, "post", post.Slug, "index.html"), "post.html", *basePath, tagURLs, cfg.Theme, core.PostPageData{
			Title:        post.Title,
			BasePath:     base,
			AuthorGitHub: cfg.AuthorGitHub,
			StylePath:    stylePath,
			FaviconPath:  faviconPath,
			SEO:          core.MakeSEO(cfg, post.Title+" - "+cfg.SiteTitle, core.Excerpt(post.Markdown, 140), core.WithBase(*basePath, "/post/"+post.Slug+"/"), "article", post.Date.Format(time.RFC3339)),
			Post:         post,
			Comments:     core.BuildCommentConfig(cfg, post),
		}); err != nil {
			log.Fatal(err)
		}
	}

	if err := renderToFile(filepath.Join(*outDir, "404.html"), "404.html", *basePath, tagURLs, cfg.Theme, core.NotFoundPageData{
		Title:        "页面不存在",
		BasePath:     base,
		AuthorGitHub: cfg.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          core.MakeSEO(cfg, "页面不存在 - "+cfg.SiteTitle, "你访问的页面不存在或已移动。", core.WithBase(*basePath, "/404.html"), "website", ""),
		Message:      "你访问的页面不存在或已移动。",
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("static site generated at %s (posts=%d, tags=%d)", *outDir, len(posts), len(tagStats))
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func renderToFile(dstPath, page, basePath string, tagURLs map[string]string, theme string, data any) error {
	tpl, err := parseTemplate(basePath, tagURLs, theme, page)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tpl.Execute(f, data); err != nil {
		return err
	}
	return nil
}

func parseTemplate(basePath string, tagURLs map[string]string, theme, page string) (*template.Template, error) {
	return core.ParseTemplate(theme, page, func(tag string) string {
		if u, ok := tagURLs[tag]; ok {
			return u
		}
		return core.WithBase(basePath, "/tags/")
	})
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDirIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return copyDir(src, dst)
}

func fingerprintAssets(staticDir string, names []string) (map[string]string, error) {
	out := make(map[string]string, len(names))
	for _, name := range names {
		src := filepath.Join(staticDir, name)
		b, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		sum := sha256.Sum256(b)
		hash := fmt.Sprintf("%x", sum[:])[:8]
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		fingerprinted := fmt.Sprintf("%s.%s%s", base, hash, ext)
		if err := os.WriteFile(filepath.Join(staticDir, fingerprinted), b, 0644); err != nil {
			return nil, err
		}
		out[name] = fingerprinted
	}
	return out, nil
}

func buildStaticPagination(basePath string, currentPage, totalPages int) core.Pagination {
	p := core.Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = staticIndexPageURL(basePath, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = staticIndexPageURL(basePath, currentPage+1)
	}
	links := make([]core.PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, core.PageLink{
			Number:  i,
			URL:     staticIndexPageURL(basePath, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func staticIndexPageURL(basePath string, page int) string {
	if page <= 1 {
		return core.WithBase(basePath, "/")
	}
	return core.WithBase(basePath, fmt.Sprintf("/page/%d/", page))
}

func buildStaticTagsPagination(basePath, slug string, currentPage, totalPages int) core.Pagination {
	p := core.Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = staticTagsPageURL(basePath, slug, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = staticTagsPageURL(basePath, slug, currentPage+1)
	}
	links := make([]core.PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, core.PageLink{
			Number:  i,
			URL:     staticTagsPageURL(basePath, slug, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func buildStaticArchivesPagination(basePath string, currentPage, totalPages int) core.Pagination {
	p := core.Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = staticArchivesPageURL(basePath, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = staticArchivesPageURL(basePath, currentPage+1)
	}
	links := make([]core.PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, core.PageLink{
			Number:  i,
			URL:     staticArchivesPageURL(basePath, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func staticTagsPageURL(basePath, slug string, page int) string {
	if strings.TrimSpace(slug) == "" {
		if page <= 1 {
			return core.WithBase(basePath, "/tags/")
		}
		return core.WithBase(basePath, fmt.Sprintf("/tags/page/%d/", page))
	}
	if page <= 1 {
		return core.WithBase(basePath, "/tags/"+slug+"/")
	}
	return core.WithBase(basePath, fmt.Sprintf("/tags/%s/page/%d/", slug, page))
}

func staticArchivesPageURL(basePath string, page int) string {
	if page <= 1 {
		return core.WithBase(basePath, "/archives/")
	}
	return core.WithBase(basePath, fmt.Sprintf("/archives/page/%d/", page))
}

func staticTagsOutPath(outDir, slug string, page int) string {
	if strings.TrimSpace(slug) == "" {
		if page <= 1 {
			return filepath.Join(outDir, "tags", "index.html")
		}
		return filepath.Join(outDir, "tags", "page", fmt.Sprintf("%d", page), "index.html")
	}
	if page <= 1 {
		return filepath.Join(outDir, "tags", slug, "index.html")
	}
	return filepath.Join(outDir, "tags", slug, "page", fmt.Sprintf("%d", page), "index.html")
}

func staticArchivesOutPath(outDir string, page int) string {
	if page <= 1 {
		return filepath.Join(outDir, "archives", "index.html")
	}
	return filepath.Join(outDir, "archives", "page", fmt.Sprintf("%d", page), "index.html")
}
