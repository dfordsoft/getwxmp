package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dfordsoft/golib/fsutil"
	uuid "github.com/satori/go.uuid"
)

var (
	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/604.4.7 (KHTML, like Gecko) Version/11.0.2 Safari/604.4.7",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.108 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.13; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.94 Safari/537.36 OPR/49.0.2725.64",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36 Edge/16.16299",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.108 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/604.4.7 (KHTML, like Gecko) Version/11.0.2 Safari/604.4.7",
		"Mozilla/5.0 (Windows NT 6.3; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko",
		"Mozilla/5.0 (Windows NT 6.3; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:52.0) Gecko/20100101 Firefox/52.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.94 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.12; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.108 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.108 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Ubuntu Chromium/63.0.3239.84 Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.3; Win64; x64; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; rv:11.0) like Gecko",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64; rv:52.0) Gecko/20100101 Firefox/52.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_1) AppleWebKit/604.3.5 (KHTML, like Gecko) Version/11.0.1 Safari/604.3.5",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36 OPR/50.0.2762.58",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/604.4.7 (KHTML, like Gecko) Version/11.0.2 Safari/604.4.7",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/52.0.2743.116 Safari/537.36 Edge/15.15063",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.11; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.108 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:58.0) Gecko/20100101 Firefox/58.0",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.94 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:52.0) Gecko/20100101 Firefox/52.0",
		"Mozilla/5.0 (Windows NT 10.0; WOW64; rv:56.0) Gecko/20100101 Firefox/56.0",
		"Mozilla/5.0 (X11; Fedora; Linux x86_64; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.94 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/603.3.8 (KHTML, like Gecko) Version/10.1.2 Safari/603.3.8",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.94 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; Trident/7.0; rv:11.0) like Gecko",
		"Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0;  Trident/5.0)",
		"Mozilla/5.0 (iPad; CPU OS 11_2_1 like Mac OS X) AppleWebKit/604.4.7 (KHTML, like Gecko) Version/11.0 Mobile/15C153 Safari/604.1",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.84 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; rv:52.0) Gecko/20100101 Firefox/52.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:57.0) Gecko/20100101 Firefox/57.0",
		"Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.0; Trident/5.0;  Trident/5.0)",
		"Mozilla/5.0 (Windows NT 5.1; rv:52.0) Gecko/20100101 Firefox/52.0",
		"Mozilla/5.0 (X11; Linux x86_64; rv:58.0) Gecko/20100101 Firefox/58.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/604.3.5 (KHTML, like Gecko) Version/11.0.1 Safari/604.3.5",
	}
	clientPool = sync.Pool{
		New: func() interface{} {
			return &http.Client{
				Timeout: 90 * time.Second,
				Transport: &http.Transport{
					DisableKeepAlives: true,
					IdleConnTimeout:   30 * time.Second,
				},
			}
		},
	}

	articleQueue         = make(chan article, 2000)
	htmlQueue            = make(chan string, 2000)
	stopDownload         = make(chan bool)
	stopConvert          = make(chan bool)
	startDownloadArticle = make(chan bool)
	endConvertArticle    = make(chan bool)
)

func downloadArticleInQueue() {
	for {
		select {
		case a := <-articleQueue:
			startDownloadArticle <- true
			downloadArticle(a)
			if strings.ToLower(opts.Format) == "pdf" {
				htmlQueue <- a.SaveAs
			}
		case <-stopDownload:
			return
		}
	}
}

func convertHTMLToPDFInQueue() {
	for {
		select {
		case saveAs := <-htmlQueue:
			inputFilePath := fmt.Sprintf("%s/%s.html", wxmpTitle, saveAs)
			outputFilePath := fmt.Sprintf("%s/%s.pdf", wxmpTitle, saveAs)
			convertToPDF(inputFilePath, outputFilePath)
			endConvertArticle <- true
		case <-stopConvert:
			return
		}
	}
}

func articleInProgress() {
	articleInProgress := 0
	for {
		select {
		case <-startDownloadArticle:
			articleInProgress++
		case <-endConvertArticle:
			articleInProgress--
			if articleInProgress == 0 {
				switch strings.ToLower(opts.Format) {
				case "pdf":
					go postConverteHTMLToPDF()
				case "mobi":
					fmt.Println(`You need to run kindlegen utility to generate the final mobi file.`)
					fmt.Printf("For example: kindlegen -c2 -o %s.mobi content.opf\n", wxmpTitle)
				}
				return
			}
		}
	}
}

