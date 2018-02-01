package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"os"

	"github.com/dfordsoft/golib/fsutil"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
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
			<item id="content" media-type="text/x-oeb1-document" href="toc.html"></item>
			<item id="ncx" media-type="application/x-dtbncx+xml" href="toc.ncx"/>
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
	coverImage := image.NewRGBA(image.Rect(0, 0, 800, 600))
	col := color.RGBA{0, 0, 0, 255}
	point := fixed.Point26_6{fixed.Int26_6(10 * 64), fixed.Int26_6(200 * 64)}

	d := &font.Drawer{
		Dst:  coverImage,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(originalTitle)
	file, err := os.Create(wxmpTitle + "/images/cover.jpg")
	if err != nil {
		log.Println("creating cover.jpg failed", err)
		return false
	}
	defer file.Close()

	err = jpeg.Encode(file, coverImage, nil)
	if err != nil {
		log.Println("jpeg encoding failed", err)
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
