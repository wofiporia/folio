package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "folio/internal/folio"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	return filepath.Dir(wd)
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func withWorkdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestConfigAndSEO(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.json")
	mustWriteFile(t, cfgPath, "\uFEFF{\"site_title\":\"\",\"author_github\":\"abc\",\"comments_provider\":\"UTTERANCES\"}")

	cfg, err := core.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.SiteTitle == "" || cfg.AuthorGitHub != "https://github.com/abc" || cfg.CommentsProvider != "utterances" {
		t.Fatalf("unexpected normalize result: %+v", cfg)
	}

	seo := core.MakeSEO(cfg, "t", "", "/p", "", "")
	if seo.Description == "" || seo.OGType == "" || seo.CanonicalURL == "" || seo.OGURL == "" {
		t.Fatalf("unexpected seo: %+v", seo)
	}
}

func TestURLTagPaginateAndSearch(t *testing.T) {
	if got := core.CanonicalURL("https://x.com/repo", "/repo/post/a"); got != "https://x.com/repo/post/a" {
		t.Fatalf("unexpected canonical: %s", got)
	}
	if got := core.WithBase("/repo", "/x"); got != "/repo/x" {
		t.Fatalf("unexpected WithBase: %s", got)
	}
	if got := core.NormalizeBasePath("repo/"); got != "/repo" {
		t.Fatalf("unexpected NormalizeBasePath: %s", got)
	}
	if got := core.BuildTagURL("/repo", "Go", "static"); !strings.Contains(got, "/tags/go/") {
		t.Fatalf("unexpected tag url: %s", got)
	}

	posts := []core.Post{
		{Slug: "a", Tags: []string{"Go", "Tag"}, Date: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Markdown: "# A", DateDisplay: "2026-03-01"},
		{Slug: "b", Tags: []string{"go", "Tag", "Tag 2"}, Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Markdown: "B", DateDisplay: "2026-02-01"},
	}
	stats := core.BuildTagStats(posts, "/repo", "dynamic")
	if len(stats) < 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	staticStats, tagSlugs, tagURLs := core.BuildStaticTagStats(posts, "/repo")
	if len(staticStats) == 0 || tagSlugs["Tag 2"] == "" || tagURLs["Tag"] == "" {
		t.Fatalf("unexpected static tags: %+v %+v %+v", staticStats, tagSlugs, tagURLs)
	}
	filtered := core.FilterPostsByTag(posts, "go")
	if len(filtered) != 2 {
		t.Fatalf("filter failed: %+v", filtered)
	}

	groups := core.BuildArchiveGroups(posts)
	if len(groups) != 2 || groups[0].Label != "2026-03" {
		t.Fatalf("unexpected groups: %+v", groups)
	}

	visible, total, current := core.PaginatePosts(posts, 10, 1)
	if len(visible) != 1 || total != 2 || current != 2 {
		t.Fatalf("unexpected paginate: len=%d total=%d current=%d", len(visible), total, current)
	}

	docs := core.MakeSearchDocs(posts)
	if len(docs) != 2 || docs[0].Content == "" {
		t.Fatalf("unexpected docs: %+v", docs)
	}
	if got := core.Excerpt("abc", 2); got != "ab..." {
		t.Fatalf("unexpected excerpt: %s", got)
	}
	if got := core.NormalizeSearchText("# a\nb"); got != "a b" {
		t.Fatalf("unexpected search text: %s", got)
	}
}

