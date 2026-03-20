package folio

import (
	"net/url"
	"strconv"
	"strings"
)

func IsValidSlug(slug string) bool {
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

func IsValidTag(tag string) bool {
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

func ParsePositiveIntOrDefault(raw string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return def
	}
	return n
}

func BuildDynamicPagination(basePath string, currentPage, totalPages int) Pagination {
	p := Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = DynamicIndexPageURL(basePath, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = DynamicIndexPageURL(basePath, currentPage+1)
	}
	links := make([]PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, PageLink{
			Number:  i,
			URL:     DynamicIndexPageURL(basePath, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func DynamicIndexPageURL(basePath string, page int) string {
	if page <= 1 {
		return WithBase(basePath, "/")
	}
	return WithBase(basePath, "/?page="+strconv.Itoa(page))
}

func BuildDynamicTagsPagination(basePath, currentTag string, currentPage, totalPages int) Pagination {
	p := Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = DynamicTagsPageURL(basePath, currentTag, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = DynamicTagsPageURL(basePath, currentTag, currentPage+1)
	}
	links := make([]PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, PageLink{
			Number:  i,
			URL:     DynamicTagsPageURL(basePath, currentTag, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func BuildDynamicArchivesPagination(basePath string, currentPage, totalPages int) Pagination {
	p := Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = DynamicArchivesPageURL(basePath, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = DynamicArchivesPageURL(basePath, currentPage+1)
	}
	links := make([]PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, PageLink{
			Number:  i,
			URL:     DynamicArchivesPageURL(basePath, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func DynamicTagsPageURL(basePath, currentTag string, page int) string {
	values := url.Values{}
	if strings.TrimSpace(currentTag) != "" {
		values.Set("tag", currentTag)
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	if encoded := values.Encode(); encoded != "" {
		return WithBase(basePath, "/tags?"+encoded)
	}
	return WithBase(basePath, "/tags")
}

func DynamicArchivesPageURL(basePath string, page int) string {
	if page <= 1 {
		return WithBase(basePath, "/archives")
	}
	return WithBase(basePath, "/archives?page="+strconv.Itoa(page))
}

func BuildStaticPagination(basePath string, currentPage, totalPages int) Pagination {
	p := Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = StaticIndexPageURL(basePath, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = StaticIndexPageURL(basePath, currentPage+1)
	}
	links := make([]PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, PageLink{
			Number:  i,
			URL:     StaticIndexPageURL(basePath, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func StaticIndexPageURL(basePath string, page int) string {
	if page <= 1 {
		return WithBase(basePath, "/")
	}
	return WithBase(basePath, "/page/"+strconv.Itoa(page)+"/")
}

func BuildStaticTagsPagination(basePath, slug string, currentPage, totalPages int) Pagination {
	p := Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = StaticTagsPageURL(basePath, slug, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = StaticTagsPageURL(basePath, slug, currentPage+1)
	}
	links := make([]PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, PageLink{
			Number:  i,
			URL:     StaticTagsPageURL(basePath, slug, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func BuildStaticArchivesPagination(basePath string, currentPage, totalPages int) Pagination {
	p := Pagination{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
	}
	if totalPages <= 1 {
		return p
	}
	if currentPage > 1 {
		p.PrevURL = StaticArchivesPageURL(basePath, currentPage-1)
	}
	if currentPage < totalPages {
		p.NextURL = StaticArchivesPageURL(basePath, currentPage+1)
	}
	links := make([]PageLink, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		links = append(links, PageLink{
			Number:  i,
			URL:     StaticArchivesPageURL(basePath, i),
			Current: i == currentPage,
		})
	}
	p.Pages = links
	return p
}

func StaticTagsPageURL(basePath, slug string, page int) string {
	if strings.TrimSpace(slug) == "" {
		if page <= 1 {
			return WithBase(basePath, "/tags/")
		}
		return WithBase(basePath, "/tags/page/"+strconv.Itoa(page)+"/")
	}
	if page <= 1 {
		return WithBase(basePath, "/tags/"+slug+"/")
	}
	return WithBase(basePath, "/tags/"+slug+"/page/"+strconv.Itoa(page)+"/")
}

func StaticArchivesPageURL(basePath string, page int) string {
	if page <= 1 {
		return WithBase(basePath, "/archives/")
	}
	return WithBase(basePath, "/archives/page/"+strconv.Itoa(page)+"/")
}
