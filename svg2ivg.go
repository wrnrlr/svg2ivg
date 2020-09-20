// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a modification from https://gist.github.com/Inkeliz/728993bf10abfa6d8832302b2f2e87cd
// which is an modification from https://github.com/golang/exp/blob/00229845015e38294862ecd9909318241789d41c/shiny/materialdesign/icons/gen.go
// DON'T supports all types of SVG.

package svg2ivg

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"golang.org/x/exp/shiny/iconvg"
	"golang.org/x/image/math/f32"
)

type SVG struct {
	Width   float32 `xml:"width,attr"`
	Height  float32 `xml:"height,attr"`
	ViewBox string  `xml:"viewBox,attr"`
	Paths   []Path  `xml:"path"`
	// Some of the SVG files contain <circle> elements, not just <path>
	// elements. IconVG doesn't have circles per se. Instead, we convert such
	// circles to be paired arcTo commands, tacked on to the first path.
	//
	// In general, this isn't correct if the circles and the path overlap, but
	// that doesn't happen in the specific case of the Material Design icons.
	Circles []Circle `xml:"circle"`
}

// NewSVG reads the given reader as an SVG
func NewSVG(r io.Reader) (svg *SVG, err error) {
	bsvg, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	svg = new(SVG)
	if err := xml.Unmarshal(bsvg, svg); err != nil {
		return nil, err
	}

	return svg, nil
}

// NewSVGFile reads the .SVG file from the given path as an SVG.
func NewSVGFile(path string) (svg *SVG, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return NewSVG(file)
}

// IVG creates the IVG from active SVG
func (svg *SVG) IVG() (iconVG IVG, err error) {
	var enc iconvg.Encoder

	enc.Reset(iconvg.Metadata{
		ViewBox: iconvg.Rectangle{
			Min: f32.Vec2{-24, -24},
			Max: f32.Vec2{+24, +24},
		},
		Palette: iconvg.DefaultPalette,
	})

	var vbx, vby float32
	for i, v := range strings.Split(svg.ViewBox, " ") {
		f, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return nil, err
		}

		switch i {
		case 0:
			vbx = float32(f)
		case 1:
			vby = float32(f)
		}
	}
	offset := f32.Vec2{
		vbx * outSize / svg.Height,
		vby * outSize / svg.Width,
	}

	// adjs maps from opacity to a cReg adj value.
	adjs := map[float32]uint8{}

	for _, p := range svg.Paths {
		if err := genPath(&enc, &p, adjs, svg.Width, offset, svg.Circles); err != nil {
			return nil, err
		}
		svg.Circles = nil
	}

	if len(svg.Circles) != 0 {
		if err := genPath(&enc, &Path{}, adjs, svg.Width, offset, svg.Circles); err != nil {
			return nil, err
		}
		svg.Circles = nil
	}

	ivgData, err := enc.Bytes()
	if err != nil {
		return nil, err
	}

	return ivgData, nil
}

// IVG is the IconVG
type IVG []byte

// NewIVG creates the IVG of the given SVG
func NewIVG(svg *SVG) (iconVG []byte, err error) {
	return svg.IVG()
}

type Path struct {
	D           string   `xml:"d,attr"`
	Fill        string   `xml:"fill,attr"`
	FillOpacity *float32 `xml:"fill-opacity,attr"`
	Opacity     *float32 `xml:"opacity,attr"`
}

type Circle struct {
	Cx float32 `xml:"cx,attr"`
	Cy float32 `xml:"cy,attr"`
	R  float32 `xml:"r,attr"`
}

// outSize is the width and height (in ideal vector space) of the generated
// IconVG graphic, regardless of the size of the input SVG.
const outSize = 48

