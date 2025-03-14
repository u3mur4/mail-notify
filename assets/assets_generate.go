package main

import (
	"log"
	"net/http"
	"os"

	"github.com/shurcooL/httpfs/filter"
	"github.com/shurcooL/vfsgen"
)

func main() {
	// Assets contains project assets.
	var FS = filter.Keep(
		http.Dir("assets"),
		func(path string, fi os.FileInfo) bool {
			return fi.Name() != "assets_generate.go"
		},
	)

	err := vfsgen.Generate(FS, vfsgen.Options{
		PackageName:  "main",
		BuildTags:    "!dev",
		VariableName: "Assets",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
