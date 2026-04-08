package folio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	PluginHookAfterPostsLoaded = "after_posts_loaded"
	PluginHookAfterStaticBuild = "after_static_build"
	PluginHookBeforePageRender = "before_page_render"
)

type PluginConfig struct {
	Name      string         `json:"name"`
	Command   string         `json:"command"`
	Args      []string       `json:"args"`
	Hooks     []string       `json:"hooks"`
	TimeoutMS int            `json:"timeout_ms"`
	FailFast  *bool          `json:"fail_fast"`
	Config    map[string]any `json:"config"`
}

type PluginManager struct {
	configDir string
	site      AppConfig
	plugins   []PluginConfig
}

type PluginBuildContext struct {
	Mode     string `json:"mode"`
	OutDir   string `json:"out_dir,omitempty"`
	BasePath string `json:"base_path,omitempty"`
}

type pluginSite struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	SiteURL     string `json:"site_url"`
	AuthorName  string `json:"author_name"`
	BasePath    string `json:"base_path,omitempty"`
}

type pluginPost struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Date        string   `json:"date"`
	DateDisplay string   `json:"date_display"`
	Tags        []string `json:"tags"`
	Draft       bool     `json:"draft"`
	Markdown    string   `json:"markdown"`
	HTML        string   `json:"html"`
}

type pluginRequest struct {
	Version      string             `json:"version"`
	Hook         string             `json:"hook"`
	Site         pluginSite         `json:"site"`
	Build        PluginBuildContext `json:"build"`
	Posts        []pluginPost       `json:"posts,omitempty"`
	PluginConfig map[string]any     `json:"plugin_config,omitempty"`
}

type pluginResponse struct {
	Posts    []pluginPost `json:"posts,omitempty"`
	HeadHTML string       `json:"head_html,omitempty"`
	Message  string       `json:"message,omitempty"`
	Error    string       `json:"error,omitempty"`
}

func NewPluginManager(configDir string, cfg AppConfig) *PluginManager {
	cleanDir := strings.TrimSpace(configDir)
	if cleanDir == "" {
		cleanDir = "."
	}

	normalized := make([]PluginConfig, 0, len(cfg.Plugins))
	for _, p := range cfg.Plugins {
		p.Name = strings.TrimSpace(p.Name)
		p.Command = strings.TrimSpace(p.Command)
		if p.Command == "" && p.Name != "" {
			if preset, err := loadPluginPreset(cleanDir, p.Name); err == nil {
				p = mergePluginConfig(preset, p)
			}
		}
		if p.Command == "" && p.Name != "" {
			p = inferPluginCommand(p)
		}
		if p.TimeoutMS <= 0 {
			p.TimeoutMS = 5000
		}
		cleanHooks := make([]string, 0, len(p.Hooks))
		for _, h := range p.Hooks {
			h = strings.ToLower(strings.TrimSpace(h))
			if h != "" {
				cleanHooks = append(cleanHooks, h)
			}
		}
		p.Hooks = cleanHooks
		if p.Command == "" || len(p.Hooks) == 0 {
			continue
		}
		normalized = append(normalized, p)
	}

	if len(normalized) == 0 {
		return nil
	}
	return &PluginManager{
		configDir: cleanDir,
		site:      cfg,
		plugins:   normalized,
	}
}

func (m *PluginManager) HeadSnippet(basePath string) template.HTML {
	if m == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range m.plugins {
		if !supportsHook(p.Hooks, PluginHookBeforePageRender) {
			continue
		}
		req := pluginRequest{
			Version: "1",
			Hook:    PluginHookBeforePageRender,
			Site: pluginSite{
				Title:       m.site.SiteTitle,
				Description: m.site.SiteDescription,
				SiteURL:     m.site.SiteURL,
				AuthorName:  m.site.AuthorName,
				BasePath:    basePath,
			},
			Build: PluginBuildContext{
				Mode:     "render",
				BasePath: basePath,
			},
			PluginConfig: p.Config,
		}
		resp, err := runPlugin(context.Background(), m.configDir, p, req)
		if err != nil {
			continue
		}
		if strings.TrimSpace(resp.Error) != "" {
			continue
		}
		if strings.TrimSpace(resp.HeadHTML) != "" {
			b.WriteString(resp.HeadHTML)
			b.WriteString("\n")
		}
	}
	return template.HTML(b.String())
}