func downloadArticle(a article) bool {
	fmt.Println("正在下载", a.Title, a.URL, "到", a.SaveAs)
	client := clientPool.Get().(*http.Client)
	defer func() {
		clientPool.Put(client)
	}()
doRequest:
	if !opts.DirectConnecting {
		pi := getProxyItem()
		proxyString := fmt.Sprintf("%s://%s:%s", pi.Type, pi.Host, pi.Port)
		proxyURL, _ := url.Parse(proxyString)

		client.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
	}

	req, err := http.NewRequest("GET", a.URL, nil)
	if err != nil {
		//log.Println("article - Could not parse article request:", err)
		return false
	}
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])

	resp, err := client.Do(req)
	if err != nil {
		//log.Println("article - Could not send article request:", err)
		time.Sleep(3 * time.Second)
		goto doRequest
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		//log.Println("article - article request not 200")
		time.Sleep(3 * time.Second)
		goto doRequest
	}

	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		//log.Println("article - ", err)
		return false
	}

	invalid := `<p class="title">接相关投诉，此内容违反《即时通信工具公众信息服务发展管理暂行规定》，查看<a href="http://www.cac.gov.cn/2014-08/07/c_1111983456.htm">详细内容</a></p>`
	if bytes.Contains(content, []byte(invalid)) {
		return true
	}

	dir := fmt.Sprintf("%s/%s", wxmpTitle, a.SaveAs)
	os.Mkdir(dir, 0755)
	contentHTML, err := os.OpenFile(wxmpTitle+`/`+a.SaveAs+`.html`, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Println("opening file "+wxmpTitle+`/`+a.SaveAs+`.html`+" for writing failed ", err)
		return false
	}

	contentHTML.Write(processArticleHTMLContent(a.SaveAs, content))
	contentHTML.Close()

	return true
}

func downloadImage(savePath string, u string, wg *sync.WaitGroup) bool {
	client := clientPool.Get().(*http.Client)
	defer func() {
		clientPool.Put(client)
		semaImage.Release()
		wg.Done()
	}()
doRequest:
	if !opts.DirectConnecting {
		pi := getProxyItem()
		proxyString := fmt.Sprintf("%s://%s:%s", pi.Type, pi.Host, pi.Port)
		proxyURL, _ := url.Parse(proxyString)

		client.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		//log.Println("image - Could not parse image request:", err)
		return false
	}
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])

	resp, err := client.Do(req)
	if err != nil {
		//log.Println("image - Could not send image request:", err)
		time.Sleep(3 * time.Second)
		goto doRequest
	}

	if resp.StatusCode != 200 {
		//log.Println("image - image request not 200")
		resp.Body.Close()
		time.Sleep(3 * time.Second)
		goto doRequest
	}

	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		//log.Println("image - ", err)
		return false
	}

	if ext := filepath.Ext(savePath); strings.ToLower(ext) == ".gif" {
		if err = saveAnimatedGIFAsStaticImage(bytes.NewReader(content), savePath); err == nil {
			return true
		}
		log.Println("saving animated GIF as static image failed", err)
	}
	return saveImage(content, savePath)
}

func saveImage(b []byte, savePath string) bool {
	image, err := os.OpenFile(savePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Println("opening file ", savePath, " for writing failed ", err)
		return false
	}

	image.Write(b)
	image.Close()
	return true
}

func saveAnimatedGIFAsStaticImage(reader io.Reader, savePath string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error while decoding: %s", r)
		}
	}()

	gifImage, err := gif.DecodeAll(reader)
	if err != nil {
		return err
	}

	imgWidth, imgHeight := getGifDimensions(gifImage)

	overpaintImage := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	draw.Draw(overpaintImage, overpaintImage.Bounds(), gifImage.Image[0], image.ZP, draw.Src)

	for _, srcImg := range gifImage.Image {
		draw.Draw(overpaintImage, overpaintImage.Bounds(), srcImg, image.ZP, draw.Over)
	}
	// save current frame "stack". This will overwrite an existing file with that name
	file, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = gif.Encode(file, overpaintImage, nil)
	if err != nil {
		return err
	}

	return nil
}

func getGifDimensions(gif *gif.GIF) (x, y int) {
	var lowestX int
	var lowestY int
	var highestX int
	var highestY int

	for _, img := range gif.Image {
		if img.Rect.Min.X < lowestX {
			lowestX = img.Rect.Min.X
		}
		if img.Rect.Min.Y < lowestY {
			lowestY = img.Rect.Min.Y
		}
		if img.Rect.Max.X > highestX {
			highestX = img.Rect.Max.X
		}
		if img.Rect.Max.Y > highestY {
			highestY = img.Rect.Max.Y
		}
	}

	return highestX - lowestX, highestY - lowestY
}

