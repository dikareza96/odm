// Command extpack builds the Chromium and Firefox packages of the ODM
// browser extension (used by the release workflow).
package main

import (
	"flag"
	"fmt"
	"os"

	"odm/internal/extpack"
)

func main() {
	src := flag.String("src", "extension", "extension source folder")
	out := flag.String("out", "dist/extension", "output folder")
	tag := flag.String("version", "", `release tag or version for the manifest, e.g. "v0.2.0" (blank keeps the source version)`)
	flag.Parse()

	version := ""
	if *tag != "" {
		v, err := extpack.ManifestVersion(*tag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "extpack:", err)
			os.Exit(2)
		}
		version = v
	}
	archives, err := extpack.Build(*src, *out, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extpack:", err)
		os.Exit(1)
	}
	for _, t := range []string{extpack.Chromium, extpack.Firefox} {
		fmt.Println(archives[t])
	}
}
