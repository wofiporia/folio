package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type request struct {
	Hook   string         `json:"hook"`
	Build  buildInfo      `json:"build"`
	Config map[string]any `json:"plugin_config"`
}

type buildInfo struct {
	Mode     string `json:"mode"`
	OutDir   string `json:"out_dir"`
	BasePath string `json:"base_path"`
}

type response struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type privatePost struct {
	Slug  string
	Title string
}

type encryptedPayload struct {
	Version    int          `json:"version"`
	Algorithm  string       `json:"algorithm"`
	KDF        kdfParams    `json:"kdf"`
	IV         string       `json:"iv"`
	Ciphertext string       `json:"ciphertext"`
	Meta       payloadMeta  `json:"meta,omitempty"`
}

type kdfParams struct {
	Name       string `json:"name"`
	Hash       string `json:"hash"`
	Iterations int    `json:"iterations"`
	Salt       string `json:"salt"`
}

type payloadMeta struct {
	Title string `json:"title,omitempty"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		writeResp(response{Error: fmt.Sprintf("decode request failed: %v", err)})
		os.Exit(1)
	}
	if req.Hook != "after_static_build" {
		writeResp(response{Message: "hook skipped"})
		return
	}

	outDir := strings.TrimSpace(req.Build.OutDir)
	if outDir == "" {
		writeResp(response{Error: "missing build.out_dir for private_posts"})
		os.Exit(1)
	}

	postsDir := configString(req.Config, "posts_dir", "posts")
	iterations := configInt(req.Config, "kdf_iterations", 210000)
	if iterations < 50000 {
		iterations = 50000
	}

	privatePosts, err := findPrivatePosts(postsDir)
	if err != nil {
		writeResp(response{Error: fmt.Sprintf("scan private posts failed: %v", err)})
		os.Exit(1)
	}
	if len(privatePosts) == 0 {
		writeResp(response{Message: "private_posts: no private articles found"})
		return
	}

	password := configString(req.Config, "password", "")
	if password == "" {
		writeResp(response{Error: "private_posts requires plugin config password"})
		os.Exit(1)
	}

	count := 0
	for _, p := range privatePosts {
		postPath := filepath.Join(outDir, "post", p.Slug, "index.html")
		plainHTML, err := os.ReadFile(postPath)
		if err != nil {
			continue
		}

		payload, err := encryptHTML(plainHTML, password, iterations, p)
		if err != nil {
			writeResp(response{Error: fmt.Sprintf("encrypt post %q failed: %v", p.Slug, err)})
			os.Exit(1)
		}

		if err := os.WriteFile(filepath.Join(filepath.Dir(postPath), "payload.json"), mustJSON(payload), 0o644); err != nil {
			writeResp(response{Error: fmt.Sprintf("write payload for %q failed: %v", p.Slug, err)})
			os.Exit(1)
		}

		lockedHTML := buildLockedPage(req.Build.BasePath, p)
		if err := os.WriteFile(postPath, []byte(lockedHTML), 0o644); err != nil {
			writeResp(response{Error: fmt.Sprintf("write locked page for %q failed: %v", p.Slug, err)})
			os.Exit(1)
		}

		count++
	}

	writeResp(response{Message: fmt.Sprintf("private_posts: encrypted %d article(s)", count)})
}

func findPrivatePosts(postsDir string) ([]privatePost, error) {
	matches, err := filepath.Glob(filepath.Join(postsDir, "*.md"))
	if err != nil {
		return nil, err
	}
	out := make([]privatePost, 0)
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		meta := parseFrontMatter(string(b))
		if !parseBool(meta["private"]) {
			continue
		}
		slug := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		title := strings.TrimSpace(meta["title"])
		if title == "" {
			title = slug
		}
		out = append(out, privatePost{Slug: slug, Title: title})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func parseFrontMatter(content string) map[string]string {
	content = strings.TrimPrefix(content, "\uFEFF")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	meta := map[string]string{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return meta
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		meta[strings.ToLower(strings.TrimSpace(k))] = strings.Trim(strings.TrimSpace(v), "\"")
	}
	return meta
}

func parseBool(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func encryptHTML(plain []byte, password string, iterations int, p privatePost) (encryptedPayload, error) {
	salt := make([]byte, 16)
	iv := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return encryptedPayload{}, err
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return encryptedPayload{}, err
	}
	key := pbkdf2SHA256([]byte(password), salt, iterations, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return encryptedPayload{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedPayload{}, err
	}
	ct := aead.Seal(nil, iv, plain, nil)

	return encryptedPayload{
		Version:   1,
		Algorithm: "AES-GCM-256",
		KDF: kdfParams{
			Name:       "PBKDF2",
			Hash:       "SHA-256",
			Iterations: iterations,
			Salt:       base64.StdEncoding.EncodeToString(salt),
		},
		IV:         base64.StdEncoding.EncodeToString(iv),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
		Meta: payloadMeta{
			Title: p.Title,
		},
	}, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hLen := 32
	numBlocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, numBlocks*hLen)
	for block := 1; block <= numBlocks; block++ {
		u := pbkdf2Block(password, salt, iterations, block)
		out = append(out, u...)
	}
	return out[:keyLen]
}

func pbkdf2Block(password, salt []byte, iterations, blockNum int) []byte {
	mac := hmac.New(sha256.New, password)
	buf := make([]byte, len(salt)+4)
	copy(buf, salt)
	binary.BigEndian.PutUint32(buf[len(salt):], uint32(blockNum))
	_, _ = mac.Write(buf)
	u := mac.Sum(nil)
	res := make([]byte, len(u))
	copy(res, u)
	for i := 1; i < iterations; i++ {
		mac = hmac.New(sha256.New, password)
		_, _ = mac.Write(u)
		u = mac.Sum(nil)
		for j := range res {
			res[j] ^= u[j]
		}
	}
	return res
}

func buildLockedPage(basePath string, p privatePost) string {
	title := htmlEscape(p.Title)
	homeURL := withBase(basePath, "/")
	styleURL := withBase(basePath, "/static/style.css")
	faviconURL := withBase(basePath, "/static/favicon.png")
	return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex,nofollow">
  <title>私密文章 · ` + title + `</title>
  <link rel="icon" type="image/png" href="` + faviconURL + `">
  <link rel="stylesheet" href="` + styleURL + `">
  <style>
    body { min-height:100vh; }
    .lock-wrap { min-height:100vh; display:grid; place-items:center; padding:24px; }
    .card {
      width:min(92vw, 500px);
      background:var(--paper, #fff);
      color:var(--ink, #111);
      border:1px solid var(--line, rgba(0,0,0,.2));
      border-radius:14px;
      padding:22px;
      box-shadow:0 18px 40px rgba(0,0,0,.18);
    }
    h1 { font-size:20px; margin:0 0 8px; }
    p { margin:8px 0; color:var(--muted, #556); }
    .row { display:flex; gap:8px; margin-top:14px; }
    input {
      flex:1;
      padding:10px 12px;
      border-radius:10px;
      border:1px solid var(--line, rgba(0,0,0,.2));
      background:var(--bg, #fff);
      color:var(--ink, #111);
    }
    button {
      padding:10px 14px;
      border-radius:10px;
      border:1px solid var(--line, rgba(0,0,0,.2));
      background:var(--paper, #fff);
      color:var(--ink, #111);
      cursor:pointer;
    }
    button:hover { filter:brightness(.98); }
    .err { min-height:20px; margin-top:10px; color:#ff7f7f; }
    .muted { margin-top:14px; font-size:12px; }
    a { color:var(--ink, #111); }
  </style>
</head>
<body>
  <div class="lock-wrap">
    <main class="card">
      <h1>这是一篇私密文章</h1>
      <p>请输入密码</p>
      <form id="unlock-form" class="row">
        <input id="password" type="password" placeholder="输入访问密码" autocomplete="current-password" required>
        <button type="submit">解锁</button>
      </form>
      <div id="err" class="err"></div>
      <p class="muted"><a href="` + homeURL + `">返回首页</a></p>
    </main>
  </div>
  <script>
    (function(){
      const form = document.getElementById('unlock-form');
      const passInput = document.getElementById('password');
      const err = document.getElementById('err');

      function b64ToBytes(b64){
        const bin = atob(b64);
        const out = new Uint8Array(bin.length);
        for(let i=0;i<bin.length;i++){ out[i]=bin.charCodeAt(i); }
        return out;
      }

      async function deriveKey(password, salt, iterations){
        const enc = new TextEncoder();
        const keyMat = await crypto.subtle.importKey('raw', enc.encode(password), 'PBKDF2', false, ['deriveKey']);
        return crypto.subtle.deriveKey(
          { name:'PBKDF2', hash:'SHA-256', salt:salt, iterations:iterations },
          keyMat,
          { name:'AES-GCM', length:256 },
          false,
          ['decrypt']
        );
      }

      async function unlock(password){
        const resp = await fetch('./payload.json', { cache:'no-store' });
        if(!resp.ok){ throw new Error('未找到加密内容'); }
        const payload = await resp.json();
        const salt = b64ToBytes(payload.kdf.salt);
        const iv = b64ToBytes(payload.iv);
        const ct = b64ToBytes(payload.ciphertext);
        const key = await deriveKey(password, salt, Number(payload.kdf.iterations||210000));
        const plainBuf = await crypto.subtle.decrypt({ name:'AES-GCM', iv:iv }, key, ct);
        const html = new TextDecoder().decode(new Uint8Array(plainBuf));
        document.open();
        document.write(html);
        document.close();
      }

      form.addEventListener('submit', async function(e){
        e.preventDefault();
        err.textContent = '';
        const pw = passInput.value;
        if(!pw){ return; }
        try {
          await unlock(pw);
        } catch(_e){
          err.textContent = '密码错误或内容已损坏';
        }
      });
    })();
  </script>
</body>
</html>`
}

