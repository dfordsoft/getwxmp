package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
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

func onRequestWeixinMP(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	fmt.Println(req.URL.String())
	for k, v := range req.Header {
		fmt.Println("    ", k, v)
	}
	fmt.Println("========================================================================\n\n")
	return req, nil
}

func onResponseWeixinMP(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	fmt.Println(resp.Request.URL.Path)
	if resp.Request.URL.Path == "/s" {
		// article
		fmt.Println()
	}
	fmt.Println("************************************************************************\n\n")
	return resp
}

func urlContains(u string) goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		return strings.Contains(req.URL.String(), u)
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
	fmt.Println("-----------------------------------------------------------------------\n\n")
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
	fmt.Println("-----------------------------------------------------------------------\n\n")
	return resp
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
	proxy.OnRequest(goproxy.DstHostIs("mp.weixin.qq.com:443")).DoFunc(onRequestWeixinMP)
	proxy.OnResponse(urlContains("mp.weixin.qq.com:443/s")).DoFunc(handleArticle)
	proxy.OnResponse(urlContains("mp.weixin.qq.com:443/mp/profile_ext")).DoFunc(handleProfileExt)
	proxy.Verbose = *verbose
	log.Fatal(http.ListenAndServe(*addr, proxy))
}
