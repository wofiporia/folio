package folio

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Post struct {
	Slug        string
	Title       string
	Author      string
	Date        time.Time
	DateDisplay string
	Tags        []string
	Draft       bool
	Markdown    string
	HTML        template.HTML
}

type SearchDoc struct {
	Title   string   `json:"title"`
	Slug    string   `json:"slug"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
}

type SEO struct {
	Description   string
	CanonicalURL  string
	OGType        string
	OGURL         string
	OGImage       string
	SiteName      string
	PublishedTime string
}

type AppConfig struct {
	SiteTitle          string `json:"site_title"`
	SiteDescription    string `json:"site_description"`
	SiteURL            string `json:"site_url"`
	AuthorName         string `json:"author_name"`
	DefaultDescription string `json:"default_description"`
	DefaultOGImage     string `json:"default_og_image"`
	DefaultOGType      string `json:"default_og_type"`
}

type TagStat struct {
	Name  string
	Count int
	URL   string
}

type ArchiveGroup struct {
	Label string
	Posts []Post
}

var (
	reLink   = regexp.MustCompile(`\[(.+?)\]\(([^)\s]+)\)`)
	reBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic = regexp.MustCompile(`\*(.+?)\*`)
	reCode   = regexp.MustCompile("`([^`]+)`")
)

func DefaultConfig() AppConfig {
	return AppConfig{
		SiteTitle:          "Folio",
		SiteDescription:    "一个基于 Go 和文件系统的轻量博客。",
		SiteURL:            "",
		AuthorName:         "Anonymous",
		DefaultDescription: "一个基于 Go 和文件系统的轻量博客。",
		DefaultOGImage:     "",
		DefaultOGType:      "website",
	}
}

func (c *AppConfig) normalize() {
	d := DefaultConfig()
	if strings.TrimSpace(c.SiteTitle) == "" {
		c.SiteTitle = d.SiteTitle
	}
	if strings.TrimSpace(c.SiteDescription) == "" {
		c.SiteDescription = d.SiteDescription
	}
	c.SiteURL = strings.TrimRight(strings.TrimSpace(c.SiteURL), "/")
	if strings.TrimSpace(c.AuthorName) == "" {
		c.AuthorName = d.AuthorName
	}
	if strings.TrimSpace(c.DefaultDescription) == "" {
		c.DefaultDescription = c.SiteDescription
	}
	if strings.TrimSpace(c.DefaultOGType) == "" {
		c.DefaultOGType = d.DefaultOGType
	}
}

func LoadConfig(path string) (AppConfig, error) {
	cfg := DefaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return cfg, nil
	}
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	cfg.normalize()
	return cfg, nil
}

func CanonicalURL(siteURL, path string) string {
	site := strings.TrimRight(strings.TrimSpace(siteURL), "/")
	if site == "" {
		return path
	}
	return site + path
}

func MakeSEO(cfg AppConfig, title, desc, path, ogType, publishedTime string) SEO {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		desc = cfg.DefaultDescription
	}
	ogType = strings.TrimSpace(ogType)
	if ogType == "" {
		ogType = cfg.DefaultOGType
	}
	url := CanonicalURL(cfg.SiteURL, path)
	return SEO{
		Description:   desc,
		CanonicalURL:  url,
		OGType:        ogType,
		OGURL:         url,
		OGImage:       cfg.DefaultOGImage,
		SiteName:      cfg.SiteTitle,
		PublishedTime: publishedTime,
	}
}

func WithBase(basePath, path string) string {
	if basePath == "" {
		return path
	}
	base := "/" + strings.Trim(basePath, "/")
	if strings.HasPrefix(path, "/") {
		return base + path
	}
	return base + "/" + path
}

func NormalizeBasePath(basePath string) string {
	if strings.TrimSpace(basePath) == "" {
		return ""
	}
	return "/" + strings.Trim(basePath, "/")
}

func BuildTagURL(basePath, tag, mode string) string {
	if mode == "static" {
		return WithBase(basePath, "/tags/"+SlugifyTag(tag)+"/")
	}
	return WithBase(basePath, "/tags?tag="+url.QueryEscape(tag))
}

func BuildTagStats(posts []Post, basePath, mode string) []TagStat {
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
		stats = append(stats, TagStat{Name: name, Count: count, URL: BuildTagURL(basePath, name, mode)})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})
	return stats
}

