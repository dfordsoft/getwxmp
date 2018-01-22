package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dfordsoft/golib/httputil"
	"github.com/elazarl/goproxy"
)

var (
	articleRequestURL        string
	articleRequestHeader     http.Header
	articleListRequestURL    string
	articleListRequestHeader http.Header
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

func onRequestWeixinMPArticle(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if articleRequestURL != "" {
		return req, nil
	}
	articleRequestURL = req.URL.String()
	articleRequestHeader = req.Header
	fmt.Println(req.URL.String())
	for k, v := range req.Header {
		fmt.Println("    ", k, v)
	}
	fmt.Printf("========================================================================\n\n")
	return req, nil
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
			for _, vv := range v.AppMsgExtInfo.MultiAppMsgItemList {
				fmt.Println(vv.Title, strings.Replace(vv.ContentURL, `&amp;`, `&`, -1))
			}
		}

		if m.CanMsgContinue == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
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

func handleArticle(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctx.Warnf("Cannot read string from resp body: %v", err)
		return resp
	}
	resp.Body.Close()
	for k, v := range resp.Request.Header {
		fmt.Println("    ", k, v)
	}
	resp.Body = &readFirstCloseBoth{ioutil.NopCloser(bytes.NewBuffer(b)), resp.Body}
	fmt.Printf("-----------------------------------------------------------------------\n\n")
	return resp
}

func handleProfileExt(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctx.Warnf("Cannot read string from resp body: %v", err)
		return resp
	}
	resp.Body.Close()
	for k, v := range resp.Request.Header {
		fmt.Println("    ", k, v)
	}
	resp.Body = &readFirstCloseBoth{ioutil.NopCloser(bytes.NewBuffer(b)), resp.Body}
	fmt.Printf("-----------------------------------------------------------------------\n\n")
	return resp
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

func handleMsgList(s string, ctx *goproxy.ProxyCtx) string {
	var m getMsgResponse
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		log.Fatalln(err)
		return s
	}

	var list mpList
	err := json.Unmarshal([]byte(m.GeneralMsgList), &list)
	if err != nil {
		log.Fatalln(err)
		return s
	}

	fmt.Println(list)

	fmt.Printf("\n\n")
	return s
}

func main() {
	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":8080", "proxy listen address")
	caCert := flag.String("cert", "cert/ca.cer", "set ca certificate file path")
	caKey := flag.String("key", "cert/ca.key", "set ca private key file path")
	flag.Parse()
	if err := setCA(*caCert, *caKey); err != nil {
		log.Fatalln(err)
	}
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	var r goproxy.ReqConditionFunc = func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.Contains(req.URL.String(), "mp.weixin.qq.com:443/s")
	}
	proxy.OnRequest(r).DoFunc(onRequestWeixinMPArticle)

	r = func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.Contains(req.URL.String(), "action=getmsg")
	}
	proxy.OnRequest(r).DoFunc(onRequestWeixinMPArticleList)

	proxy.Verbose = *verbose
	log.Fatal(http.ListenAndServe(*addr, proxy))
}
