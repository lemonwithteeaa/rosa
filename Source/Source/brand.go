//go:build windows

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// The approved icon contains the complete РОСА lettering. The header must use
// this single source instead of joining a reduced Р icon with system-font text.
//
//go:embed ROSA_app_icon_transparent.png
var embeddedFullBrandPNG []byte

// The complete Cyrillic LUKOIL wordmark is a single network-sourced asset. It
// is never reconstructed from a system font or separate emblem.
//
//go:embed LUKOIL_logo_cyr_network.png
var embeddedLukoilHeaderPNG []byte

// Original RITEK vertical brand lockup used in the lower-right corner of the login screen.
// The emblem and the proprietary RITEK wordmark are kept as one raster source so the
// branded lettering is never approximated with a system font.
//
//go:embed RITEK_footer_label.png
var embeddedRitekFooterPNG []byte

// User-approved full-width oilfield background for the login screen.
// The source already contains the subtle RITEK mark in the lower-right corner,
// therefore no separate footer RITEK lockup is drawn on top of it.
//
//go:embed login_background_oilfield_v1.png
var embeddedLoginBackgroundPNG []byte

// High-quality login artwork prepared from the approved blue/red visual language.
// These are decorative only; all text stays native Windows controls.
//
//go:embed login_theme_strip_v2.png
var embeddedLoginThemeStripPNG []byte

//go:embed login_side_left_v1.png
var embeddedLoginSideLeftPNG []byte

//go:embed login_side_right_v1.png
var embeddedLoginSideRightPNG []byte

// Sharp standalone login icons used by the native RC386 login layout.
//
//go:embed login_feature_1.png
var embeddedLoginFeature1PNG []byte

//go:embed login_feature_2.png
var embeddedLoginFeature2PNG []byte

//go:embed login_feature_3.png
var embeddedLoginFeature3PNG []byte

//go:embed login_feature_4.png
var embeddedLoginFeature4PNG []byte

//go:embed login_nav_1.png
var embeddedLoginNav1PNG []byte

//go:embed login_nav_2.png
var embeddedLoginNav2PNG []byte

//go:embed login_nav_3.png
var embeddedLoginNav3PNG []byte

//go:embed login_nav_4.png
var embeddedLoginNav4PNG []byte

// Exact static card artwork derived from the user-approved RC382 reference.
// Interactive controls are overlaid natively on the left card; the right card is fully static.
//
//go:embed login_left_reference_v2.png
var embeddedLoginLeftReferencePNG []byte

//go:embed login_right_reference_v2.png
var embeddedLoginRightReferencePNG []byte

// Realistic pumpjack artwork approved for the startup/loading panel.
// It is embedded so the splash remains self-contained and does not create
// loose image files beside the executable.
//
//go:embed startup_pumpjack_v1.png
var embeddedStartupPumpjackPNG []byte

func buildStartupPumpjackBitmap(width, height int) (syscall.Handle, error) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	src, err := png.Decode(bytes.NewReader(embeddedStartupPumpjackPNG))
	if err != nil {
		return 0, err
	}
	out := resizeContainOnBackgroundBilinear(src, width, height, color.RGBA{R: 18, G: 53, B: 84, A: 255})

	// RC560: create a top-down 32-bit DIB directly from the embedded PNG. The old
	// cache-file + LoadImage path could fail on a user's machine and silently drop
	// to the crude vector fallback, which is exactly what the screenshots showed.
	dc, _, _ := pCreateCompatibleDC.Call(0)
	if dc == 0 {
		return 0, fmt.Errorf("не удалось создать DC для заставки")
	}
	defer pDeleteDC.Call(dc)
	bmi := laboratoryBitmapInfo{}
	bmi.Header.Size = uint32(unsafe.Sizeof(laboratoryBitmapInfoHeader{}))
	bmi.Header.Width = int32(width)
	bmi.Header.Height = -int32(height)
	bmi.Header.Planes = 1
	bmi.Header.BitCount = 32
	bmi.Header.Compression = BI_RGB_STIFF
	bmi.Header.SizeImage = uint32(width * height * 4)
	var bits unsafe.Pointer
	bmp, _, _ := pCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS_STIFF, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return 0, fmt.Errorf("не удалось создать bitmap заставки")
	}
	raw := unsafe.Slice((*byte)(bits), width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := out.At(x, y).RGBA()
			i := (y*width + x) * 4
			raw[i+0] = byte(b >> 8)
			raw[i+1] = byte(g >> 8)
			raw[i+2] = byte(r >> 8)
			raw[i+3] = 0xFF
		}
	}
	return syscall.Handle(bmp), nil
}

