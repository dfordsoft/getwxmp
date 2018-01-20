package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net/http"

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
	proxy.Verbose = *verbose
	log.Fatal(http.ListenAndServe(*addr, proxy))
}
