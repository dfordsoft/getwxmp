package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"os"
	"time"

	"github.com/missdeer/golib/fsutil"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func generateTOCNCX(articles []article) bool {
	tocNCXTemplate := `<?xml version="1.0" encoding="utf-8"?>
	<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" 
	   "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
	<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1" xml:lang="zh-CN">
	  <head>
	  </head>
	  <docTitle>
	  	<text>%s</text>
	  </docTitle>
	<navMap>
	<navPoint id="navpoint-1" playOrder="1"><navLabel><text>Content</text></navLabel><content src="toc.html#toc"/></navPoint>
		%s
	</navMap>
	</ncx>`
	navPoint := `<navPoint id="navpoint-%d" playOrder="%d"><navLabel><text>%s</text></navLabel><content src="%s.html"/></navPoint>`
	var navPoints string
	for i, a := range articles {
		if b, e := fsutil.FileExists(fmt.Sprintf("%s/%s.html", wxmpTitle, a.SaveAs)); e != nil || !b {
			continue
		}
		navPoints += fmt.Sprintf(navPoint, i+2, i+2, a.Title, a.SaveAs) + "\n"
	}
	tocNCXContent := fmt.Sprintf(tocNCXTemplate, originalTitle, navPoints)

	tocNCXFd, err := os.OpenFile(wxmpTitle+`/toc.ncx`, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Println("opening file "+wxmpTitle+"/toc.ncx for writing failed ", err)
		return false
	}
	tocNCXFd.WriteString(tocNCXContent)
	tocNCXFd.Close()

	return true
}

func generateContentOPF(articles []article) bool {

	opfTemplate := `<?xml version="1.0" encoding="utf-8"?>
	<package unique-identifier="uid" xmlns:opf="http://www.idpf.org/2007/opf" xmlns:asd="http://www.idpf.org/asdfaf">
		<metadata>
			<dc-metadata  xmlns:dc="http://purl.org/metadata/dublin_core" xmlns:oebpackage="http://openebook.org/namespaces/oeb-package/1.0/">
				<dc:Title>%s</dc:Title>
				<dc:Language>zh-CN</dc:Language>
				<dc:creator>GetWXMP用户制作</dc:creator>
				<dc:publisher>GetWXMP，仅限个人研究学习，对其造成的任何后果，软件作者不负任何责任</dc:publisher>
				<x-metadata>
					<EmbeddedCover>images/cover.jpg</EmbeddedCover>
				</x-metadata>
			</dc-metadata>
		</metadata>
		<manifest>
		%s
			<item id="content" media-type="text/x-oeb1-document" href="toc.html"/>
			<item id="ncx" media-type="application/x-dtbncx+xml" href="toc.ncx"/>
			<item id="cimage" media-type="image/jpeg" href="images/cover.jpg" properties="cover-image"/>
		</manifest>
		<spine toc="ncx">
			<itemref idref="content"/>
			%s
		</spine>
		<guide>
			<reference type="toc" title="Table of Contents" href="toc.html"/>
			<reference type="text" title="Book" href="1_article.html"/>
		</guide>
	</package>
	`

	item := `<item id="article%d" media-type="text/x-oeb1-document" href="%s.html"></item>`
	itemref := `<itemref idref="article%d"/>`
	var items, itemrefs string
	for i, a := range articles {
		if b, e := fsutil.FileExists(fmt.Sprintf("%s/%s.html", wxmpTitle, a.SaveAs)); e != nil || !b {
			continue
		}
		items += fmt.Sprintf(item, i+1, a.SaveAs) + "\n"
		itemrefs += fmt.Sprintf(itemref, i+1) + "\n"
	}
	opfContent := fmt.Sprintf(opfTemplate, originalTitle, items, itemrefs)
	opfFd, err := os.OpenFile(wxmpTitle+`/content.opf`, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Println("opening file "+wxmpTitle+"/content.opf for writing failed ", err)
		return false
	}
	opfFd.WriteString(opfContent)
	opfFd.Close()

	return true
}

