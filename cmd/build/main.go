package main

import (
	"encoding/json"
	"flag"
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
	if err := copyDir("static", filepath.Join(*outDir, "static")); err != nil {
		log.Fatal(err)
	}

	base := core.NormalizeBasePath(*basePath)

	if err := renderToFile(filepath.Join(*outDir, "index.html"), "templates/index.html", *basePath, tagURLs, core.IndexPageData{
		Title:           cfg.SiteTitle,
		BasePath:        base,
		SiteDescription: cfg.SiteDescription,
		SEO:             core.MakeSEO(cfg, cfg.SiteTitle, cfg.SiteDescription, core.WithBase(*basePath, "/"), "website", ""),
		Posts:           posts,
	}); err != nil {
		log.Fatal(err)
	}

	if err := renderToFile(filepath.Join(*outDir, "archives", "index.html"), "templates/archives.html", *basePath, tagURLs, core.ArchivesPageData{
		Title:    "归档",
		BasePath: base,
		SEO:      core.MakeSEO(cfg, "归档 - "+cfg.SiteTitle, "按月份浏览历史文章。", core.WithBase(*basePath, "/archives"), "website", ""),
		Groups:   core.BuildArchiveGroups(posts),
	}); err != nil {
		log.Fatal(err)
	}

	if err := renderToFile(filepath.Join(*outDir, "search", "index.html"), "templates/search.html", *basePath, tagURLs, core.SearchPageData{
		Title:    "搜索",
		BasePath: base,
		SEO:      core.MakeSEO(cfg, "搜索 - "+cfg.SiteTitle, "在博客中搜索标题、标签和正文。", core.WithBase(*basePath, "/search"), "website", ""),
	}); err != nil {
		log.Fatal(err)
	}

	if err := writeJSON(filepath.Join(*outDir, "search-index.json"), core.MakeSearchDocs(posts)); err != nil {
		log.Fatal(err)
	}

	if err := renderToFile(filepath.Join(*outDir, "tags", "index.html"), "templates/tags.html", *basePath, tagURLs, core.TagsPageData{
		Title:      "标签",
		BasePath:   base,
		SEO:        core.MakeSEO(cfg, "标签 - "+cfg.SiteTitle, "按标签浏览文章内容。", core.WithBase(*basePath, "/tags"), "website", ""),
		CurrentTag: "",
		Tags:       tagStats,
		Posts:      posts,
	}); err != nil {
		log.Fatal(err)
	}

	for _, stat := range tagStats {
		filtered := core.FilterPostsByTag(posts, stat.Name)
		slug := tagSlugs[stat.Name]
		if err := renderToFile(filepath.Join(*outDir, "tags", slug, "index.html"), "templates/tags.html", *basePath, tagURLs, core.TagsPageData{
			Title:      "标签: " + stat.Name,
			BasePath:   base,
			SEO:        core.MakeSEO(cfg, "标签: "+stat.Name+" - "+cfg.SiteTitle, "按标签浏览文章内容。", core.WithBase(*basePath, "/tags/"+slug+"/"), "website", ""),
			CurrentTag: stat.Name,
			Tags:       tagStats,
			Posts:      filtered,
		}); err != nil {
			log.Fatal(err)
		}
	}

	for _, post := range posts {
		if err := renderToFile(filepath.Join(*outDir, "post", post.Slug, "index.html"), "templates/post.html", *basePath, tagURLs, core.PostPageData{
			Title:    post.Title,
			BasePath: base,
			SEO:      core.MakeSEO(cfg, post.Title+" - "+cfg.SiteTitle, core.Excerpt(post.Markdown, 140), core.WithBase(*basePath, "/post/"+post.Slug+"/"), "article", post.Date.Format(time.RFC3339)),
			Post:     post,
		}); err != nil {
			log.Fatal(err)
		}
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

func renderToFile(dstPath, templatePath, basePath string, tagURLs map[string]string, data any) error {
	tpl, err := parseTemplate(basePath, tagURLs, templatePath)
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

func parseTemplate(basePath string, tagURLs map[string]string, page string) (*template.Template, error) {
	return core.ParseTemplate(page, basePath, func(tag string) string {
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
