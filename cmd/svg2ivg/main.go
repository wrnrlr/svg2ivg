package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/wrnrlr/svg2ivg"
)

func main() {
	pattern := os.Args[1]
	dst := os.Args[2]
	if dst == "" {
		dst = "data"
	}
	pkgName := os.Args[3]
	if pkgName == "" {
		dst = "icons"
	}
	prefix := os.Args[4]
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Println("Failed to parse file glob")
		panic(err)
	}
	output, err := os.Open(dst)
	if err != nil {
		panic(err)
	}
	_, err = output.WriteString(fmt.Sprintf("package %s\n\n", pkgName))
	if err != nil {
		panic(err)
	}
	for _, match := range matches {
		fileName := strings.TrimRight(match, ".svg")
		f, err := os.Open(match)
		if err != nil {
			panic(err)
		}
		svg, err := svg2ivg.NewSVG(f)
		if err != nil {
			panic(err)
		}
		ivg, err := svg.IVG()
		if err != nil {
			panic(err)
		}
		varName := strcase.ToCamel(fmt.Sprintf("%s%s", prefix, fileName))
		_, err = output.WriteString(fmt.Sprintf("var %s = %#v\n\n", varName, ivg))
		if err != nil {
			panic(err)
		}
	}
}
