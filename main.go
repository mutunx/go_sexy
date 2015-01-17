// sexy project main.go
package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//一张需要下载的图片
type image struct {
	imageURL string
	fileName string //保存到本地的文件名
	retry    int    //重试次数
}

//一个需要解析的页面
type page struct {
	url   string  //页面地址
	body  *[]byte //html数据
	retry int     //重试次数
}

type context struct {
	pageMap   map[string]int //记录已处理的页面，key是地址，value是处理状态
	imgMap    map[string]int //记录已处理的图片
	pageChan  chan *page     //待抓取的网页channel
	imgChan   chan *image    //待下载的图片channel
	parseChan chan *page     //待解析的网页channel
	imgCount  chan int       //统计已下载完成的图片
	savePath  string         //图片存放的路径，默认是 ./sexy
	rootURL   *url.URL       //起始地址，从这个页面开始爬
}

const (
	bufferSize     = 64 * 1024        //写图片文件的缓冲区大小
	numPoller      = 5                //抓取网页的并发数
	numDownloader  = 10               //下载图片的并发数
	maxRetry       = 3                //抓取网页或下载图片失败时重试的次数
	statusInterval = 15 * time.Second //进行状态监控的间隔
	chanBufferSize = 80               //待解析的

	//图片或页面处理状态
	ready = iota //待处理
	done         //已处理
	fail         //失败
)

var (
	imgExp  = regexp.MustCompile(`\s+bigimgsrc="([^"'<>]*)"`)          //regexp.MustCompile(`<img\s+src="([^"'<>]*)"/?>`)
	hrefExp = regexp.MustCompile(`\s+href="([a-zA-Z0-9_\-/:\.%?=]+)"`) //regexp.MustCompile(`\s+href="(http://sexy.faceks.com/[^"':<>]+)"`)
)

func main() {

	var savePath string

	root := "http://sexy.faceks.com/"

	rootURL, err := url.Parse(root)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("starting downloads...")

	switch len(os.Args) {
	case 1:
		savePath = "./sexy"
	case 2:
		savePath = os.Args[1]
	default:
		panic("invalid argument")
	}
	savePath += "/"
	os.MkdirAll(savePath, 0777)

	ctx := start(savePath, rootURL)

	stateMonitor(ctx)
}

//启动各种goroutine
func start(savePath string, rootURL *url.URL) (ctx *context) {
	ctx = &context{
		pageMap:   make(map[string]int),
		imgMap:    make(map[string]int),
		pageChan:  make(chan *page, chanBufferSize*3),
		imgChan:   make(chan *image, chanBufferSize*5),
		parseChan: make(chan *page, chanBufferSize),
		imgCount:  make(chan int),
		savePath:  savePath,
		rootURL:   rootURL,
	}

	//抓取网页
	for i := 0; i < numPoller; i++ {
		go func() {
			for {
				p := <-ctx.pageChan
				p.pollPage(ctx)
			}
		}()
	}

	//下载图片
	for i := 0; i < numDownloader; i++ {
		go func() {
			for {
				img := <-ctx.imgChan
				img.downloadImage(ctx)
			}
		}()
	}

	//用于解析html的goroutine
	//因为parsePage方法里需要对Map读写，
	//这个goroutine相当于对Map进行了同步的操作，
	//所以这个goroutine只能有一个，如果有多个就要对Map的操作做同步
	go func() {
		for {
			p := <-ctx.parseChan
			p.parsePage(ctx)
		}
	}()

	//放入起始页面，开始工作了
	ctx.pageChan <- &page{url: rootURL.String()}

	return ctx
}

//状态监控
func stateMonitor(ctx *context) {
	time.Sleep(10 * time.Second)
	ticker := time.NewTicker(statusInterval)
	count := 0
	done := true
	for {
		select {
		case <-ticker.C:
			fmt.Printf("========================================================\nqueue:page(%v)\timage(%v)\tparse(%v)\nimage:found(%v)\tdone(%v)\n========================================================\n", len(ctx.pageChan), len(ctx.imgChan), len(ctx.parseChan), len(ctx.imgMap), count)
			//当所有channel都为空，并且所有图片都已下载则退出程序
			if len(ctx.pageChan) == 0 && len(ctx.imgChan) == 0 && len(ctx.parseChan) == 0 {
				for _, val := range ctx.imgMap {
					if val == ready {
						done = false
						break
					}
				}
				if done {
					os.Exit(0)
				}
			}
		case c := <-ctx.imgCount:
			count += c //统计下载成功的图片数量
		}
	}
}