func genPath(enc *iconvg.Encoder, p *Path, adjs map[float32]uint8, size float32, offset f32.Vec2, circles []Circle) error {
	// Modified (Inkeliz) if Fill is none so ignore it
	if p.Fill == "none" {
		return nil
	}

	adj := uint8(0)
	opacity := float32(1)
	if p.Opacity != nil {
		opacity = *p.Opacity
	} else if p.FillOpacity != nil {
		opacity = *p.FillOpacity
	}
	if opacity != 1 {
		var ok bool
		if adj, ok = adjs[opacity]; !ok {
			adj = uint8(len(adjs) + 1)
			adjs[opacity] = adj
			// Set CREG[0-adj] to be a blend of transparent (0x7f) and the
			// first custom palette color (0x80).
			enc.SetCReg(adj, false, iconvg.BlendColor(uint8(opacity*0xff), 0x7f, 0x80))
		}
	}

	needStartPath := true
	if p.D != "" {
		needStartPath = false
		if err := genPathData(enc, adj, p.D, size, offset); err != nil {
			return err
		}
	}

	for _, c := range circles {
		// Normalize.
		cx := c.Cx * outSize / size
		cx -= outSize/2 + offset[0]
		cy := c.Cy * outSize / size
		cy -= outSize/2 + offset[1]
		r := c.R * outSize / size

		if needStartPath {
			needStartPath = false
			enc.StartPath(adj, cx-r, cy)
		} else {
			enc.ClosePathAbsMoveTo(cx-r, cy)
		}

		// Convert a circle to two relative arcTo ops, each of 180 degrees.
		// We can't use one 360 degree arcTo as the start and end point
		// would be coincident and the computation is degenerate.
		enc.RelArcTo(r, r, 0, false, true, +2*r, 0)
		enc.RelArcTo(r, r, 0, false, true, -2*r, 0)
	}

	enc.ClosePathEndPath()
	return nil
}

func genPathData(enc *iconvg.Encoder, adj uint8, pathData string, size float32, offset f32.Vec2) error {
	if strings.HasSuffix(pathData, "z") {
		pathData = pathData[:len(pathData)-1]
	}
	r := strings.NewReader(pathData)

	var args [6]float32
	op, relative, started := byte(0), false, false
	for {
		b, err := r.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch {
		case b == ' ':
			continue
		case 'A' <= b && b <= 'Z':
			op, relative = b, false
		case 'a' <= b && b <= 'z':
			op, relative = b, true
		default:
			r.UnreadByte()
		}

		n := 0
		switch op {
		case 'L', 'l', 'T', 't':
			n = 2
		case 'Q', 'q', 'S', 's':
			n = 4
		case 'C', 'c':
			n = 6
		case 'H', 'h', 'V', 'v':
			n = 1
		case 'M', 'm':
			n = 2
		case 'Z', 'z':
		default:
			return fmt.Errorf("unknown opcode %c\n", b)
		}

		scan(&args, r, n)
		normalize(&args, n, op, size, offset, relative)

		switch op {
		case 'L':
			enc.AbsLineTo(args[0], args[1])
		case 'l':
			enc.RelLineTo(args[0], args[1])
		case 'T':
			enc.AbsSmoothQuadTo(args[0], args[1])
		case 't':
			enc.RelSmoothQuadTo(args[0], args[1])
		case 'Q':
			enc.AbsQuadTo(args[0], args[1], args[2], args[3])
		case 'q':
			enc.RelQuadTo(args[0], args[1], args[2], args[3])
		case 'S':
			enc.AbsSmoothCubeTo(args[0], args[1], args[2], args[3])
		case 's':
			enc.RelSmoothCubeTo(args[0], args[1], args[2], args[3])
		case 'C':
			enc.AbsCubeTo(args[0], args[1], args[2], args[3], args[4], args[5])
		case 'c':
			enc.RelCubeTo(args[0], args[1], args[2], args[3], args[4], args[5])
		case 'H':
			enc.AbsHLineTo(args[0])
		case 'h':
			enc.RelHLineTo(args[0])
		case 'V':
			enc.AbsVLineTo(args[0])
		case 'v':
			enc.RelVLineTo(args[0])
		case 'M':
			if !started {
				started = true
				enc.StartPath(adj, args[0], args[1])
			} else {
				enc.ClosePathAbsMoveTo(args[0], args[1])
			}
		case 'm':
			enc.ClosePathRelMoveTo(args[0], args[1])
		}
	}
	return nil
}

func scan(args *[6]float32, r *strings.Reader, n int) {
	for i := 0; i < n; i++ {
		for {
			if b, _ := r.ReadByte(); b != ' ' {
				r.UnreadByte()
				break
			}
		}
		fmt.Fscanf(r, "%f", &args[i])
	}
}

func atof(s []byte) (float32, error) {
	f, err := strconv.ParseFloat(string(s), 32)
	if err != nil {
		return 0, fmt.Errorf("could not parse %q as a float32: %v", s, err)
	}
	return float32(f), err
}

func normalize(args *[6]float32, n int, op byte, size float32, offset f32.Vec2, relative bool) {
	for i := 0; i < n; i++ {
		args[i] *= outSize / size
		if relative {
			continue
		}
		args[i] -= outSize / 2
		switch {
		case n != 1:
			args[i] -= offset[i&0x01]
		case op == 'H':
			args[i] -= offset[0]
		case op == 'V':
			args[i] -= offset[1]
		}
	}
}
