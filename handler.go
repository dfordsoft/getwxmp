package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dfordsoft/golib/fsutil"
	"github.com/dfordsoft/golib/httputil"
	"github.com/elazarl/goproxy"
)

var (
	articleRequestURL        string
	articleRequestHeader     http.Header
	articleListRequestURL    string
	articleListRequestHeader http.Header
	wgWXMP                   sync.WaitGroup
)

const (
	titleSuffix = `_article`
)

type article struct {
	Title string
	URL   string
}
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
			if v.AppMsgExtInfo.Title != "" && e == nil && titleFilter(v.AppMsgExtInfo.Title) {
				a := article{Title: v.AppMsgExtInfo.Title, URL: strings.Replace(v.AppMsgExtInfo.ContentURL, `&amp;`, `&`, -1)}
				if !duplicateArticle(a) {
					articles = append(articles, a)
					title := fmt.Sprintf("%d%s", len(articles), titleSuffix)
					go processArticle(title, a)
				}
			}
			for _, vv := range v.AppMsgExtInfo.MultiAppMsgItemList {
				_, e := url.Parse(strings.Replace(vv.ContentURL, `&amp;`, `&`, -1))
				if vv.Title != "" && e == nil && titleFilter(vv.Title) {
					a := article{Title: vv.Title, URL: strings.Replace(vv.ContentURL, `&amp;`, `&`, -1)}
					if !duplicateArticle(a) {
						articles = append(articles, a)
						title := fmt.Sprintf("%d%s", len(articles), titleSuffix)
						go processArticle(title, a)
					}
				}
			}
		}

		if m.CanMsgContinue == 0 {
			break
		}
		time.Sleep(time.Duration(rand.Intn(4000)+1000) * time.Millisecond)
	}

	// wait for all articles and images downloaded, and convert them to PDFs
	wgWXMP.Wait()
	fmt.Printf("总共下载%d篇文章，并已转换为PDF格式，准备合并为 %s.pdf\n", len(articles), wxmpTitle)

	// merge those PDFs into a single big PDF document
	var inputPaths []string
	for i := 0; i < len(articles); i++ {
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
	if err := mergePDF(inputPaths, wxmpTitle+".pdf"); err != nil {
		log.Println("merging PDF documents failed", err)
		return
	}
	fmt.Println("全部PDF已合并为", wxmpTitle+".pdf")

	articleListRequestURL = ""
	fmt.Println("可以继续抓取其他微信公众号文章了。")
}