func generateTOCHTML(articles []article) bool {
	tocHTMLTemplate := `<!DOCTYPE html>
	<html xmlns="http://www.w3.org/1999/xhtml">
	<head>
	<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
	<title>TOC</title>
	</head>
	<body>
	<h1 id="toc">目录</h1>
	<ul>
		%s
	</ul>
	</body>
	</html>`

	li := `<li><a href="%s.html">%s</a></li>`
	var lis string
	for _, a := range articles {
		if b, e := fsutil.FileExists(fmt.Sprintf("%s/%s.html", wxmpTitle, a.SaveAs)); e != nil || !b {
			continue
		}
		lis += fmt.Sprintf(li, a.SaveAs, a.Title) + "\n"
	}
	tocHTMLContent := fmt.Sprintf(tocHTMLTemplate, lis)
	tocHTMLFd, err := os.OpenFile(wxmpTitle+`/toc.html`, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Println("opening file "+wxmpTitle+"/toc.html for writing failed ", err)
		return false
	}
	tocHTMLFd.WriteString(tocHTMLContent)
	tocHTMLFd.Close()

	return true
}

func generateCover() bool {
	// cover.jpg
	os.Mkdir(wxmpTitle+"/images", 0755)

	// Read the font data.
	fontBytes, err := ioutil.ReadFile("fonts/CustomFont.ttf")
	if err != nil {
		log.Println(err)
		return false
	}
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Println(err)
		return false
	}

	// Draw the background and the guidelines.
	fg, bg := image.Black, image.White
	ruler := color.RGBA{0xdd, 0xdd, 0xdd, 0xff}
	const imgW, imgH = 800, 600
	rgba := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)
	for i := 0; i < 200; i++ {
		rgba.Set(10, 10+i, ruler)
		rgba.Set(10+i, 10, ruler)
	}

	const size = 36
	const dpi = 72
	const spacing = 1.2

	// Draw the text.
	h := font.HintingNone
	d := &font.Drawer{
		Dst: rgba,
		Src: fg,
		Face: truetype.NewFace(f, &truetype.Options{
			Size:    size,
			DPI:     dpi,
			Hinting: h,
		}),
	}
	y := imgH/3 + int(math.Ceil(size*dpi/72))
	dy := int(math.Ceil(size * spacing * dpi / 72))
	d.Dot = fixed.Point26_6{
		X: (fixed.I(imgW) - d.MeasureString(originalTitle)) / 2,
		Y: fixed.I(y),
	}
	d.DrawString(originalTitle)

	d.Face = truetype.NewFace(f, &truetype.Options{
		Size:    20,
		DPI:     dpi,
		Hinting: h,
	})

	y += dy
	t := `微信公众号文章合集`
	d.Dot = fixed.Point26_6{
		X: (fixed.I(imgW) - d.MeasureString(t)) / 2,
		Y: fixed.I(y),
	}
	d.DrawString(t)
	y += dy
	t = `截止到` + time.Now().Format(time.RFC3339)
	d.Dot = fixed.Point26_6{
		X: (fixed.I(imgW) - d.MeasureString(t)) / 2,
		Y: fixed.I(y),
	}
	d.DrawString(t)

	// Save that RGBA image to disk.
	outFile, err := os.Create(wxmpTitle + "/images/cover.jpg")
	if err != nil {
		log.Println("creating cover.jpg failed", err)
		return false
	}
	defer outFile.Close()
	b := bufio.NewWriter(outFile)
	err = jpeg.Encode(b, rgba, nil)
	if err != nil {
		log.Println("jpeg encoding failed", err)
		return false
	}
	err = b.Flush()
	if err != nil {
		log.Println(err)
		return false
	}

	return true
}

func generateMobiInput(articles []article) bool {
	// generate toc.html toc.ncx content.opf
	return generateTOCNCX(articles) &&
		generateContentOPF(articles) &&
		generateTOCHTML(articles) &&
		generateCover()
}

