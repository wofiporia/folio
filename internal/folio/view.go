package folio

import (
	"fmt"
	"html/template"
)

type IndexPageData struct {
	Title           string
	BasePath        string
	AuthorGitHub    string
	StylePath       string
	FaviconPath     string
	SiteDescription string
	SEO             SEO
	Posts           []Post
	Pagination      Pagination
}

type PostPageData struct {
	Title        string
	BasePath     string
	AuthorGitHub string
	StylePath    string
	FaviconPath  string
	SEO          SEO
	Post         Post
	Comments     CommentConfig
}

type TagsPageData struct {
	Title        string
	BasePath     string
	AuthorGitHub string
	StylePath    string
	FaviconPath  string
	SEO          SEO
	CurrentTag   string
	Tags         []TagStat
	Posts        []Post
	Pagination   Pagination
}

type ArchivesPageData struct {
	Title        string
	BasePath     string
	AuthorGitHub string
	StylePath    string
	FaviconPath  string
	SEO          SEO
	Groups       []ArchiveGroup
	Pagination   Pagination
}

type SearchPageData struct {
	Title        string
	BasePath     string
	AuthorGitHub string
	StylePath    string
	FaviconPath  string
	SEO          SEO
}

type NotFoundPageData struct {
	Title        string
	BasePath     string
	AuthorGitHub string
	StylePath    string
	FaviconPath  string
	SEO          SEO
	Message      string
}

type PageLink struct {
	Number  int
	URL     string
	Current bool
}

type Pagination struct {
	CurrentPage int
	TotalPages  int
	PrevURL     string
	NextURL     string
	Pages       []PageLink
}

type CommentConfig struct {
	Enabled        bool
	Provider       string
	Repo           string
	RepoID         string
	Category       string
	CategoryID     string
	Mapping        string
	Theme          string
	Lang           string
	Label          string
	IssueTerm      string
	DiscussionTerm string
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