func uiAssetCacheDir() string {
	dir := filepath.Join(appRootDir(), "cache", "ui")
	_ = os.MkdirAll(dir, 0700)
	return dir
}

func buildLoginReferenceCardBitmap(left bool) (syscall.Handle, error) {
	var raw []byte
	var name string
	var width, height int
	if left {
		raw = embeddedLoginLeftReferencePNG
		name = "_login_left_reference_v4.bmp"
		width, height = 604, 732
	} else {
		raw = embeddedLoginRightReferencePNG
		name = "_login_right_reference_v4.bmp"
		width, height = 613, 732
	}
	path := filepath.Join(uiAssetCacheDir(), name)
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return 0, err
		}
		out := resizeContainOnBackground(src, width, height, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, uintptr(width), uintptr(height), LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func buildLoginBackgroundBitmap(width, height int) (syscall.Handle, error) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	path := filepath.Join(uiAssetCacheDir(), fmt.Sprintf("_login_background_oilfield_v5_%dx%d.bmp", width, height))
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(embeddedLoginBackgroundPNG))
		if err != nil {
			return 0, err
		}
		bb := src.Bounds()
		sw, sh := bb.Dx(), bb.Dy()
		// The approved background asset is already the exact native cache size
		// (1672×941). The former nearest-neighbour loop visited every pixel and
		// converted it through the generic image interface on every cold start.
		// A direct draw preserves the same pixels while removing that startup cost.
		if sw == width && sh == height {
			out := image.NewRGBA(image.Rect(0, 0, width, height))
			draw.Draw(out, out.Bounds(), src, bb.Min, draw.Src)
			if err = writeBMP24(out, path); err != nil {
				return 0, err
			}
			h, _, loadErr := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, uintptr(width), uintptr(height), LR_LOADFROMFILE)
			return syscall.Handle(h), loadErr
		}
		cropX, cropY, cropW, cropH := 0, 0, sw, sh
		// Aspect-preserving cover crop. Vertical cropping is anchored to the bottom
		// so the RITEK mark in the supplied source remains fully visible.
		if width*sh >= height*sw {
			cropH = maxInt(1, height*sw/width)
			cropY = sh - cropH
		} else {
			cropW = maxInt(1, width*sh/height)
			cropX = sw - cropW
		}
		out := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			sy := bb.Min.Y + cropY + y*cropH/height
			if sy >= bb.Max.Y {
				sy = bb.Max.Y - 1
			}
			for x := 0; x < width; x++ {
				sx := bb.Min.X + cropX + x*cropW/width
				if sx >= bb.Max.X {
					sx = bb.Max.X - 1
				}
				out.Set(x, y, src.At(sx, sy))
			}
		}
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, uintptr(width), uintptr(height), LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func buildHeaderBrandBitmap() (syscall.Handle, error) {
	path := filepath.Join(uiAssetCacheDir(), "_rosa_header_brand_v10.bmp")
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(embeddedFullBrandPNG))
		if err != nil {
			return 0, err
		}
		b := src.Bounds()
		// The upper-middle part of the application icon is the complete logo;
		// the derricks and decorative frame are intentionally excluded.
		cropRect := image.Rect(
			b.Min.X+b.Dx()*8/100,
			b.Min.Y+b.Dy()*26/100,
			b.Min.X+b.Dx()*92/100,
			b.Min.Y+b.Dy()*70/100,
		)
		logo := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
		draw.Draw(logo, logo.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		inkMinX, inkMinY := logo.Bounds().Max.X, logo.Bounds().Max.Y
		inkMaxX, inkMaxY := -1, -1
		for y := cropRect.Min.Y; y < cropRect.Max.Y; y++ {
			for x := cropRect.Min.X; x < cropRect.Max.X; x++ {
				r, g, bl, _ := src.At(x, y).RGBA()
				r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(bl>>8)
				maxC := max3(r8, g8, b8)
				minC := min3(r8, g8, b8)
				// Keep the saturated red/blue corporate artwork and replace the
				// icon's pale decorative background with the header's white.
				lx, ly := x-cropRect.Min.X, y-cropRect.Min.Y
				// The source illustration also contains a separate blue derrick
				// below the letter A. It is not part of the РОСА wordmark.
				decorativeBelowA := lx > logo.Bounds().Dx()*62/100 && ly > logo.Bounds().Dy()*83/100
				if int(maxC)-int(minC) >= 38 && minC < 220 && !decorativeBelowA {
					logo.SetRGBA(lx, ly, color.RGBA{R: r8, G: g8, B: b8, A: 255})
					inkMinX, inkMinY = minInt(inkMinX, lx), minInt(inkMinY, ly)
					inkMaxX, inkMaxY = maxInt(inkMaxX, lx), maxInt(inkMaxY, ly)
				}
			}
		}
		if inkMaxX >= inkMinX && inkMaxY >= inkMinY {
			padding := 2
			inkRect := image.Rect(
				maxInt(0, inkMinX-padding),
				maxInt(0, inkMinY-padding),
				minInt(logo.Bounds().Max.X, inkMaxX+padding+1),
				minInt(logo.Bounds().Max.Y, inkMaxY+padding+1),
			)
			tight := image.NewRGBA(image.Rect(0, 0, inkRect.Dx(), inkRect.Dy()))
			draw.Draw(tight, tight.Bounds(), logo, inkRect.Min, draw.Src)
			logo = tight
		}
		out := image.NewRGBA(image.Rect(0, 0, 174, 68))
		draw.Draw(out, out.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		scale := minFloat(float64(out.Bounds().Dx()-4)/float64(logo.Bounds().Dx()), float64(out.Bounds().Dy()-4)/float64(logo.Bounds().Dy()))
		dw, dh := maxInt(1, int(float64(logo.Bounds().Dx())*scale)), maxInt(1, int(float64(logo.Bounds().Dy())*scale))
		ox, oy := (out.Bounds().Dx()-dw)/2, (out.Bounds().Dy()-dh)/2
		for y := 0; y < dh; y++ {
			sy := minInt(logo.Bounds().Dy()-1, y*logo.Bounds().Dy()/dh)
			for x := 0; x < dw; x++ {
				sx := minInt(logo.Bounds().Dx()-1, x*logo.Bounds().Dx()/dw)
				out.Set(ox+x, oy+y, logo.At(sx, sy))
			}
		}
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 174, 68, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func buildHeaderLukoilBitmap() (syscall.Handle, error) {
	path := filepath.Join(uiAssetCacheDir(), "_lukoil_header_brand_v11.bmp")
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(embeddedLukoilHeaderPNG))
		if err != nil {
			return 0, err
		}
		// The network PNG contains a sizeable white margin. Crop only that margin
		// before proportional scaling so the complete emblem and wordmark fill the
		// approved compact header block instead of looking undersized inside it.
		b := src.Bounds()
		minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := src.At(x, y).RGBA()
				if a>>8 > 8 && min3(uint8(r>>8), uint8(g>>8), uint8(bl>>8)) < 242 {
					minX, minY = minInt(minX, x), minInt(minY, y)
					maxX, maxY = maxInt(maxX, x), maxInt(maxY, y)
				}
			}
		}
		logo := image.Image(src)
		if maxX >= minX && maxY >= minY {
			padX, padY := maxInt(2, (maxX-minX+1)/100), maxInt(2, (maxY-minY+1)/80)
			crop := image.Rect(maxInt(b.Min.X, minX-padX), maxInt(b.Min.Y, minY-padY), minInt(b.Max.X, maxX+padX+1), minInt(b.Max.Y, maxY+padY+1))
			tight := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
			draw.Draw(tight, tight.Bounds(), src, crop.Min, draw.Src)
			logo = tight
		}

		// Adjust the internal balance requested for the header while still
		// producing one complete bitmap from the official source: the square
		// emblem is larger, and the Cyrillic wordmark is smaller with a protected
		// right margin so its final letter can never be clipped.
		redMinX, redMinY, redMaxX, redMaxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
		blackMinX, blackMinY, blackMaxX, blackMaxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := src.At(x, y).RGBA()
				r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)
				if a8 > 8 && r8 > 150 && g8 < 120 && b8 < 140 {
					redMinX, redMinY = minInt(redMinX, x), minInt(redMinY, y)
					redMaxX, redMaxY = maxInt(redMaxX, x), maxInt(redMaxY, y)
				}
				if a8 > 8 && max3(r8, g8, b8) < 120 {
					blackMinX, blackMinY = minInt(blackMinX, x), minInt(blackMinY, y)
					blackMaxX, blackMaxY = maxInt(blackMaxX, x), maxInt(blackMaxY, y)
				}
			}
		}
		balanced := image.NewRGBA(image.Rect(0, 0, 156, 52))
		draw.Draw(balanced, balanced.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		if redMaxX >= redMinX && blackMaxX >= blackMinX {
			redRect := image.Rect(maxInt(b.Min.X, redMinX-2), maxInt(b.Min.Y, redMinY-2), minInt(b.Max.X, redMaxX+3), minInt(b.Max.Y, redMaxY+3))
			blackRect := image.Rect(maxInt(b.Min.X, blackMinX-2), maxInt(b.Min.Y, blackMinY-2), minInt(b.Max.X, blackMaxX+3), minInt(b.Max.Y, blackMaxY+3))
			drawScaledNearest(balanced, image.Rect(3, 3, 49, 49), src, redRect)
			drawScaledNearest(balanced, image.Rect(57, 14, 151, 38), src, blackRect)
			logo = balanced
		}
		headerLogo := resizeContain(logo, 156, 52)
		if exact, ok := logo.(*image.RGBA); ok && exact.Bounds().Dx() == 156 && exact.Bounds().Dy() == 52 {
			headerLogo = exact
		}
		if err = writeBMP24(headerLogo, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 156, 52, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func buildFooterRitekBitmap() (syscall.Handle, error) {
	path := filepath.Join(uiAssetCacheDir(), "_ritek_footer_label_v3.bmp")
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(embeddedRitekFooterPNG))
		if err != nil {
			return 0, err
		}
		// The source label has a white raster background. Replace only near-white
		// pixels by the footer colour before resizing so the mark visually floats
		// on the footer instead of sitting in a white rectangle.
		b := src.Bounds()
		clean := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		panel := color.RGBA{R: 238, G: 243, B: 248, A: 255}
		draw.Draw(clean, clean.Bounds(), &image.Uniform{C: panel}, image.Point{}, draw.Src)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := src.At(x, y).RGBA()
				r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)
				if a8 < 8 || (r8 > 242 && g8 > 242 && b8 > 242) {
					continue
				}
				clean.SetRGBA(x-b.Min.X, y-b.Min.Y, color.RGBA{R: r8, G: g8, B: b8, A: 255})
			}
		}
		out := resizeContainOnBackground(clean, 150, 136, panel)
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 150, 136, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func resizeContainOnBackgroundBilinear(src image.Image, targetW, targetH int, bg color.RGBA) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, maxInt(1, targetW), maxInt(1, targetH)))
	draw.Draw(out, out.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return out
	}
	pad := 2
	usableW, usableH := maxInt(1, targetW-2*pad), maxInt(1, targetH-2*pad)
	scale := minFloat(float64(usableW)/float64(b.Dx()), float64(usableH)/float64(b.Dy()))
	dw, dh := maxInt(1, int(float64(b.Dx())*scale)), maxInt(1, int(float64(b.Dy())*scale))
	ox, oy := (targetW-dw)/2, (targetH-dh)/2
	for y := 0; y < dh; y++ {
		fy := (float64(y)+0.5)*float64(b.Dy())/float64(dh) - 0.5
		y0 := int(fy)
		wy := fy - float64(y0)
		if y0 < 0 {
			y0, wy = 0, 0
		}
		y1 := minInt(y0+1, b.Dy()-1)
		for x := 0; x < dw; x++ {
			fx := (float64(x)+0.5)*float64(b.Dx())/float64(dw) - 0.5
			x0 := int(fx)
			wx := fx - float64(x0)
			if x0 < 0 {
				x0, wx = 0, 0
			}
			x1 := minInt(x0+1, b.Dx()-1)
			sample := func(xx, yy int) (float64, float64, float64, float64) {
				r, g, bl, a := src.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
				return float64(r >> 8), float64(g >> 8), float64(bl >> 8), float64(a >> 8)
			}
			r00, g00, b00, a00 := sample(x0, y0)
			r10, g10, b10, a10 := sample(x1, y0)
			r01, g01, b01, a01 := sample(x0, y1)
			r11, g11, b11, a11 := sample(x1, y1)
			mix := func(v00, v10, v01, v11 float64) float64 { return (v00*(1-wx)+v10*wx)*(1-wy) + (v01*(1-wx)+v11*wx)*wy }
			r, g, bl, a := mix(r00, r10, r01, r11), mix(g00, g10, g01, g11), mix(b00, b10, b01, b11), mix(a00, a10, a01, a11)
			alpha := a / 255.0
			out.SetRGBA(ox+x, oy+y, color.RGBA{R: uint8(r*alpha + float64(bg.R)*(1-alpha) + 0.5), G: uint8(g*alpha + float64(bg.G)*(1-alpha) + 0.5), B: uint8(bl*alpha + float64(bg.B)*(1-alpha) + 0.5), A: 255})
		}
	}
	return out
}

