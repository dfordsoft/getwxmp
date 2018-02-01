package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/dfordsoft/golib/filter"
	"github.com/dfordsoft/golib/semaphore"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/html"
	flags "github.com/jessevdk/go-flags"
)

// Options defines structure for command line options
type Options struct {
	Verbose          bool   `short:"v" long:"verbose" description:"should every proxy request be logged to stdout"`
	DisableProxyLog  bool   `long:"disable-proxy-log" description:"disable proxy logs"`
	UpdateProxyOnly  bool   `short:"p" long:"update-proxy-only" description:"update proxy list only then exit immediately"`
	DirectConnecting bool   `short:"d" long:"direct-connecting" description:"download articles/image without proxy"`
	Format           string `short:"f" long:"format" description:"output format, supported: pdf, mobi"`
	Address          string `short:"a" long:"address" description:"set listen address"`
	CaCert           string `short:"c" long:"ca-cert" description:"set ca certificate file path"`
	CaKey            string `short:"k" long:"ca-key" description:"set ca private key file path"`
	PaperSize        string `short:"s" long:"paper-size" description:"set output PDF paper size, examples: 5in*7.5in, 10cm*20cm, A4, Letter. Supported dimension units are: 'mm', 'cm', 'in', 'px'. No unit means 'px'. Supported formats are: 'A3', 'A4', 'A5', 'Legal', 'Letter', 'Tabloid'."`
	Margin           string `short:"m" long:"margin" description:"set page margins, examples: 0px, 0.2cm. Supported dimension units are: 'mm', 'cm', 'in', 'px'. No unit means 'px'."`
	Zoom             string `short:"z" long:"zoom" description:"set paper zoom factor, the default is 1, i.e. 100% zoom."`
	FontFamily       string `long:"font-family" description:"set font family, which should be installed in the system"`
	Parallel         int    `long:"parallel" description:"set concurrent downloading count"`
	ReverseOrder     bool   `short:"r" long:"reverse-order" description:"put older articles in front"`
	Filter           string `short:"i" long:"filter" description:"set filter to article title, supported: contains(), equal(), suffix(), prefix(), regexp(), !contains(), !equal(), !suffix(), !prefix(), !regexp()"`
}

var (
	originalTitle string
	wxmpTitle     string
	opts          = Options{
		Verbose:          false,
		DisableProxyLog:  true,
		UpdateProxyOnly:  false,
		DirectConnecting: true,
		Format:           "mobi",
		Address:          ":8080",
		CaCert:           "cert/ca.cer",
		CaKey:            "cert/ca.key",
		Parallel:         15,
		ReverseOrder:     false,
		PaperSize:        "A4",
		Margin:           "0.2cm",
		Zoom:             "1",
	}
	semaImage   *semaphore.Semaphore
	titleFilter filter.F
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

func setProxy() *goproxy.ProxyHttpServer {
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
	proxy.OnResponse(resp).Do(goproxy_html.HandleString(homepageHandler))

	proxy.Verbose = opts.Verbose
	if opts.Verbose == false && opts.DisableProxyLog {
		proxy.Logger = log.New(ioutil.Discard, "GOPROXY: ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	return proxy
}

func main() {
	p := map[string]int{
		"windows":   30,
		"darwin":    15,
		"android":   5,
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
	titleFilter = filter.Filter(opts.Filter)

	if err := setCA(opts.CaCert, opts.CaKey); err != nil {
		log.Fatalln(err)
	}

	if !opts.DirectConnecting {
		go updateProxyPierodically()
	}

	for i := 0; i < opts.Parallel; i++ {
		go downloadArticleInQueue()
	}
	for i := 0; i < 15; i++ {
		go convertHTMLToPDFInQueue()
	}

	proxy := setProxy()
	log.Fatal(http.ListenAndServe(opts.Address, proxy))
}
