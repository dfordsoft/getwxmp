package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/dfordsoft/golib/semaphore"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/html"
	flags "github.com/jessevdk/go-flags"
	"github.com/mozillazg/go-pinyin"
)

// Options defines structure for command line options
type Options struct {
	Verbose         bool   `short:"v" long:"verbose" description:"should every proxy request be logged to stdout"`
	UpdateProxyOnly bool   `short:"p" long:"updateProxyOnly" description:"update proxy list only then exit immediately"`
	Address         string `short:"a" long:"address" description:"set listen address"`
	CaCert          string `long:"cacert" description:"set ca certificate file path"`
	CaKey           string `long:"cakey" description:"set ca private key file path"`
	Parallel        int    `long:"parallel" description:"set concurrent downloading count"`
}

var (
	wxmpTitle string
	opts      = Options{
		Verbose:         false,
		UpdateProxyOnly: false,
		Address:         ":8080",
		CaCert:          "cert/ca.cer",
		CaKey:           "cert/ca.key",
		Parallel:        15,
	}
	semaImage   *semaphore.Semaphore
	semaArticle *semaphore.Semaphore
	semaPDF     *semaphore.Semaphore
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

func main() {
	p := map[string]int{
		"windows":   50,
		"darwin":    15,
		"android":   15,
		"linux":     30,
		"dragonfly": 30,
		"freebsd":   30,
		"netbsd":    30,
		"openbsd":   30,
		"saloris":   30,
		"plan9":     30,
	}
	opts.Parallel = p[runtime.GOOS]

	_, err := flags.Parse(&opts)
	if err != nil {
		if err.(*flags.Error).Type != flags.ErrHelp {
			log.Fatalln("invalid command line options", err)
		}
		return
	}

	if opts.UpdateProxyOnly {
		updateProxy()
		return
	}

	semaImage = semaphore.New(opts.Parallel * 10)
	semaArticle = semaphore.New(opts.Parallel)
	semaPDF = semaphore.New(opts.Parallel)

	if err := setCA(opts.CaCert, opts.CaKey); err != nil {
		log.Fatalln(err)
	}

	go updateProxyPierodically()

	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	var r goproxy.ReqConditionFunc = func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.Contains(req.URL.String(), "action=getmsg")
	}
	proxy.OnRequest(r).DoFunc(onRequestWeixinMPArticleList)

	var resp goproxy.RespConditionFunc = func(r *http.Response, ctx *goproxy.ProxyCtx) bool {
		if r != nil && r.Request != nil && r.Request.URL != nil {
			return strings.Contains(r.Request.URL.String(), "profile_ext?action=home")
		}
		return false
	}
	proxy.OnResponse(resp).Do(goproxy_html.HandleString(func(s string, ctx *goproxy.ProxyCtx) string {
		beginStr := `<strong class="profile_nickname" id="nickname">`
		begin := strings.Index(s, beginStr)
		wxmpTitle = s[begin+len(beginStr):]
		endStr := `</strong>`
		end := strings.Index(wxmpTitle, endStr)
		wxmpTitle = wxmpTitle[:end]
		t := strings.TrimSpace(wxmpTitle)
		originalTitle := t
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
	}))

	proxy.Verbose = opts.Verbose
	proxy.Logger = log.New(ioutil.Discard, "GOPROXY: ", log.Ldate|log.Ltime|log.Lshortfile)
	log.Fatal(http.ListenAndServe(opts.Address, proxy))
}
