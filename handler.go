package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dfordsoft/golib/fsutil"
	"github.com/dfordsoft/golib/httputil"
	"github.com/dfordsoft/golib/semaphore"
	"github.com/elazarl/goproxy"
)

var (
	articleRequestURL        string
	articleRequestHeader     http.Header
	articleListRequestURL    string
	articleListRequestHeader http.Header
	semaArticle              = semaphore.New(15)
	wgWXMP                   sync.WaitGroup
	semaImage                = semaphore.New(150)
)

type getMsgResponse struct {
	Ret            int    `json:"ret"`
	ErrMsg         string `json:"errmsg"`
	MsgCount       int    `json:"msg_count"`
	CanMsgContinue int    `json:"can_msg_continue"`
	GeneralMsgList string `json:"general_msg_list"`
}

type mpList struct {
	List []struct {
		CommMsgInfo struct {
			ID       int    `json:"id"`
			Type     int    `json:"type"`
			DateTime int    `json:"datetime"`
			FakeID   string `json:"fakeid"`
			Status   int    `json:"status"`
			Content  string `json:"content"`
		} `json:"comm_msg_info"`
		AppMsgExtInfo struct {
			Title               string `json:"title"`
			Digest              string `json:"digest"`
			Content             string `json:"content"`
			FileID              int    `json:"fileid"`
			ContentURL          string `json:"content_url"`
			SourceURL           string `json:"source_url"`
			Cover               string `json:"cover"`
			SubType             int    `json:"subtype"`
			IsMulti             int    `json:"is_multi"`
			MultiAppMsgItemList []struct {
				Title         string `json:"title"`
				Digest        string `json:"digest"`
				Content       string `json:"content"`
				FileID        int    `json:"fileid"`
				ContentURL    string `json:"content_url"`
				SourceURL     string `json:"source_url"`
				Cover         string `json:"cover"`
				Author        string `json:"author"`
				CopyrightStat int    `json:"copyright_stat"`
				DelFlag       int    `json:"del_flag"`
			} `json:"multi_app_msg_item_list"`
			Author        string `json:"author"`
			CopyrightStat int    `json:"copyright_stat"`
			DelFlag       int    `json:"del_flag"`
		} `json:"app_msg_ext_info"`
	} `json:"list"`
}

func onRequestWeixinMPArticleList(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if articleListRequestURL != "" {
		return req, nil
	}
	articleListRequestURL = req.URL.String()
	articleListRequestHeader = req.Header
	start := strings.Index(articleListRequestURL, "offset=")
	end := strings.Index(articleListRequestURL[start:], "&")
	articleListRequestURL = articleListRequestURL[:start] + "offset=0" + articleListRequestURL[start:][end:]
	articleListRequestURL = strings.Replace(articleListRequestURL, ":443", "", -1)
	go getArticleList()
	return req, nil
}

func getArticleList() {
	rand.Seed(time.Now().UnixNano())
	type article struct {
		Title string
		URL   string
	}
	var articles []article

	duplicateArticle := func(a article) bool {
		for _, art := range articles {
			if a.URL == art.URL {
				return true
			}
		}
		return false
	}

	for i := 0; ; i += 10 {
		u := strings.Replace(articleListRequestURL, "offset=0", fmt.Sprintf("offset=%d", i), -1)
		b, e := httputil.GetBytes(u, articleListRequestHeader, 30*time.Second, 3)
		if e != nil {
			log.Fatalln(e)
			return
		}
		var m getMsgResponse
		if err := json.Unmarshal(b, &m); err != nil {
			log.Fatalln(err, string(b))
			return
		}

		var list mpList
		err := json.Unmarshal([]byte(m.GeneralMsgList), &list)
		if err != nil {
			log.Fatalln(err, m.GeneralMsgList)
			return
		}

		for _, v := range list.List {
			_, e := url.Parse(strings.Replace(v.AppMsgExtInfo.ContentURL, `&amp;`, `&`, -1))
			if v.AppMsgExtInfo.Title != "" && e == nil {
				a := article{Title: v.AppMsgExtInfo.Title, URL: strings.Replace(v.AppMsgExtInfo.ContentURL, `&amp;`, `&`, -1)}
				if !duplicateArticle(a) {
					articles = append(articles, a)
				}
			}
			for _, vv := range v.AppMsgExtInfo.MultiAppMsgItemList {
				_, e := url.Parse(strings.Replace(vv.ContentURL, `&amp;`, `&`, -1))
				if vv.Title != "" && e == nil {
					a := article{Title: vv.Title, URL: strings.Replace(vv.ContentURL, `&amp;`, `&`, -1)}
					if !duplicateArticle(a) {
						articles = append(articles, a)
					}
				}
			}
		}

		fmt.Printf("\r已经找到 %d 篇文章...", len(articles))
		if m.CanMsgContinue == 0 {
			break
		}
		time.Sleep(time.Duration(rand.Intn(4000)+1000) * time.Millisecond)
	}

	fmt.Println("一共找到", len(articles), "篇文章，开始下载...")

	list, e := os.OpenFile(wxmpTitle+`/list.txt`, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if e != nil {
		log.Fatalln("opening file "+wxmpTitle+"/list.txt for writing failed ", e)
		return
	}
	for _, a := range articles {
		list.WriteString(fmt.Sprintf("%s <==> %s\r\n", a.Title, a.URL))
	}
	list.Close()

	l := 1
	if len(articles) < 100 {
		l = 2
	} else if len(articles) < 1000 {
		l = 3
	} else if len(articles) < 10000 {
		l = 4
	} else {
		l = 5
	}
	for i, a := range articles {
		semaArticle.Acquire()
		fmt.Println("正在下载", fmt.Sprintf("%."+strconv.Itoa(l)+"d_article %s", i+1, a.Title), a.URL)
		go downloadArticle(fmt.Sprintf("%."+strconv.Itoa(l)+"d_article", i+1), a.URL)
	}

	wgWXMP.Wait()
	fmt.Println("全部采集完成！一共", len(articles), "篇文章。")

	var inputPaths []string
	for i := range articles {
		semaArticle.Acquire()
		inputFilePath := fmt.Sprintf("%s/%."+strconv.Itoa(l)+"d_article.html", wxmpTitle, i+1)
		if b, _ := fsutil.FileExists(inputFilePath); b {
			outputFilePath := fmt.Sprintf("%s/%."+strconv.Itoa(l)+"d_article.pdf", wxmpTitle, i+1)
			inputPaths = append(inputPaths, outputFilePath)
			fmt.Println("转换", inputFilePath, "为", outputFilePath)
			go phantomjs(inputFilePath, outputFilePath)
		} else {
			semaArticle.Release()
		}
	}

	wgWXMP.Wait()
	fmt.Println("全部转换为PDF！一共", len(inputPaths), "篇文章。")

	if err := mergePDF(inputPaths, wxmpTitle+".pdf"); err != nil {
		log.Println("merging PDF documents failed", err)
		return
	}
	fmt.Println("全部PDF合并为" + wxmpTitle + ".pdf")
}

func wkhtmltopdf(inputFilePath string, outputFilePath string) {
	wgWXMP.Add(1)
	defer func() {
		semaArticle.Release()
		wgWXMP.Done()
	}()
	cmd := exec.Command("wkhtmltopdf", inputFilePath, outputFilePath)
	cmd.Run()
}

func phantomjs(inputFilePath string, outputFilePath string) {
	wgWXMP.Add(1)
	defer func() {
		semaArticle.Release()
		wgWXMP.Done()
	}()
	cmd := exec.Command("phantomjs", "rasterize.js", inputFilePath, outputFilePath)
	cmd.Run()
}