func parseDataSrc(b []byte) (originalURL string, ext string) {
	re, _ := regexp.Compile(`data\-src="([^"]+)"`)
	cc := re.FindAllSubmatch(b, -1)
	for _, c := range cc {
		originalURL = string(c[1])
		begin := strings.Index(originalURL, "wx_fmt=")
		if begin > 0 {
			ext = originalURL[begin+7:]
		}
		end := strings.Index(ext, "\"")
		if end > 0 {
			ext = ext[:end]
		}
		if ext != "" {
			return
		}
	}

	re2, _ := regexp.Compile(`data\-type="([^"]+)"`)
	cc = re2.FindAllSubmatch(b, -1)
	for _, c := range cc {
		ext = string(c[1])
		return
	}

	return
}

func parseSrc(b []byte) (originalURL string, ext string) {
	re2, _ := regexp.Compile(` src="([^"]+)"`)
	cc := re2.FindAllSubmatch(b, -1)
	for _, c := range cc {
		originalURL = string(c[1])
		fileName := originalURL
		lastSlash := strings.LastIndex(fileName, "/")
		if lastSlash > 0 {
			fileName = fileName[lastSlash+1:]
		}
		ext = filepath.Ext(fileName)
		if ext != "" {
			ext = ext[1:]
		}
		return
	}

	return
}

func processArticleHTMLContent(saveTo string, c []byte) []byte {
	var wg sync.WaitGroup
	re, _ := regexp.Compile(`<img[^>]+>`)
	b := re.FindAllSubmatch(c, -1)
	m := make(map[string]string)
	for _, bb := range b {
		if originalURL, ext := parseDataSrc(bb[0]); originalURL != "" && ext != "" {
			savePath := fmt.Sprintf("%s/%s/%s.%s", wxmpTitle, saveTo, uuid.Must(uuid.NewV4()).String(), ext)
			m[originalURL] = savePath
			wg.Add(1)
			semaImage.Acquire()
			go downloadImage(savePath, originalURL, &wg)
		}

		if originalURL, ext := parseSrc(bb[0]); originalURL != "" && ext != "" {
			savePath := fmt.Sprintf("%s/%s/%s.%s", wxmpTitle, saveTo, uuid.Must(uuid.NewV4()).String(), ext)
			m[originalURL] = savePath
			if strings.HasPrefix(originalURL, "//") {
				originalURL = "https:" + originalURL
			}
			wg.Add(1)
			semaImage.Acquire()
			go downloadImage(savePath, originalURL, &wg)
		}
	}
	wg.Wait()
	for originalURL, localPath := range m {
		c = bytes.Replace(c, []byte(fmt.Sprintf(`data-src="%s"`, originalURL)), []byte(fmt.Sprintf(`src="%s"`, localPath[len(wxmpTitle)+1:])), -1)
		c = bytes.Replace(c, []byte(originalURL), []byte(localPath[len(wxmpTitle)+1:]), -1)
	}
	if opts.FontFamily != "" {
		c = bytes.Replace(c, []byte(`"Helvetica Neue"`), []byte(opts.FontFamily+`,"Helvetica Neue"`), -1)
	}
	if strings.ToLower(opts.Format) == "mobi" {
		return processHTMLForMobi(c)
	}
	return c
}

func convertToPDF(inputFilePath string, outputFilePath string) {
	fmt.Println("正在转换", inputFilePath, "为", outputFilePath)
	cmd := exec.Command("phantomjs", "rasterize.js", inputFilePath, outputFilePath, opts.PaperSize, opts.Zoom, opts.Margin)
	cmd.Run()
}

func postConverteHTMLToPDF() {
	fmt.Printf("总共下载%d篇文章，并已转换为PDF格式，准备合并为 %s.pdf\n", articleCount, wxmpTitle)

	// merge those PDFs into a single big PDF document
	var inputPaths []string
	for i := 0; i < articleCount; i++ {
		inputFilePath := fmt.Sprintf("%s/%d%s.pdf", wxmpTitle, i+1, titleSuffix)
		if b, _ := fsutil.FileExists(inputFilePath); b {
			if opts.ReverseOrder {
				inputPaths = append([]string{inputFilePath}, inputPaths...)
			} else {
				inputPaths = append(inputPaths, inputFilePath)
			}
		}
	}

	fmt.Println(inputPaths)
	if err := mergePDFs(inputPaths, wxmpTitle+".pdf"); err != nil {
		log.Println("merging PDF documents failed", err)
		return
	}
	fmt.Println("全部PDF已合并为", wxmpTitle+".pdf")

	articleListRequestURL = ""
	fmt.Println("可以继续抓取其他微信公众号文章了。")
}
