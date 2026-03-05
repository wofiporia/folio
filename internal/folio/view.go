package folio

import (
	"fmt"
	"html/template"
)

type IndexPageData struct {
	Title           string
	BasePath        string
	StylePath       string
	FaviconPath     string
	SiteDescription string
	SEO             SEO
	Posts           []Post
}

type PostPageData struct {
	Title       string
	BasePath    string
	StylePath   string
	FaviconPath string
	SEO         SEO
	Post        Post
}

type TagsPageData struct {
	Title       string
	BasePath    string
	StylePath   string
	FaviconPath string
	SEO         SEO
	CurrentTag  string
	Tags        []TagStat
	Posts       []Post
}

type ArchivesPageData struct {
	Title       string
	BasePath    string
	StylePath   string
	FaviconPath string
	SEO         SEO
	Groups      []ArchiveGroup
}

type SearchPageData struct {
	Title       string
	BasePath    string
	StylePath   string
	FaviconPath string
	SEO         SEO
}

func ParseTemplate(theme, pageRel string, tagResolver func(string) string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"tagURL": tagResolver,
	}
	head := ResolveTemplatePath(theme, "partials/head-common.html")
	nav := ResolveTemplatePath(theme, "partials/nav.html")
	page := ResolveTemplatePath(theme, pageRel)
	if head == "" || nav == "" || page == "" {
		return nil, fmt.Errorf("theme templates not found: theme=%s page=%s", theme, pageRel)
	}
	files := []string{head, nav, page}
	return template.New(pageRel).Funcs(funcMap).ParseFiles(files...)
}
