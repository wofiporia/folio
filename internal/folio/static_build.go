package folio

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const staticPageSize = 10

type BuildOptions struct {
	OutDir     string
	BasePath   string
	ConfigPath string
	SiteURL    string
	PostsDir   string
}

func BuildStaticSite(opts BuildOptions) error {
	if strings.TrimSpace(opts.OutDir) == "" {
		opts.OutDir = "dist"
	}
	if strings.TrimSpace(opts.ConfigPath) == "" {
		opts.ConfigPath = "config.json"
	}
	if strings.TrimSpace(opts.PostsDir) == "" {
		opts.PostsDir = "posts"
	}

	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.SiteURL) != "" {
		cfg.SiteURL = strings.TrimRight(strings.TrimSpace(opts.SiteURL), "/")
	}

	posts, err := LoadPosts(opts.PostsDir, cfg.AuthorName)
	if err != nil {
		return err
	}
	pluginManager := NewPluginManager(filepath.Dir(opts.ConfigPath), cfg)
	posts, err = pluginManager.RunAfterPostsLoaded(context.Background(), posts, PluginBuildContext{
		Mode:     "static",
		OutDir:   opts.OutDir,
		BasePath: opts.BasePath,
	})
	if err != nil {
		return err
	}

	tagStats, tagSlugs, tagURLs := BuildStaticTagStats(posts, opts.BasePath)

	if err := os.RemoveAll(opts.OutDir); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, ".nojekyll"), []byte{}, 0o644); err != nil {
		return err
	}
	if err := copyDirIfExists(filepath.Join("themes", "default", "static"), filepath.Join(opts.OutDir, "static")); err != nil {
		return err
	}
	if err := copyDirIfExists(filepath.Join("themes", NormalizeThemeName(cfg.Theme), "static"), filepath.Join(opts.OutDir, "static")); err != nil {
		return err
	}
	if err := copyEnabledPluginStatics(pluginManager, filepath.Dir(opts.ConfigPath), filepath.Join(opts.OutDir, "static", "plugins")); err != nil {
		return err
	}

	assets, err := fingerprintAssets(filepath.Join(opts.OutDir, "static"), []string{"style.css", "favicon.png"})
	if err != nil {
		return err
	}

	base := NormalizeBasePath(opts.BasePath)
	stylePath := WithBase(opts.BasePath, "/static/style.css")
	if name := assets["style.css"]; name != "" {
		stylePath = WithBase(opts.BasePath, "/static/"+name)
	}
	faviconPath := WithBase(opts.BasePath, "/static/favicon.png")
	if name := assets["favicon.png"]; name != "" {
		faviconPath = WithBase(opts.BasePath, "/static/"+name)
	}
	pluginHead := pluginManager.HeadSnippet(opts.BasePath)

	_, totalPages, _ := PaginatePosts(posts, 1, staticPageSize)
	for page := 1; page <= totalPages; page++ {
		pagePosts, _, currentPage := PaginatePosts(posts, page, staticPageSize)
		pageTitle := cfg.SiteTitle
		pagePathForSEO := WithBase(opts.BasePath, "/")
		outPath := filepath.Join(opts.OutDir, "index.html")
		if currentPage > 1 {
			pageTitle = fmt.Sprintf("%s - 第 %d 页", cfg.SiteTitle, currentPage)
			pagePathForSEO = WithBase(opts.BasePath, fmt.Sprintf("/page/%d/", currentPage))
			outPath = filepath.Join(opts.OutDir, "page", fmt.Sprintf("%d", currentPage), "index.html")
		}

		if err := renderStaticFile(outPath, "index.html", opts.BasePath, tagURLs, cfg.Theme, IndexPageData{
			Title:           pageTitle,
			BasePath:        base,
			AuthorGitHub:    cfg.AuthorGitHub,
			StylePath:       stylePath,
			FaviconPath:     faviconPath,
			SiteDescription: cfg.SiteDescription,
			SEO:             MakeSEO(cfg, pageTitle, cfg.SiteDescription, pagePathForSEO, "website", ""),
			PluginHead:      pluginHead,
			Posts:           pagePosts,
			Pagination:      BuildStaticPagination(opts.BasePath, currentPage, totalPages),
		}); err != nil {
			return err
		}
	}

	_, totalArchivePages, _ := PaginatePosts(posts, 1, staticPageSize)
	for page := 1; page <= totalArchivePages; page++ {
		pagePosts, _, currentPage := PaginatePosts(posts, page, staticPageSize)
		pageTitle := "归档"
		if currentPage > 1 {
			pageTitle = fmt.Sprintf("归档 - 第 %d 页", currentPage)
		}

		if err := renderStaticFile(staticArchivesOutPath(opts.OutDir, currentPage), "archives.html", opts.BasePath, tagURLs, cfg.Theme, ArchivesPageData{
			Title:        pageTitle,
			BasePath:     base,
			AuthorGitHub: cfg.AuthorGitHub,
			StylePath:    stylePath,
			FaviconPath:  faviconPath,
			SEO:          MakeSEO(cfg, pageTitle+" - "+cfg.SiteTitle, "按月份浏览历史文章。", StaticArchivesPageURL(opts.BasePath, currentPage), "website", ""),
			PluginHead:   pluginHead,
			Groups:       BuildArchiveGroups(pagePosts),
			Pagination:   BuildStaticArchivesPagination(opts.BasePath, currentPage, totalArchivePages),
		}); err != nil {
			return err
		}
	}

	if err := renderStaticFile(filepath.Join(opts.OutDir, "search", "index.html"), "search.html", opts.BasePath, tagURLs, cfg.Theme, SearchPageData{
		Title:        "搜索",
		BasePath:     base,
		AuthorGitHub: cfg.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          MakeSEO(cfg, "搜索 - "+cfg.SiteTitle, "在博客中搜索标题、标签和正文。", WithBase(opts.BasePath, "/search"), "website", ""),
		PluginHead:   pluginHead,
	}); err != nil {
		return err
	}

	if err := writeBuildJSON(filepath.Join(opts.OutDir, "search-index.json"), MakeSearchDocs(posts)); err != nil {
		return err
	}

	_, totalTagPages, _ := PaginatePosts(posts, 1, staticPageSize)
	for page := 1; page <= totalTagPages; page++ {
		pagePosts, _, currentPage := PaginatePosts(posts, page, staticPageSize)
		pageTitle := "标签"
		if currentPage > 1 {
			pageTitle = fmt.Sprintf("标签 - 第 %d 页", currentPage)
		}

		if err := renderStaticFile(staticTagsOutPath(opts.OutDir, "", currentPage), "tags.html", opts.BasePath, tagURLs, cfg.Theme, TagsPageData{
			Title:        pageTitle,
			BasePath:     base,
			AuthorGitHub: cfg.AuthorGitHub,
			StylePath:    stylePath,
			FaviconPath:  faviconPath,
			SEO:          MakeSEO(cfg, pageTitle+" - "+cfg.SiteTitle, "按标签浏览文章内容。", StaticTagsPageURL(opts.BasePath, "", currentPage), "website", ""),
			PluginHead:   pluginHead,
			CurrentTag:   "",
			Tags:         tagStats,
			Posts:        pagePosts,
			Pagination:   BuildStaticTagsPagination(opts.BasePath, "", currentPage, totalTagPages),
		}); err != nil {
			return err
		}
	}

	for _, stat := range tagStats {
		filtered := FilterPostsByTag(posts, stat.Name)
		slug := tagSlugs[stat.Name]
		_, totalTagPagesForCurrent, _ := PaginatePosts(filtered, 1, staticPageSize)

		for page := 1; page <= totalTagPagesForCurrent; page++ {
			pagePosts, _, currentPage := PaginatePosts(filtered, page, staticPageSize)
			pageTitle := "标签: " + stat.Name
			if currentPage > 1 {
				pageTitle = fmt.Sprintf("标签: %s - 第 %d 页", stat.Name, currentPage)
			}

			if err := renderStaticFile(staticTagsOutPath(opts.OutDir, slug, currentPage), "tags.html", opts.BasePath, tagURLs, cfg.Theme, TagsPageData{
				Title:        pageTitle,
				BasePath:     base,
				AuthorGitHub: cfg.AuthorGitHub,
				StylePath:    stylePath,
				FaviconPath:  faviconPath,
				SEO:          MakeSEO(cfg, pageTitle+" - "+cfg.SiteTitle, "按标签浏览文章内容。", StaticTagsPageURL(opts.BasePath, slug, currentPage), "website", ""),
				PluginHead:   pluginHead,
				CurrentTag:   stat.Name,
				Tags:         tagStats,
				Posts:        pagePosts,
				Pagination:   BuildStaticTagsPagination(opts.BasePath, slug, currentPage, totalTagPagesForCurrent),
			}); err != nil {
				return err
			}
		}
	}

	for _, post := range posts {
		if err := renderStaticFile(filepath.Join(opts.OutDir, "post", post.Slug, "index.html"), "post.html", opts.BasePath, tagURLs, cfg.Theme, PostPageData{
			Title:        post.Title,
			BasePath:     base,
			AuthorGitHub: cfg.AuthorGitHub,
			StylePath:    stylePath,
			FaviconPath:  faviconPath,
			SEO:          MakeSEO(cfg, post.Title+" - "+cfg.SiteTitle, Excerpt(post.Markdown, 140), WithBase(opts.BasePath, "/post/"+post.Slug+"/"), "article", post.Date.Format(time.RFC3339)),
			PluginHead:   pluginHead,
			Post:         post,
			Comments:     BuildCommentConfig(cfg, post),
		}); err != nil {
			return err
		}
	}

	if err := renderStaticFile(filepath.Join(opts.OutDir, "404.html"), "404.html", opts.BasePath, tagURLs, cfg.Theme, NotFoundPageData{
		Title:        "页面不存在",
		BasePath:     base,
		AuthorGitHub: cfg.AuthorGitHub,
		StylePath:    stylePath,
		FaviconPath:  faviconPath,
		SEO:          MakeSEO(cfg, "页面不存在 - "+cfg.SiteTitle, "你访问的页面不存在或已移动。", WithBase(opts.BasePath, "/404.html"), "website", ""),
		PluginHead:   pluginHead,
		Message:      "你访问的页面不存在或已移动。",
	}); err != nil {
		return err
	}
	if err := pluginManager.RunAfterStaticBuild(context.Background(), posts, PluginBuildContext{
		Mode:     "static",
		OutDir:   opts.OutDir,
		BasePath: opts.BasePath,
	}); err != nil {
		return err
	}

	return nil
}

func writeBuildJSON(path string, v any) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func renderStaticFile(dstPath, page, basePath string, tagURLs map[string]string, theme string, data any) (err error) {
	tpl, err := parseBuildTemplate(basePath, tagURLs, theme, page)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	return tpl.Execute(f, data)
}

func parseBuildTemplate(basePath string, tagURLs map[string]string, theme, page string) (*template.Template, error) {
	return ParseTemplate(theme, page, func(tag string) string {
		if u, ok := tagURLs[tag]; ok {
			return u
		}
		return WithBase(basePath, "/tags/")
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
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, in.Close())
	}()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
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

func copyEnabledPluginStatics(m *PluginManager, configDir, dstRoot string) error {
	if m == nil {
		return nil
	}
	for _, name := range m.EnabledPluginNames() {
		src := filepath.Join(configDir, "plugins", name, "static")
		dst := filepath.Join(dstRoot, name)
		if err := copyDirIfExists(src, dst); err != nil {
			return err
		}
	}
	return nil
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
		if err := os.WriteFile(filepath.Join(staticDir, fingerprinted), b, 0o644); err != nil {
			return nil, err
		}
		out[name] = fingerprinted
	}
	return out, nil
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