func resizeContainOnBackground(src image.Image, targetW, targetH int, bg color.RGBA) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, maxInt(1, targetW), maxInt(1, targetH)))
	draw.Draw(out, out.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return out
	}
	pad := 2
	usableW, usableH := maxInt(1, targetW-2*pad), maxInt(1, targetH-2*pad)
	scale := minFloat(float64(usableW)/float64(b.Dx()), float64(usableH)/float64(b.Dy()))
	dw, dh := maxInt(1, int(float64(b.Dx())*scale)), maxInt(1, int(float64(b.Dy())*scale))
	ox, oy := (targetW-dw)/2, (targetH-dh)/2
	for y := 0; y < dh; y++ {
		sy := b.Min.Y + y*b.Dy()/dh
		for x := 0; x < dw; x++ {
			sx := b.Min.X + x*b.Dx()/dw
			r16, g16, b16, a16 := src.At(sx, sy).RGBA()
			a := uint32(a16 >> 8)
			r := (uint32(r16>>8)*a + uint32(bg.R)*(255-a) + 127) / 255
			g := (uint32(g16>>8)*a + uint32(bg.G)*(255-a) + 127) / 255
			bl := (uint32(b16>>8)*a + uint32(bg.B)*(255-a) + 127) / 255
			out.SetRGBA(ox+x, oy+y, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(bl), A: 255})
		}
	}
	return out
}

