package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dfordsoft/golib/httputil"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/html"
	"github.com/mozillazg/go-pinyin"
	"golang.org/x/sync/semaphore"
)

var (
	articleRequestURL        string
	articleRequestHeader     http.Header
	articleListRequestURL    string
	articleListRequestHeader http.Header
	ctxArticle               = context.TODO()
	semaArticle              = semaphore.NewWeighted(10)
	wgWXMP                   sync.WaitGroup
	ctxImage                 = context.TODO()
	semaImage                = semaphore.NewWeighted(20)
	wxmpTitle                string
)

func setCA(caCert, caKey string) error {
	goproxyCa, err := tls.LoadX509KeyPair(caCert, caKey)
	if err != nil {
		return err
	}
	if goproxyCa.Leaf, err = x509.ParseCertificate(goproxyCa.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = goproxyCa
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	return nil
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
			articles = append(articles, article{Title: v.AppMsgExtInfo.Title, URL: strings.Replace(v.AppMsgExtInfo.ContentURL, `&amp;`, `&`, -1)})
			for _, vv := range v.AppMsgExtInfo.MultiAppMsgItemList {
				articles = append(articles, article{Title: vv.Title, URL: strings.Replace(vv.ContentURL, `&amp;`, `&`, -1)})
			}
		}

		if m.CanMsgContinue == 0 {
			break
		}
		time.Sleep(time.Duration(rand.Intn(4000)+1000) * time.Millisecond)
	}

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
		semaArticle.Acquire(ctxArticle, 1)
		fmt.Println("downloading", fmt.Sprintf("%."+strconv.Itoa(l)+"d_article %s", i+1, a.Title), a.URL)
		go downloadArticle(fmt.Sprintf("%."+strconv.Itoa(l)+"d_article", i+1), a.URL)
	}

	wgWXMP.Wait()
	fmt.Println("全部采集完成！一共", len(articles), "篇文章。")

	var inputPaths []string
	for i := range articles {
		semaArticle.Acquire(ctxArticle, 1)
		inputFilePath := fmt.Sprintf("%s/%."+strconv.Itoa(l)+"d_article.html", wxmpTitle, i+1)
		if b, _ := isFileExists(inputFilePath); b {
			outputFilePath := fmt.Sprintf("%s/%."+strconv.Itoa(l)+"d_article.pdf", wxmpTitle, i+1)
			inputPaths = append(inputPaths, outputFilePath)
			fmt.Println("converting", inputFilePath, "to", outputFilePath)
			go convertToPDF(inputFilePath, outputFilePath)
		} else {
			semaArticle.Release(1)
		}
	}

	wgWXMP.Wait()
	fmt.Println("全部转换为PDF！一共", len(inputPaths), "篇文章。")

	if err := mergePdf(inputPaths, wxmpTitle+".pdf"); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("全部PDF合并为" + wxmpTitle + ".pdf")
}

func isFileExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == nil {
		if stat.Mode()&os.ModeType == 0 {
			return true, nil
		}
		return false, errors.New(path + " exists but is not regular file")
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func convertToPDF(inputFilePath string, outputFilePath string) {
	wgWXMP.Add(1)
	defer func() {
		semaArticle.Release(1)
		wgWXMP.Done()
	}()
	//cmd := exec.Command("wkhtmltopdf", inputFilePath, outputFilePath)
	cmd := exec.Command("phantomjs", "rasterize.js", inputFilePath, outputFilePath)
	cmd.Run()
}

type readFirstCloseBoth struct {
	r io.ReadCloser
	c io.Closer
}

func (rfcb *readFirstCloseBoth) Read(b []byte) (nr int, err error) {
	return rfcb.r.Read(b)
}
func (rfcb *readFirstCloseBoth) Close() error {
	err1 := rfcb.r.Close()
	err2 := rfcb.c.Close()
	if err1 != nil && err2 != nil {
		return errors.New(err1.Error() + ", " + err2.Error())
	}
	if err1 != nil {
		return err1
	}
	return err2
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

func main() {
	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":8080", "proxy listen address")
	caCert := flag.String("cert", "cert/ca.cer", "set ca certificate file path")
	caKey := flag.String("key", "cert/ca.key", "set ca private key file path")
	flag.Parse()

	go updateProxyPierodically()

	if err := setCA(*caCert, *caKey); err != nil {
		log.Fatalln(err)
	}
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	var r goproxy.ReqConditionFunc = func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.Contains(req.URL.String(), "action=getmsg")
	}
	proxy.OnRequest(r).DoFunc(onRequestWeixinMPArticleList)

	var resp goproxy.RespConditionFunc = func(r *http.Response, ctx *goproxy.ProxyCtx) bool {
		return strings.Contains(r.Request.URL.String(), "profile_ext?action=home")
	}
	proxy.OnResponse(resp).Do(goproxy_html.HandleString(func(s string, ctx *goproxy.ProxyCtx) string {
		beginStr := `<strong class="profile_nickname" id="nickname">`
		begin := strings.Index(s, beginStr)
		wxmpTitle = s[begin+len(beginStr):]
		endStr := `</strong>`
		end := strings.Index(wxmpTitle, endStr)
		wxmpTitle = wxmpTitle[:end]
		py := pinyin.LazyPinyin(strings.TrimSpace(wxmpTitle), pinyin.NewArgs())
		wxmpTitle = strings.Join(py, "-")
		os.Mkdir(wxmpTitle, 0644)
		return s
	}))

	proxy.Verbose = *verbose
	log.Fatal(http.ListenAndServe(*addr, proxy))
}
