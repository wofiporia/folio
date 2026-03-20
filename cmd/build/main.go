package main

import (
	"flag"
	"log"

	core "folio/internal/folio"
)

func main() {
	outDir := flag.String("out", "dist", "output directory")
	basePath := flag.String("base-path", "", "base path prefix, e.g. /repo")
	configPath := flag.String("config", "config.json", "config file path")
	siteURL := flag.String("site-url", "", "absolute site url, e.g. https://example.com")
	flag.Parse()

	if err := core.BuildStaticSite(core.BuildOptions{
		OutDir:     *outDir,
		BasePath:   *basePath,
		ConfigPath: *configPath,
		SiteURL:    *siteURL,
		PostsDir:   "posts",
	}); err != nil {
		log.Fatal(err)
	}
}