func pruneSearchIndex(path string, blocked map[string]struct{}) error {
	if len(blocked) == 0 {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var docs []map[string]any
	if err := json.Unmarshal(b, &docs); err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(docs))
	for _, d := range docs {
		slug, _ := d["slug"].(string)
		if _, deny := blocked[slug]; deny {
			continue
		}
		out = append(out, d)
	}
	clean, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	clean = append(clean, '\n')
	return os.WriteFile(path, clean, 0o644)
}

func pruneListingPages(outDir string, blocked map[string]struct{}) error {
	paths := []string{}
	paths = append(paths, filepath.Join(outDir, "index.html"))
	if ms, _ := filepath.Glob(filepath.Join(outDir, "page", "*", "index.html")); len(ms) > 0 {
		paths = append(paths, ms...)
	}
	paths = append(paths, collectIndexFiles(filepath.Join(outDir, "tags"))...)
	paths = append(paths, collectIndexFiles(filepath.Join(outDir, "archives"))...)
	for _, p := range paths {
		_ = pruneListingFile(p, blocked)
	}
	return nil
}

func collectIndexFiles(root string) []string {
	out := []string{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "index.html") {
			out = append(out, path)
		}
		return nil
	})
	return out
}

func pruneListingFile(path string, blocked map[string]struct{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(b)
	orig := s
	for slug := range blocked {
		slugQ := regexp.QuoteMeta(slug)
		// Remove post cards in index/tags layouts.
		reCard := regexp.MustCompile(`(?s)<a class=\"post-card-link\" href=\"[^\"]*/post/` + slugQ + `/?[^\"]*\">.*?</a>`)
		s = reCard.ReplaceAllString(s, "")
		// Remove archive list entries.
		reArchiveLi := regexp.MustCompile(`(?s)<li>\s*<span class=\"meta\">.*?</span>\s*<a href=\"[^\"]*/post/` + slugQ + `/?[^\"]*\">.*?</a>\s*</li>`)
		s = reArchiveLi.ReplaceAllString(s, "")
	}
	if s == orig {
		return nil
	}
	return os.WriteFile(path, []byte(s), 0o644)
}

func configString(cfg map[string]any, key, fallback string) string {
	if cfg == nil {
		return fallback
	}
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}

func configInt(cfg map[string]any, key string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err == nil {
			return n
		}
	}
	return fallback
}

func withBase(basePath, p string) string {
	basePath = strings.Trim(basePath, "/")
	if basePath == "" {
		return p
	}
	if strings.HasPrefix(p, "/") {
		return "/" + basePath + p
	}
	return "/" + basePath + "/" + p
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func mustJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(b, '\n')
}

func writeResp(resp response) {
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
