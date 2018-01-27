package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/html"
	"github.com/mozillazg/go-pinyin"
)

var (
	wxmpTitle string
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
	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	updateProxyOnly := flag.Bool("proxy", false, "update proxy list only")
	addr := flag.String("addr", ":8080", "proxy listen address")
	caCert := flag.String("cert", "cert/ca.cer", "set ca certificate file path")
	caKey := flag.String("key", "cert/ca.key", "set ca private key file path")
	flag.Parse()

	if *updateProxyOnly {
		updateProxy()
		return
	}
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
		os.Mkdir(wxmpTitle, 0755)
		return s
	}))

	proxy.Verbose = *verbose
	log.Fatal(http.ListenAndServe(*addr, proxy))
}