func TestPostsCommentsThemeAndTemplate(t *testing.T) {
	tmp := t.TempDir()
	postsDir := filepath.Join(tmp, "posts")
	mustWriteFile(t, filepath.Join(postsDir, "a.md"), `---
title: "Alpha"
author: "Alice"
date: "2026-03-02T01:02:03Z"
tags: ["Go","Blog"]
draft: false
---
# Hello
`)
	mustWriteFile(t, filepath.Join(postsDir, "b.md"), `---
draft: true
date: "2026-03-01"
---
draft body`)
	mustWriteFile(t, filepath.Join(postsDir, "c.md"), "No front matter")
	if err := os.Mkdir(filepath.Join(postsDir, "bad.md"), 0o755); err != nil {
		t.Fatalf("create bad.md dir failed: %v", err)
	}

	post, err := core.LoadPost(filepath.Join(postsDir, "a.md"), "fallback")
	if err != nil {
		t.Fatalf("LoadPost error: %v", err)
	}
	if post.Title != "Alpha" || post.Author != "Alice" || len(post.Tags) != 2 || post.HTML == "" {
		t.Fatalf("unexpected post: %+v", post)
	}

	list, err := core.LoadPosts(postsDir, "fallback")
	if err == nil {
		t.Fatalf("expected LoadPosts to fail on malformed post")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Fatalf("expected malformed file path in error, got: %v", err)
	}
	if list != nil {
		t.Fatalf("expected nil post list on load error, got %d entries", len(list))
	}
	_ = os.RemoveAll(filepath.Join(postsDir, "bad.md"))

	list, err = core.LoadPosts(postsDir, "fallback")
	if err != nil {
		t.Fatalf("LoadPosts error after removing bad post: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 visible posts, got %d", len(list))
	}
	if _, err := core.LoadPostBySlug(postsDir, "a", "x"); err != nil {
		t.Fatalf("LoadPostBySlug error: %v", err)
	}

	cfg := core.DefaultConfig()
	cfg.CommentsProvider = "utterances"
	cfg.CommentsRepo = "o/r"
	cfg.CommentsIssueTerm = "slug"
	cc := core.BuildCommentConfig(cfg, core.Post{Slug: "hello"})
	if !cc.Enabled || cc.IssueTerm != "hello" {
		t.Fatalf("unexpected utterances config: %+v", cc)
	}

	cfg.CommentsProvider = "giscus"
	cfg.CommentsRepo = "o/r"
	cfg.CommentsRepoID = "rid"
	cfg.CommentsCategory = "cat"
	cfg.CommentsCategoryID = "cid"
	cc = core.BuildCommentConfig(cfg, core.Post{})
	if !cc.Enabled || cc.Provider != "giscus" {
		t.Fatalf("unexpected giscus config: %+v", cc)
	}

	if got := core.SlugifyTag("  C++  "); got != "c" {
		t.Fatalf("unexpected SlugifyTag: %s", got)
	}
	if got := core.NormalizeThemeName("bad/theme"); got != "default" {
		t.Fatalf("unexpected NormalizeThemeName: %s", got)
	}

	// Use real repo themes for template parse coverage.
	withWorkdir(t, repoRoot(t))
	if p := core.ResolveTemplatePath("default", "index.html"); p == "" {
		t.Fatalf("ResolveTemplatePath failed")
	}
	if p := core.ResolveStaticPath("default", "style.css"); p == "" {
		t.Fatalf("ResolveStaticPath failed")
	}
	tpl, err := core.ParseTemplate("default", "index.html", func(tag string) string { return "/tags/" + tag })
	if err != nil || tpl == nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}
}

func TestSearchIndexJSONShape(t *testing.T) {
	docs := core.MakeSearchDocs([]core.Post{
		{Title: "t", Slug: "s", DateDisplay: "2026-03-01", Tags: []string{"x"}, Markdown: "hello"},
	})
	b, err := json.Marshal(docs)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	if !strings.Contains(string(b), `"title":"t"`) || !strings.Contains(string(b), `"slug":"s"`) {
		t.Fatalf("unexpected json: %s", string(b))
	}
}