func processHTMLForMobi(c []byte) []byte {
	c = bytes.Replace(c, []byte(`<!--headTrap<body></body><head></head><html></html>-->`), []byte(""), -1)
	c = bytes.Replace(c, []byte(`<!--tailTrap<body></body><head></head><html></html>-->`), []byte(""), -1)

	startPos := bytes.Index(c, []byte("<script"))
	endPos := bytes.Index(c, []byte("<title>"))
	if startPos < 0 || endPos < startPos {
		return []byte{}
	}
	c = append(c[:startPos], c[endPos:]...)

	startPos = bytes.Index(c, []byte("<style>"))
	endPos = bytes.Index(c, []byte("</head>"))
	if startPos < 0 || endPos < startPos {
		return []byte{}
	}
	c = append(c[:startPos], c[endPos:]...)

	startPos = bytes.Index(c, []byte("<script"))
	endPos = bytes.Index(c, []byte("<div class=\"rich_media_content \" id=\"js_content\">"))
	if startPos < 0 || endPos < startPos {
		return []byte{}
	}
	c = append(c[:startPos], c[endPos:]...)

	startPos = bytes.Index(c, []byte("<script"))
	endPos = bytes.Index(c, []byte("</body>"))
	if startPos < 0 || endPos < startPos {
		return []byte{}
	}
	c = append(c[:startPos], c[endPos:]...)

	startPos = bytes.Index(c, []byte("<script"))
	endPos = bytes.LastIndex(c, []byte("</html>"))
	if startPos < 0 || endPos < startPos {
		return []byte{}
	}
	c = append(c[:startPos], c[endPos:]...)

	startPos = bytes.Index(c, []byte("<iframe"))
	endPos = bytes.LastIndex(c, []byte("</iframe>"))
	if startPos >= 0 && endPos > startPos {
		c = append(c[:startPos], c[endPos+len("</iframe>"):]...)
	}

	startPos = bytes.Index(c, []byte("<qqmusic"))
	endPos = bytes.LastIndex(c, []byte("</qqmusic>"))
	if startPos >= 0 && endPos > startPos {
		c = append(c[:startPos], c[endPos+len("</qqmusic>"):]...)
	}
	// remove style attributes
	leadingStr := ` style="`
	for startPos = bytes.Index(c, []byte(leadingStr)); startPos > 0; startPos = bytes.Index(c, []byte(leadingStr)) {
		endPos := bytes.Index(c[startPos+len(leadingStr):], []byte(`"`)) + startPos + len(leadingStr)
		if endPos > startPos {
			c = append(c[:startPos], c[endPos+1:]...)
		}
	}

	// extract paragraphs
	var ps [][]byte
	t := c
	leadingStr = "<p"
	endingStr := "</p>"
	for startPos = bytes.Index(t, []byte(leadingStr)); startPos >= 0; startPos = bytes.Index(t, []byte(leadingStr)) {
		endPos := bytes.Index(t[startPos:], []byte(endingStr))
		p := t[startPos : startPos+endPos+len(endingStr)]
		ps = append(ps, p)
		t = t[startPos+endPos+len(endingStr):]
	}
	// merge paragraphs
	t = bytes.Join(ps, []byte(""))
	leadingStr = "<div class=\"rich_media_content \" id=\"js_content\">"
	startPos = bytes.Index(c, []byte(leadingStr)) + len(leadingStr)
	endPos = bytes.LastIndex(c, []byte("</div>"))
	if startPos < 0 || endPos < startPos {
		return []byte{}
	}
	startStr := c[:startPos]
	endStr := c[endPos:]
	c = append(append(startStr, t...), endStr...)

	c = bytes.Replace(c, []byte(`<strong><br  /></strong>`), []byte(""), -1)
	c = bytes.Replace(c, []byte(`<span><br  /></span>`), []byte(""), -1)
	c = bytes.Replace(c, []byte(`<strong></strong>`), []byte(""), -1)
	c = bytes.Replace(c, []byte(`<span></span>`), []byte(""), -1)
	c = bytes.Replace(c, []byte(`<p></p>`), []byte(""), -1)
	c = bytes.Replace(c, []byte(`<p><br  /></p>`), []byte(""), -1)
	c = bytes.Replace(c, []byte(`<p>&nbsp;</p>`), []byte(""), -1)

	return c
}
