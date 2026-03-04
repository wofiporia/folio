package folio

import (
	"html/template"
	"path/filepath"
)

type IndexPageData struct {
	Title           string
	BasePath        string
	SiteDescription string
	SEO             SEO
	Posts           []Post
}

type PostPageData struct {
	Title    string
	BasePath string
	SEO      SEO
	Post     Post
}

type TagsPageData struct {
	Title      string
	BasePath   string
	SEO        SEO
	CurrentTag string
	Tags       []TagStat
	Posts      []Post
}

type ArchivesPageData struct {
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

func ParseTemplate(pagePath, basePath string, tagResolver func(string) string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"tagURL": tagResolver,
	}
	files := []string{
		"templates/partials/head-common.html",
		"templates/partials/nav.html",
		pagePath,
	}
	return template.New(filepath.Base(pagePath)).Funcs(funcMap).ParseFiles(files...)
}