func TestRoutingHelpers(t *testing.T) {
	if !core.IsValidSlug("hello-1") || core.IsValidSlug("hello_1") {
		t.Fatalf("IsValidSlug mismatch")
	}
	if !core.IsValidTag("go") || core.IsValidTag("<script>") {
		t.Fatalf("IsValidTag mismatch")
	}
	if got := core.ParsePositiveIntOrDefault("0", 9); got != 9 {
		t.Fatalf("ParsePositiveIntOrDefault mismatch: %d", got)
	}

	p := core.BuildDynamicPagination("", 2, 3)
	if p.PrevURL == "" || p.NextURL == "" || len(p.Pages) != 3 {
		t.Fatalf("BuildDynamicPagination mismatch: %+v", p)
	}
	if got := core.DynamicTagsPageURL("", "Go Lang", 2); !strings.Contains(got, "tag=Go+Lang") || !strings.Contains(got, "page=2") {
		t.Fatalf("DynamicTagsPageURL mismatch: %s", got)
	}
	if got := core.DynamicArchivesPageURL("", 2); got != "/archives?page=2" {
		t.Fatalf("DynamicArchivesPageURL mismatch: %s", got)
	}

	pTags := core.BuildDynamicTagsPagination("", "go", 2, 3)
	if pTags.PrevURL == "" || pTags.NextURL == "" || len(pTags.Pages) != 3 {
		t.Fatalf("BuildDynamicTagsPagination mismatch: %+v", pTags)
	}
	pTagsSingle := core.BuildDynamicTagsPagination("", "", 1, 1)
	if pTagsSingle.PrevURL != "" || pTagsSingle.NextURL != "" || len(pTagsSingle.Pages) != 0 {
		t.Fatalf("BuildDynamicTagsPagination single-page mismatch: %+v", pTagsSingle)
	}

	pArchives := core.BuildDynamicArchivesPagination("", 2, 3)
	if pArchives.PrevURL == "" || pArchives.NextURL == "" || len(pArchives.Pages) != 3 {
		t.Fatalf("BuildDynamicArchivesPagination mismatch: %+v", pArchives)
	}
	pArchivesSingle := core.BuildDynamicArchivesPagination("", 1, 1)
	if pArchivesSingle.PrevURL != "" || pArchivesSingle.NextURL != "" || len(pArchivesSingle.Pages) != 0 {
		t.Fatalf("BuildDynamicArchivesPagination single-page mismatch: %+v", pArchivesSingle)
	}

	ps := core.BuildStaticPagination("/repo", 2, 3)
	if ps.PrevURL != "/repo/" || ps.NextURL != "/repo/page/3/" {
		t.Fatalf("BuildStaticPagination mismatch: %+v", ps)
	}
	psSingle := core.BuildStaticPagination("/repo", 1, 1)
	if psSingle.PrevURL != "" || psSingle.NextURL != "" || len(psSingle.Pages) != 0 {
		t.Fatalf("BuildStaticPagination single-page mismatch: %+v", psSingle)
	}

	pStaticTags := core.BuildStaticTagsPagination("/repo", "go", 2, 3)
	if pStaticTags.PrevURL == "" || pStaticTags.NextURL == "" || len(pStaticTags.Pages) != 3 {
		t.Fatalf("BuildStaticTagsPagination mismatch: %+v", pStaticTags)
	}
	pStaticTagsSingle := core.BuildStaticTagsPagination("/repo", "", 1, 1)
	if pStaticTagsSingle.PrevURL != "" || pStaticTagsSingle.NextURL != "" || len(pStaticTagsSingle.Pages) != 0 {
		t.Fatalf("BuildStaticTagsPagination single-page mismatch: %+v", pStaticTagsSingle)
	}

	pStaticArchives := core.BuildStaticArchivesPagination("/repo", 2, 3)
	if pStaticArchives.PrevURL == "" || pStaticArchives.NextURL == "" || len(pStaticArchives.Pages) != 3 {
		t.Fatalf("BuildStaticArchivesPagination mismatch: %+v", pStaticArchives)
	}
	pStaticArchivesSingle := core.BuildStaticArchivesPagination("/repo", 1, 1)
	if pStaticArchivesSingle.PrevURL != "" || pStaticArchivesSingle.NextURL != "" || len(pStaticArchivesSingle.Pages) != 0 {
		t.Fatalf("BuildStaticArchivesPagination single-page mismatch: %+v", pStaticArchivesSingle)
	}

	if got := core.StaticTagsPageURL("/repo", "", 1); got != "/repo/tags/" {
		t.Fatalf("StaticTagsPageURL root mismatch: %s", got)
	}
	if got := core.StaticTagsPageURL("/repo", "", 2); got != "/repo/tags/page/2/" {
		t.Fatalf("StaticTagsPageURL paged mismatch: %s", got)
	}
	if got := core.StaticTagsPageURL("/repo", "go", 2); got != "/repo/tags/go/page/2/" {
		t.Fatalf("StaticTagsPageURL mismatch: %s", got)
	}
	if got := core.StaticArchivesPageURL("/repo", 1); got != "/repo/archives/" {
		t.Fatalf("StaticArchivesPageURL root mismatch: %s", got)
	}
	if got := core.StaticArchivesPageURL("/repo", 2); got != "/repo/archives/page/2/" {
		t.Fatalf("StaticArchivesPageURL mismatch: %s", got)
	}
}

func TestPluginManagerConstruction(t *testing.T) {
	cfg := core.DefaultConfig()
	if m := core.NewPluginManager(".", cfg); m != nil {
		t.Fatalf("expected nil plugin manager for empty config")
	}

	cfg.Plugins = core.PluginConfigs{
		{
			Name:  "music_player",
			Hooks: []string{"before_page_render"},
		},
	}
	if m := core.NewPluginManager(".", cfg); m == nil {
		t.Fatalf("expected non-nil plugin manager when plugins are configured")
	}
}

func TestPluginNamesConfigParsing(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.json")
	mustWriteFile(t, cfgPath, `{
  "site_title": "Folio",
  "plugins": ["music_player"]
}`)

	cfg, err := core.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if len(cfg.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(cfg.Plugins))
	}
	if cfg.Plugins[0].Name != "music_player" {
		t.Fatalf("unexpected plugin names: %+v", cfg.Plugins)
	}
}