//获取页面html
func (p *page) pollPage(ctx *context) {
	//检查是否已解析
	if ctx.pageMap[p.url] == done {
		return
	}
	defer p.retryPage(ctx)
	fmt.Println(p.url)

	resp, err := http.Get(p.url)
	if err != nil {
		log.Print(err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)		
		return
	}
	ctx.pageMap[p.url] = done
	p.body = &body
	ctx.parseChan <- p
}

//失败后重新把页面放入channel
func (p *page) retryPage(ctx *context) {
	if ctx.pageMap[p.url] == done {
		return
	}
	//这里很奇葩，写if ++p.retry < maxRetry 会报错
	//写if ++p.retry; p.retry < maxRetry 也不行
	if p.retry++; p.retry < maxRetry {
		go func() {
			ctx.pageChan <- p
		}()
	}else{
		ctx.pageMap[p.url] = fail
	}
}

//解析页面html
func (p *page) parsePage(ctx *context) {
	body := *(p.body)
	pageURL, err := url.Parse(p.url)
	if err != nil {
		log.Println(err)
		return
	}

	if strings.Index(p.url, "/post/") > 0 { //post页面解析图片
		imgIndex := imgExp.FindAllSubmatchIndex(body, -1)
		for i, n := range imgIndex {
			imgUrl := strings.TrimSpace(string(body[n[2]:n[3]]))
			if imgUrl == "" {
				continue
			}
			_, exist := ctx.imgMap[imgUrl] //检查是否已放入队列，这里需要同步
			if !exist {
				ctx.imgMap[imgUrl] = ready
				fileName := path.Base(p.url) + "_" + strconv.Itoa(i) + path.Ext(imgUrl)
				ctx.imgChan <- &image{imgUrl, fileName, 0}

			}
		}
	}

	//解析链接
	hrefIndex := hrefExp.FindAllSubmatchIndex(body, -1)
	for _, n := range hrefIndex {
		linkURL := toAbs(pageURL, string(body[n[2]:n[3]]))
		if linkURL.Host != ctx.rootURL.Host { //忽略非本站的地址
			continue
		}
		href := linkURL.String()

		_, exist := ctx.pageMap[href] //检查是否已放入队列，需要同步
		if !exist {
			ctx.pageMap[href] = ready
			go func() { //这里必须异步，不然会和pollPage互相等待造成死锁
				ctx.pageChan <- &page{url: href}
			}()
		}
	}
}

//下载图片
func (imgInfo *image) downloadImage(ctx *context) {
	imgUrl := imgInfo.imageURL

	if ctx.imgMap[imgUrl] == done {
		return
	}
	defer imgInfo.imageRetry(ctx) //失败时重试

	resp, err := http.Get(imgUrl)
	if err != nil {
		log.Print(err)
		return
	}
	defer resp.Body.Close()

	//fmt.Println("download:" + imgUrl)
	saveFile := ctx.savePath + imgInfo.fileName //path.Base(imgUrl)

	img, err := os.Create(saveFile)
	if err != nil {
		log.Print(err)
		return
	}
	defer img.Close()

	imgWriter := bufio.NewWriterSize(img, bufferSize)
	_, err = io.Copy(imgWriter, resp.Body)

	if err != nil {
		log.Print(err)
		return
	}

	ctx.imgMap[imgUrl] = done
	ctx.imgCount <- 1
}

//失败重试
func (imgInfo *image) imageRetry(ctx *context) {
	if ctx.imgMap[imgInfo.imageURL] == done {
		return
	}
	if imgInfo.retry++; imgInfo.retry < maxRetry {
		go func() { //异步发送，避免阻塞
			ctx.imgChan <- imgInfo
		}()
	} else {
		ctx.imgMap[imgInfo.imageURL] = fail
	}
}

//转换成绝对地址
func toAbs(pageURL *url.URL, href string) *url.URL {
	if href[0] == '?' {
		href = pageURL.Path + href
	}

	h, err := url.Parse(href)
	if err != nil {
		log.Print(err)
	}

	if h.Scheme == "http" {
		return h
	}

	url := h.ResolveReference(pageURL)
	url.RawQuery = h.RawQuery
	return url
}