func BuildStaticTagStats(posts []Post, basePath string) ([]TagStat, map[string]string, map[string]string) {
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

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	used := map[string]bool{}
	tagSlugs := map[string]string{}
	tagURLs := map[string]string{}
	stats := make([]TagStat, 0, len(names))

	for _, name := range names {
		baseSlug := SlugifyTag(name)
		slug := baseSlug
		i := 2
		for used[slug] {
			slug = fmt.Sprintf("%s-%d", baseSlug, i)
			i++
		}
		used[slug] = true
		tagSlugs[name] = slug
		tagURLs[name] = WithBase(basePath, "/tags/"+slug+"/")
		stats = append(stats, TagStat{Name: name, Count: counts[name], URL: tagURLs[name]})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})
	return stats, tagSlugs, tagURLs
}

func FilterPostsByTag(posts []Post, target string) []Post {
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

func BuildArchiveGroups(posts []Post) []ArchiveGroup {
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

func MakeSearchDocs(posts []Post) []SearchDoc {
	docs := make([]SearchDoc, 0, len(posts))
	for _, post := range posts {
		docs = append(docs, SearchDoc{
			Title:   post.Title,
			Slug:    post.Slug,
			Date:    post.DateDisplay,
			Tags:    post.Tags,
			Content: NormalizeSearchText(post.Markdown),
		})
	}
	return docs
}

func LoadPosts(dir, fallbackAuthor string) ([]Post, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}

	posts := make([]Post, 0, len(files))
	for _, path := range files {
		post, err := LoadPost(path, fallbackAuthor)
		if err != nil {
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
	return posts, nil
}

func LoadPostBySlug(dir, slug, fallbackAuthor string) (Post, error) {
	path := filepath.Join(dir, slug+".md")
	if _, err := os.Stat(path); err != nil {
		return Post{}, err
	}
	return LoadPost(path, fallbackAuthor)
}

func LoadPost(path, fallbackAuthor string) (Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Post{}, err
	}

	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fm, body := splitFrontMatter(string(b))

	post := Post{
		Slug:     slug,
		Title:    fallbackTitle(fm["title"], slug),
		Author:   fallbackAuthorName(fm["author"], fallbackAuthor),
		Tags:     parseList(fm["tags"]),
		Draft:    strings.EqualFold(strings.TrimSpace(fm["draft"]), "true"),
		Markdown: strings.TrimSpace(body),
	}

	post.Date = parseDateOrNow(fm["date"])
	post.DateDisplay = post.Date.Format("2006-01-02")
	post.HTML = template.HTML(renderMarkdown(post.Markdown))
	return post, nil
}

func Excerpt(s string, n int) string {
	text := NormalizeSearchText(s)
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n]) + "..."
}

func NormalizeSearchText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	replacer := strings.NewReplacer(
		"#", " ", "*", " ", "`", " ", ">", " ", "[", " ", "]", " ",
		"(", " ", ")", " ", "-", " ", "_", " ", "\n", " ", "\t", " ",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func SlugifyTag(s string) string {
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

	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"}
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

func fallbackAuthorName(author, fallback string) string {
	author = strings.TrimSpace(author)
	if author != "" {
		return author
	}
	return strings.TrimSpace(fallback)
}

func renderMarkdown(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(input))
	var out strings.Builder
	inCode := false
	listTag := ""
	var paragraph []string

	closeList := func() {
		if listTag != "" {
			out.WriteString("</" + listTag + ">\n")
			listTag = ""
		}
	}
	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		text := formatInline(strings.Join(paragraph, " "))
		out.WriteString("<p>" + text + "</p>\n")
		paragraph = paragraph[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			closeList()
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
			closeList()
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			flushParagraph()
			closeList()
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 6 {
				level = 6
			}
			text := strings.TrimSpace(trimmed[level:])
			out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, formatInline(text), level))
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			flushParagraph()
			closeList()
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			out.WriteString("<blockquote><p>" + formatInline(text) + "</p></blockquote>\n")
			continue
		}

		if item, tag, ok := parseListItem(trimmed); ok {
			flushParagraph()
			if listTag != tag {
				closeList()
				listTag = tag
				out.WriteString("<" + listTag + ">\n")
			}
			out.WriteString("<li>" + formatInline(item) + "</li>\n")
			continue
		}

		closeList()
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	closeList()
	if inCode {
		out.WriteString("</code></pre>\n")
	}
	return out.String()
}

func parseListItem(line string) (item, tag string, ok bool) {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(line[2:]), "ul", true
	}

	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(line) && line[i] == '.' && line[i+1] == ' ' {
		return strings.TrimSpace(line[i+2:]), "ol", true
	}
	return "", "", false
}

func formatInline(s string) string {
	out := template.HTMLEscapeString(s)
	out = reLink.ReplaceAllString(out, `<a href="$2">$1</a>`)
	out = reBold.ReplaceAllString(out, `<strong>$1</strong>`)
	out = reItalic.ReplaceAllString(out, `<em>$1</em>`)
	out = reCode.ReplaceAllString(out, `<code>$1</code>`)
	return out
}
