package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/dfordsoft/golib/semaphore"
)

var (
	proxyList      = `https://raw.githubusercontent.com/fate0/proxylist/master/proxy.list`
	semaProxy      = semaphore.New(5)
	wg             sync.WaitGroup
	proxyPool      []proxyItem
	proxyPoolMutex sync.RWMutex
)

type proxyItem struct {
	Port    json.Number `json:"port"`
	Type    string      `json:"type"`
	Host    string      `json:"host"`
	Country string      `json:"country"`
}

func insertProxyItem(pi proxyItem) {
	proxyPoolMutex.Lock()
	defer proxyPoolMutex.Unlock()
	for _, p := range proxyPool {
		if p.Type == pi.Type && p.Host == pi.Host && p.Port == pi.Port {
			return
		}
	}
	proxyPool = append(proxyPool, pi)
}

func removeProxyItem(pi proxyItem) {
	proxyPoolMutex.Lock()
	defer proxyPoolMutex.Unlock()
	for i, p := range proxyPool {
		if p.Type == pi.Type && p.Host == pi.Host && p.Port == pi.Port {
			proxyPool = append(proxyPool[:i], proxyPool[i+1:]...)
			return
		}
	}
}

func getProxyItem() proxyItem {
	proxyPoolMutex.RLock()
	defer proxyPoolMutex.RUnlock()
	if len(proxyPool) == 0 {
		return proxyItem{}
	}
	index := rand.Intn(len(proxyPool))
	return proxyPool[index]
}

func validateProxyItem(pi proxyItem) bool {
	c := clientPool.Get().(*http.Client)
	defer func() {
		clientPool.Put(c)
		semaProxy.Release()
		wg.Done()
	}()
	proxyString := fmt.Sprintf("%s://%s:%s", pi.Type, pi.Host, pi.Port)
	proxyURL, _ := url.Parse(proxyString)

	c.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}

	req, err := http.NewRequest("GET", "http://ip.cn", nil)
	if err != nil {
		//fmt.Println(err)
		removeProxyItem(pi)
		return false
	}

	req.Header.Set("User-Agent", "curl/7.54.0")
	resp, err := c.Do(req)
	if err != nil {
		//fmt.Println(err)
		removeProxyItem(pi)
		return false
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//fmt.Println(err)
		removeProxyItem(pi)
		return false
	}

	if len(content) > 200 {
		//fmt.Println("too long response is treated as unexpected response")
		removeProxyItem(pi)
		return false
	}
	c = nil
	return true
}

func updateProxy() {
	client := clientPool.Get().(*http.Client)
	client.Transport = http.DefaultTransport
	defer clientPool.Put(client)

	retry := 0
	req, err := http.NewRequest("GET", proxyList, nil)
	if err != nil {
		log.Println("proxy list - Could not parse proxy list request:", err)
		return
	}
doRequest:
	resp, err := client.Do(req)
	if err != nil {
		log.Println("proxy list - Could not send proxy list request:", err)
		retry++
		if retry < 3 {
			time.Sleep(3 * time.Second)
			goto doRequest
		}
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println("proxy list - proxy list request not 200")
		retry++
		if retry < 3 {
			time.Sleep(3 * time.Second)
			goto doRequest
		}
		return
	}

	c, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("proxy list - proxy list read err", err)
		retry++
		if retry < 3 {
			time.Sleep(3 * time.Second)
			goto doRequest
		}
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(c))
	scanner.Split(bufio.ScanLines)

	var pi proxyItem
	for scanner.Scan() {
		line := scanner.Text()
		if err = json.Unmarshal([]byte(line), &pi); err == nil {
			insertProxyItem(pi)
			semaProxy.Acquire()
			wg.Add(1)
			go validateProxyItem(pi)
		}
	}
	wg.Wait()
}

func updateProxyPierodically() {
	updateProxy()
	ticker := time.NewTicker(15 * time.Minute)
	for {
		select {
		case <-ticker.C:
			updateProxy()
		}
	}
}