func (m *PluginManager) RunAfterPostsLoaded(ctx context.Context, posts []Post, buildCtx PluginBuildContext) ([]Post, error) {
	if m == nil {
		return posts, nil
	}
	return m.runHook(ctx, PluginHookAfterPostsLoaded, posts, buildCtx)
}

func (m *PluginManager) RunAfterStaticBuild(ctx context.Context, posts []Post, buildCtx PluginBuildContext) error {
	if m == nil {
		return nil
	}
	_, err := m.runHook(ctx, PluginHookAfterStaticBuild, posts, buildCtx)
	return err
}

func (m *PluginManager) runHook(ctx context.Context, hook string, posts []Post, buildCtx PluginBuildContext) ([]Post, error) {
	current := make([]Post, len(posts))
	copy(current, posts)

	for _, p := range m.plugins {
		if strings.TrimSpace(p.Command) == "" {
			continue
		}
		if !supportsHook(p.Hooks, hook) {
			continue
		}

		req := pluginRequest{
			Version: "1",
			Hook:    hook,
			Site: pluginSite{
				Title:       m.site.SiteTitle,
				Description: m.site.SiteDescription,
				SiteURL:     m.site.SiteURL,
				AuthorName:  m.site.AuthorName,
				BasePath:    buildCtx.BasePath,
			},
			Build:        buildCtx,
			Posts:        toPluginPosts(current),
			PluginConfig: p.Config,
		}

		resp, err := runPlugin(ctx, m.configDir, p, req)
		if err != nil {
			if pluginFailFast(p) {
				return nil, err
			}
			continue
		}
		if strings.TrimSpace(resp.Error) != "" {
			runErr := fmt.Errorf("plugin %q returned error: %s", pluginName(p), strings.TrimSpace(resp.Error))
			if pluginFailFast(p) {
				return nil, runErr
			}
			continue
		}
		if hook == PluginHookAfterPostsLoaded && resp.Posts != nil {
			converted, err := fromPluginPosts(resp.Posts)
			if err != nil {
				if pluginFailFast(p) {
					return nil, fmt.Errorf("plugin %q invalid posts payload: %w", pluginName(p), err)
				}
				continue
			}
			current = converted
		}
	}

	return current, nil
}

func runPlugin(ctx context.Context, configDir string, p PluginConfig, req pluginRequest) (pluginResponse, error) {
	var resp pluginResponse
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}

	timeout := time.Duration(p.TimeoutMS) * time.Millisecond
	runCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	cmdPath := resolvePluginCommand(configDir, p.Command)
	cmd := exec.CommandContext(runCtx, cmdPath, p.Args...)
	cmd.Dir = configDir
	cmd.Stdin = bytes.NewReader(reqBytes)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return resp, fmt.Errorf("plugin %q failed: %s", pluginName(p), msg)
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return resp, nil
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return resp, fmt.Errorf("plugin %q returned invalid JSON: %w", pluginName(p), err)
	}
	return resp, nil
}

func toPluginPosts(posts []Post) []pluginPost {
	out := make([]pluginPost, 0, len(posts))
	for _, post := range posts {
		out = append(out, pluginPost{
			Slug:        post.Slug,
			Title:       post.Title,
			Author:      post.Author,
			Date:        post.Date.Format(time.RFC3339),
			DateDisplay: post.DateDisplay,
			Tags:        append([]string{}, post.Tags...),
			Draft:       post.Draft,
			Markdown:    post.Markdown,
			HTML:        string(post.HTML),
		})
	}
	return out
}

