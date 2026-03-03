package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
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
	SiteName      string
	PublishedTime string
}

type IndexData struct {
	Title    string
	BasePath string
	SEO      SEO
	Posts    []Post
}

type PostData struct {
	Title    string
	BasePath string
	SEO      SEO
	Post     Post
}

type TagStat struct {
	Name  string
	Count int
	URL   string
}

type TagsData struct {
	Title      string
	BasePath   string
	SEO        SEO
	CurrentTag string
	Tags       []TagStat
	Posts      []Post
}

type ArchiveGroup struct {
	Label string
	Posts []Post
}

type ArchivesData struct {
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

var (
	reLink   = regexp.MustCompile(`\[(.+?)\]\(([^)\s]+)\)`)
	reBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic = regexp.MustCompile(`\*(.+?)\*`)
	reCode   = regexp.MustCompile("`([^`]+)`")
)

func main() {
	outDir := flag.String("out", "dist", "output directory")
	basePath := flag.String("base-path", "", "base path prefix, e.g. /repo")
	siteURL := flag.String("site-url", "", "absolute site url, e.g. https://example.com")
	flag.Parse()

	posts, err := loadPosts("posts")
	if err != nil {
		log.Fatal(err)
	}

	tagStats, tagSlugs, tagURLs := buildStaticTagStats(posts, *basePath)

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

	base := normalizeBasePath(*basePath)

	if err := renderToFile(filepath.Join(*outDir, "index.html"), "templates/index.html", *basePath, tagURLs, IndexData{
		Title:    "Folio",
		BasePath: base,
		SEO:      makeSEO(*siteURL, "Folio", "一个基于 Go 和文件系统的轻量博客。", withBase(*basePath, "/"), "website", ""),
		Posts:    posts,
	}); err != nil {
		log.Fatal(err)
	}

	if err := renderToFile(filepath.Join(*outDir, "archives", "index.html"), "templates/archives.html", *basePath, tagURLs, ArchivesData{
		Title:    "归档",
		BasePath: base,
		SEO:      makeSEO(*siteURL, "归档 - Folio", "按月份浏览历史文章。", withBase(*basePath, "/archives"), "website", ""),
		Groups:   buildArchiveGroups(posts),
	}); err != nil {
		log.Fatal(err)
	}

	if err := renderToFile(filepath.Join(*outDir, "search", "index.html"), "templates/search.html", *basePath, tagURLs, SearchPageData{
		Title:    "搜索",
		BasePath: base,
		SEO:      makeSEO(*siteURL, "搜索 - Folio", "在博客中搜索标题、标签和正文。", withBase(*basePath, "/search"), "website", ""),
	}); err != nil {
		log.Fatal(err)
	}

	if err := writeJSON(filepath.Join(*outDir, "search-index.json"), makeSearchDocs(posts)); err != nil {
		log.Fatal(err)
	}

	if err := renderToFile(filepath.Join(*outDir, "tags", "index.html"), "templates/tags.html", *basePath, tagURLs, TagsData{
		Title:      "标签",
		BasePath:   base,
		SEO:        makeSEO(*siteURL, "标签 - Folio", "按标签浏览文章内容。", withBase(*basePath, "/tags"), "website", ""),
		CurrentTag: "",
		Tags:       tagStats,
		Posts:      posts,
	}); err != nil {
		log.Fatal(err)
	}

	for _, stat := range tagStats {
		filtered := filterPostsByTag(posts, stat.Name)
		slug := tagSlugs[stat.Name]
		if err := renderToFile(filepath.Join(*outDir, "tags", slug, "index.html"), "templates/tags.html", *basePath, tagURLs, TagsData{
			Title:      "标签: " + stat.Name,
			BasePath:   base,
			SEO:        makeSEO(*siteURL, "标签: "+stat.Name+" - Folio", "按标签浏览文章内容。", withBase(*basePath, "/tags/"+slug+"/"), "website", ""),
			CurrentTag: stat.Name,
			Tags:       tagStats,
			Posts:      filtered,
		}); err != nil {
			log.Fatal(err)
		}
	}

	for _, post := range posts {
		if err := renderToFile(filepath.Join(*outDir, "post", post.Slug, "index.html"), "templates/post.html", *basePath, tagURLs, PostData{
			Title:    post.Title,
			BasePath: base,
			SEO:      makeSEO(*siteURL, post.Title+" - Folio", excerpt(post.Markdown, 140), withBase(*basePath, "/post/"+post.Slug+"/"), "article", post.Date.Format(time.RFC3339)),
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

func parseTemplate(basePath string, tagURLs map[string]string, files ...string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"tagURL": func(tag string) string {
			if u, ok := tagURLs[tag]; ok {
				return u
			}
			return withBase(basePath, "/tags/")
		},
	}
	return template.New(filepath.Base(files[0])).Funcs(funcMap).ParseFiles(files...)
}

func loadPosts(dir string) ([]Post, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}

	posts := make([]Post, 0, len(files))
	for _, path := range files {
		post, err := loadPost(path)
		if err != nil {
			log.Printf("skip post %s: %v", path, err)
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

func loadPost(path string) (Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Post{}, err
	}

	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fm, body := splitFrontMatter(string(b))

	post := Post{
		Slug:     slug,
		Title:    fallbackTitle(fm["title"], slug),
		Tags:     parseList(fm["tags"]),
		Draft:    strings.EqualFold(strings.TrimSpace(fm["draft"]), "true"),
		Markdown: strings.TrimSpace(body),
	}

	post.Date = parseDateOrNow(fm["date"])
	post.DateDisplay = post.Date.Format("2006-01-02")
	post.HTML = template.HTML(renderMarkdown(post.Markdown))
	return post, nil
}

func buildStaticTagStats(posts []Post, basePath string) ([]TagStat, map[string]string, map[string]string) {
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
		baseSlug := slugifyTag(name)
		slug := baseSlug
		i := 2
		for used[slug] {
			slug = fmt.Sprintf("%s-%d", baseSlug, i)
			i++
		}
		used[slug] = true
		tagSlugs[name] = slug
		tagURLs[name] = withBase(basePath, "/tags/"+slug+"/")
		stats = append(stats, TagStat{
			Name:  name,
			Count: counts[name],
			URL:   tagURLs[name],
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})
	return stats, tagSlugs, tagURLs
}

func filterPostsByTag(posts []Post, target string) []Post {
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

func buildArchiveGroups(posts []Post) []ArchiveGroup {
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

func makeSearchDocs(posts []Post) []SearchDoc {
	docs := make([]SearchDoc, 0, len(posts))
	for _, post := range posts {
		docs = append(docs, SearchDoc{
			Title:   post.Title,
			Slug:    post.Slug,
			Date:    post.DateDisplay,
			Tags:    post.Tags,
			Content: normalizeSearchText(post.Markdown),
		})
	}
	return docs
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

	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
	}
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

func normalizeSearchText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	replacer := strings.NewReplacer(
		"#", " ", "*", " ", "`", " ", ">", " ", "[", " ", "]", " ",
		"(", " ", ")", " ", "-", " ", "_", " ", "\n", " ", "\t", " ",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func canonicalURL(siteURL, path string) string {
	site := strings.TrimRight(strings.TrimSpace(siteURL), "/")
	if site == "" {
		return path
	}
	return site + path
}

func makeSEO(siteURL, title, desc, path, ogType, publishedTime string) SEO {
	url := canonicalURL(siteURL, path)
	return SEO{
		Description:   desc,
		CanonicalURL:  url,
		OGType:        ogType,
		OGURL:         url,
		SiteName:      "Folio",
		PublishedTime: publishedTime,
	}
}

func excerpt(s string, n int) string {
	text := normalizeSearchText(s)
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n]) + "..."
}

func withBase(basePath, path string) string {
	if basePath == "" {
		return path
	}
	base := "/" + strings.Trim(basePath, "/")
	if strings.HasPrefix(path, "/") {
		return base + path
	}
	return base + "/" + path
}

func normalizeBasePath(basePath string) string {
	if strings.TrimSpace(basePath) == "" {
		return ""
	}
	return "/" + strings.Trim(basePath, "/")
}

func slugifyTag(s string) string {
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