func drawScaledNearest(dst *image.RGBA, dstRect image.Rectangle, src image.Image, srcRect image.Rectangle) {
	if dstRect.Empty() || srcRect.Empty() {
		return
	}
	for y := 0; y < dstRect.Dy(); y++ {
		sy := srcRect.Min.Y + y*srcRect.Dy()/dstRect.Dy()
		for x := 0; x < dstRect.Dx(); x++ {
			sx := srcRect.Min.X + x*srcRect.Dx()/dstRect.Dx()
			dst.Set(dstRect.Min.X+x, dstRect.Min.Y+y, src.At(sx, sy))
		}
	}
}

func buildLoginStandaloneIconBitmap(nav bool, index int, width, height int) (syscall.Handle, error) {
	var raw []byte
	prefix := "feature"
	if nav {
		prefix = "nav"
	}
	switch {
	case !nav && index == 0:
		raw = embeddedLoginFeature1PNG
	case !nav && index == 1:
		raw = embeddedLoginFeature2PNG
	case !nav && index == 2:
		raw = embeddedLoginFeature3PNG
	case !nav && index == 3:
		raw = embeddedLoginFeature4PNG
	case nav && index == 0:
		raw = embeddedLoginNav1PNG
	case nav && index == 1:
		raw = embeddedLoginNav2PNG
	case nav && index == 2:
		raw = embeddedLoginNav3PNG
	case nav && index == 3:
		raw = embeddedLoginNav4PNG
	default:
		return 0, os.ErrInvalid
	}
	name := filepath.Join(uiAssetCacheDir(), fmt.Sprintf("_login_%s_%d_v6.bmp", prefix, index+1))
	if info, err := os.Stat(name); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return 0, err
		}
		bg := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		if nav {
			bg = color.RGBA{R: 246, G: 248, B: 251, A: 255}
		}
		var out *image.RGBA
		if src.Bounds().Dx() == width && src.Bounds().Dy() == height {
			out = image.NewRGBA(image.Rect(0, 0, width, height))
			draw.Draw(out, out.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					r, g, b, a := src.At(src.Bounds().Min.X+x, src.Bounds().Min.Y+y).RGBA()
					aa := uint32(a >> 8)
					rr := (uint32(r>>8)*aa + uint32(bg.R)*(255-aa) + 127) / 255
					gg := (uint32(g>>8)*aa + uint32(bg.G)*(255-aa) + 127) / 255
					bb := (uint32(b>>8)*aa + uint32(bg.B)*(255-aa) + 127) / 255
					out.SetRGBA(x, y, color.RGBA{R: uint8(rr), G: uint8(gg), B: uint8(bb), A: 255})
				}
			}
		} else {
			out = resizeContainOnBackground(src, width, height, bg)
		}
		if err = writeBMP24(out, name); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(name))), IMAGE_BITMAP, uintptr(width), uintptr(height), LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func buildWarningIconBitmap() (syscall.Handle, error) {
	path := filepath.Join(uiAssetCacheDir(), "_warning_triangle_v1.bmp")
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		out := image.NewRGBA(image.Rect(0, 0, 32, 30))
		draw.Draw(out, out.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 237, B: 240, A: 255}}, image.Point{}, draw.Src)
		red := color.RGBA{R: 203, G: 27, B: 43, A: 255}
		white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		for y := 2; y <= 27; y++ {
			half := (y - 1) * 14 / 26
			for x := 16 - half; x <= 16+half; x++ {
				out.SetRGBA(x, y, red)
			}
		}
		for y := 9; y <= 19; y++ {
			for x := 15; x <= 17; x++ {
				out.SetRGBA(x, y, white)
			}
		}
		for y := 22; y <= 24; y++ {
			for x := 15; x <= 17; x++ {
				out.SetRGBA(x, y, white)
			}
		}
		if err := writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 32, 30, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// buildLoginThemesBitmap creates four compact line-art illustrations for the
