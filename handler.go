package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dfordsoft/golib/httputil"
	"github.com/elazarl/goproxy"
	pinyin "github.com/mozillazg/go-pinyin"
)

var (
	articleRequestURL        string
	articleRequestHeader     http.Header
	articleListRequestURL    string
	articleListRequestHeader http.Header
	wgWXMP                   sync.WaitGroup
	articleCount             int
)

const (
	titleSuffix = `_article`
)

type article struct {
	SaveAs string
	Title  string
	URL    string
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
	go articleInProgress()
	go getArticleList()
	return req, nil
}

func getArticleList() {
	rand.Seed(time.Now().UnixNano())
	var articles []article
	articleCount = 0
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
	getJSON:
		b, e := httputil.GetBytes(u, articleListRequestHeader, 30*time.Second, 3)
		if e != nil {
			log.Println("get json failed", e)
			return
		}
		var m getMsgResponse
		if err := json.Unmarshal(b, &m); err != nil {
			log.Println(err, string(b))
			time.Sleep(time.Duration(rand.Intn(4000)+1000) * time.Millisecond)
			goto getJSON
		}

		var list mpList
		err := json.Unmarshal([]byte(m.GeneralMsgList), &list)
		if err != nil {
			log.Println(err, m.GeneralMsgList)
			time.Sleep(time.Duration(rand.Intn(4000)+1000) * time.Millisecond)
			goto getJSON
		}

		for _, v := range list.List {
			_, e := url.Parse(strings.Replace(v.AppMsgExtInfo.ContentURL, `&amp;`, `&`, -1))
			if v.AppMsgExtInfo.Title != "" && e == nil && titleFilter(v.AppMsgExtInfo.Title) {
				a := article{
					SaveAs: fmt.Sprintf("%d%s", articleCount+1, titleSuffix),
					Title:  v.AppMsgExtInfo.Title,
					URL:    strings.Replace(v.AppMsgExtInfo.ContentURL, `&amp;`, `&`, -1),
				}
				if !duplicateArticle(a) {
					articles = append(articles, a)
					articleCount = len(articles)
					articleQueue <- a
				}
			}
			for _, vv := range v.AppMsgExtInfo.MultiAppMsgItemList {
				_, e := url.Parse(strings.Replace(vv.ContentURL, `&amp;`, `&`, -1))
				if vv.Title != "" && e == nil && titleFilter(vv.Title) {
					a := article{
						SaveAs: fmt.Sprintf("%d%s", articleCount+1, titleSuffix),
						Title:  vv.Title,
						URL:    strings.Replace(vv.ContentURL, `&amp;`, `&`, -1),
					}
					if !duplicateArticle(a) {
						articles = append(articles, a)
						articleCount = len(articles)
						articleQueue <- a
					}
				}
			}
		}

		if m.CanMsgContinue == 0 {
			break
		}
		time.Sleep(time.Duration(rand.Intn(7000)+8000) * time.Millisecond)
	}

	if strings.ToLower(opts.Format) == "mobi" {
		generateMobiInput(articles)
		for i := 0; i < len(articles); i++ {
			endConvertArticle <- true
		}
	}
}

func homepageHandler(s string, ctx *goproxy.ProxyCtx) string {
	beginStr := `<strong class="profile_nickname" id="nickname">`
	begin := strings.Index(s, beginStr)
	wxmpTitle = s[begin+len(beginStr):]
	endStr := `</strong>`
	end := strings.Index(wxmpTitle, endStr)
	if begin < 0 || end < 0 {
		log.Fatalln(s)
	}
	wxmpTitle = wxmpTitle[:end]
	t := strings.TrimSpace(wxmpTitle)
	originalTitle = t
	wxmpTitle = ""
	isCJK := false
	for len(t) > 0 {
		r, size := utf8.DecodeRuneInString(t)
		if size == 1 {
			if isCJK == true {
				isCJK = false
				wxmpTitle += "-"
			}
			wxmpTitle += string(r)
		} else {
			isCJK = true
			py := pinyin.LazyPinyin(string(r), pinyin.NewArgs())
			if wxmpTitle == "" {
				wxmpTitle = py[0]
			} else {
				wxmpTitle += "-" + py[0]
			}
		}
		t = t[size:]
	}
	fmt.Println("检测到微信公众号", originalTitle, "首页，往下滚动开始抓取所有文章到", wxmpTitle)

	os.Mkdir(wxmpTitle, 0755)
	return s
}
