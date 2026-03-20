package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	core "folio/internal/folio"
)

func TestBuildCommandGeneratesSite(t *testing.T) {
	root := repoRoot(t)
	outDir := filepath.Join(root, "dist-test")
	_ = os.RemoveAll(outDir)
	t.Cleanup(func() { _ = os.RemoveAll(outDir) })

	cmd := exec.Command("go", "run", "./cmd/build", "-out", outDir, "-base-path", "/folio")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, string(out))
	}

	required := []string{
		filepath.Join(outDir, "index.html"),
		filepath.Join(outDir, "search-index.json"),
		filepath.Join(outDir, ".nojekyll"),
		filepath.Join(outDir, "archives", "index.html"),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file not found: %s", p)
		}
	}

	entries, err := os.ReadDir(filepath.Join(outDir, "static"))
	if err != nil {
		t.Fatalf("read static dir failed: %v", err)
	}
	foundFingerprinted := false
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "style.") && strings.HasSuffix(name, ".css") {
			foundFingerprinted = true
			break
		}
	}
	if !foundFingerprinted {
		t.Fatalf("fingerprinted style asset not found in dist static directory")
	}
}

func TestBuildStaticSiteDirect(t *testing.T) {
	root := repoRoot(t)
	withWorkdir(t, root)
	outDir := filepath.Join(root, "dist-test-direct")
	_ = os.RemoveAll(outDir)
	t.Cleanup(func() { _ = os.RemoveAll(outDir) })

	err := core.BuildStaticSite(core.BuildOptions{
		OutDir:     outDir,
		BasePath:   "/folio",
		ConfigPath: "config.json",
		PostsDir:   "posts",
	})
	if err != nil {
		t.Fatalf("BuildStaticSite failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Fatalf("expected output index.html missing: %v", err)
	}
}

func TestBuildCommandFailsOnInvalidConfig(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	badCfg := filepath.Join(tmp, "bad-config.json")
	if err := os.WriteFile(badCfg, []byte("{ invalid json"), 0o644); err != nil {
		t.Fatalf("write bad config failed: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/build", "-config", badCfg, "-out", filepath.Join(tmp, "dist"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected build to fail with invalid config, output=%s", string(out))
	}
	if !strings.Contains(strings.ToLower(string(out)), "invalid") {
		t.Fatalf("expected invalid json parse error, got: %s", string(out))
	}
}