// login screen: laboratory analytics, complication assessment, methods, and mass analysis.
func buildLoginThemesBitmap() (syscall.Handle, error) {
	path := filepath.Join(uiAssetCacheDir(), "_login_theme_strip_v3.bmp")
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(embeddedLoginThemeStripPNG))
		if err != nil {
			return 0, err
		}
		out := resizeContainOnBackground(src, 492, 120, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 492, 120, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func buildLoginSideBitmap(right bool) (syscall.Handle, error) {
	name := "_login_side_left_v5.bmp"
	data := embeddedLoginSideLeftPNG
	if right {
		name = "_login_side_right_v5.bmp"
		data = embeddedLoginSideRightPNG
	}
	path := filepath.Join(uiAssetCacheDir(), name)
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return 0, err
		}
		out := resizeContainOnBackground(src, 360, 650, color.RGBA{R: 238, G: 243, B: 248, A: 255})
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 360, 650, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func loginFillRect(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(dst, r.Intersect(dst.Bounds()), &image.Uniform{C: c}, image.Point{}, draw.Src)
}
func loginDrawLine(dst *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	dx, sx := absLogin(x1-x0), 1
	if x0 > x1 {
		sx = -1
	}
	dy, sy := -absLogin(y1-y0), 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		for yy := -thickness / 2; yy <= thickness/2; yy++ {
			for xx := -thickness / 2; xx <= thickness/2; xx++ {
				px, py := x0+xx, y0+yy
				if image.Pt(px, py).In(dst.Bounds()) {
					dst.SetRGBA(px, py, c)
				}
			}
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}
func loginFillCircle(dst *image.RGBA, cx, cy, radius int, c color.RGBA) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				px, py := cx+x, cy+y
				if image.Pt(px, py).In(dst.Bounds()) {
					dst.SetRGBA(px, py, c)
				}
			}
		}
	}
}
func absLogin(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func cropImage(src image.Image, r image.Rectangle) *image.RGBA {
	b := r.Intersect(src.Bounds())
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
	return out
}

func buildLoginFeatureIconBitmap(index int) (syscall.Handle, error) {
	name := []string{"_login_feature_icon_0_v1.bmp", "_login_feature_icon_1_v1.bmp", "_login_feature_icon_2_v1.bmp", "_login_feature_icon_3_v1.bmp"}[index]
	path := filepath.Join(uiAssetCacheDir(), name)
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(embeddedLoginRightReferencePNG))
		if err != nil {
			return 0, err
		}
		var rect image.Rectangle
		switch index {
		case 0:
			rect = image.Rect(28, 100, 92, 164)
		case 1:
			rect = image.Rect(26, 228, 94, 296)
		case 2:
			rect = image.Rect(26, 351, 98, 423)
		default:
			rect = image.Rect(28, 478, 98, 548)
		}
		crop := cropImage(src, rect)
		out := resizeContainOnBackground(crop, 52, 52, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 52, 52, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}

func buildLoginFeatureTilesBitmap() (syscall.Handle, error) {
	path := filepath.Join(uiAssetCacheDir(), "_login_feature_tiles_v1.bmp")
	if info, err := os.Stat(path); err != nil || info.Size() <= 54 {
		src, err := png.Decode(bytes.NewReader(embeddedLoginRightReferencePNG))
		if err != nil {
			return 0, err
		}
		crop := cropImage(src, image.Rect(22, 554, 592, 683))
		out := resizeContainOnBackground(crop, 492, 120, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		if err = writeBMP24(out, path); err != nil {
			return 0, err
		}
	}
	h, _, err := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(path))), IMAGE_BITMAP, 492, 120, LR_LOADFROMFILE)
	return syscall.Handle(h), err
}
