# SVG2IVG

Simple tool to convert svg files to [ivg](https://godoc.org/golang.org/x/exp/shiny/iconvg) embedded in a go file.
This can be used with `go:generate`.
Generated variable names will be formatted using PrefixedCamelCase.

## Install

```
go install github.com/wrnrlr/svg2ivg/cmd/svg2ivg
```

### Usage

Add module as dependency

```
go get -t github.com/wrnrlr/svg2ivg/cmd/svg2ivg
``` 

```
svg2ivg path/to/*.svg output.go package (prefix)
```

* `prefix` is optional

### Example

Generate two files, `solid.go` and `regular.go`, in `fontawesome` package.

```
//go:generate svg2ivg fontawesome/solid/*.svg solid.go fontawesome Solid
//go:generate svg2ivg fontawesome/regular/*.svg regular.go fontawesome Regular
```