func fromPluginPosts(posts []pluginPost) ([]Post, error) {
	out := make([]Post, 0, len(posts))
	for _, p := range posts {
		d := time.Now()
		if strings.TrimSpace(p.Date) != "" {
			parsed, err := time.Parse(time.RFC3339, p.Date)
			if err != nil {
				return nil, err
			}
			d = parsed
		}

		post := Post{
			Slug:        p.Slug,
			Title:       p.Title,
			Author:      p.Author,
			Date:        d,
			DateDisplay: p.DateDisplay,
			Tags:        append([]string{}, p.Tags...),
			Draft:       p.Draft,
			Markdown:    p.Markdown,
		}
		if strings.TrimSpace(post.DateDisplay) == "" {
			post.DateDisplay = post.Date.Format("2006-01-02")
		}
		if strings.TrimSpace(p.HTML) == "" {
			post.HTML = template.HTML(renderMarkdown(post.Markdown))
		} else {
			post.HTML = template.HTML(p.HTML)
		}
		out = append(out, post)
	}
	return out, nil
}

func resolvePluginCommand(configDir, command string) string {
	if filepath.IsAbs(command) {
		return command
	}
	if strings.Contains(command, "/") || strings.HasPrefix(command, ".") {
		return filepath.Join(configDir, command)
	}
	return command
}

func supportsHook(hooks []string, hook string) bool {
	for _, h := range hooks {
		if strings.EqualFold(strings.TrimSpace(h), hook) {
			return true
		}
	}
	return false
}

func pluginFailFast(p PluginConfig) bool {
	if p.FailFast == nil {
		return true
	}
	return *p.FailFast
}

func pluginName(p PluginConfig) string {
	if strings.TrimSpace(p.Name) != "" {
		return p.Name
	}
	if strings.TrimSpace(p.Command) != "" {
		return p.Command
	}
	return "unnamed-plugin"
}

func loadPluginPreset(configDir, name string) (PluginConfig, error) {
	var preset PluginConfig
	presetPath := filepath.Join(configDir, "plugins", name, "plugin.json")
	b, err := os.ReadFile(presetPath)
	if err != nil {
		return preset, err
	}
	if err := json.Unmarshal(b, &preset); err != nil {
		return preset, err
	}
	if strings.TrimSpace(preset.Name) == "" {
		preset.Name = name
	}
	return preset, nil
}

func mergePluginConfig(base, override PluginConfig) PluginConfig {
	out := base
	if strings.TrimSpace(override.Name) != "" {
		out.Name = override.Name
	}
	if strings.TrimSpace(override.Command) != "" {
		out.Command = override.Command
	}
	if len(override.Args) > 0 {
		out.Args = append([]string{}, override.Args...)
	}
	if len(override.Hooks) > 0 {
		out.Hooks = append([]string{}, override.Hooks...)
	}
	if override.TimeoutMS > 0 {
		out.TimeoutMS = override.TimeoutMS
	}
	if override.FailFast != nil {
		out.FailFast = override.FailFast
	}
	if len(base.Config) == 0 && len(override.Config) == 0 {
		out.Config = nil
		return out
	}
	out.Config = map[string]any{}
	for k, v := range base.Config {
		out.Config[k] = v
	}
	for k, v := range override.Config {
		out.Config[k] = v
	}
	return out
}

func inferPluginCommand(p PluginConfig) PluginConfig {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return p
	}
	if len(p.Args) > 0 || strings.TrimSpace(p.Command) != "" {
		return p
	}
	p.Command = "go"
	p.Args = []string{"run", "./plugins/" + name}
	return p
}

func (m *PluginManager) EnabledPluginNames() []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.plugins))
	seen := map[string]struct{}{}
	for _, p := range m.plugins {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func ResolvePluginStaticPath(configDir, pluginName, rel string) string {
	name := strings.TrimSpace(pluginName)
	if name == "" {
		return ""
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return ""
	}

	cleanRel := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if cleanRel == "" || cleanRel == "." {
		return ""
	}

	root := filepath.Join(configDir, "plugins", name, "static")
	full := filepath.Join(root, filepath.FromSlash(cleanRel))
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return ""
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(os.PathSeparator)) {
		return ""
	}

	info, err := os.Stat(fullAbs)
	if err != nil || info.IsDir() {
		return ""
	}
	return fullAbs
}
