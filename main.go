// sexy project main.go
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go_sexy/conf"
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
	"sync"
	"time"
)

//一张需要下载的图片
type image struct {
	imageURL string
	fileName string //保存到本地的文件名
	retry    int    //重试次数
	folder   string //存放的文件夹
}

//一个需要解析的页面
type page struct {
	url   string  //页面地址
	body  *[]byte //html数据
	retry int     //重试次数
	parse bool    //true时将不检查url是否符合config中配置的正则表达式
}

type context struct {
	lockPage  sync.RWMutex
	pageMap   map[string]int //记录已处理的页面，key是地址，value是处理状态
	lockImg   sync.RWMutex
	imgMap    map[string]int //记录已处理的图片
	pageChan  chan *page     //待抓取的网页channel
	imgChan   chan *image    //待下载的图片channel
	parseChan chan *page     //待解析的网页channel
	imgCount  chan int       //统计已下载完成的图片
	savePath  string         //图片存放的路径
	rootURL   *url.URL       //起始地址，从这个页面开始爬
	config    *conf.Config   //配置信息
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
	titleExp       = regexp.MustCompile(`<title>([^<>]+)</title>`) //regexp.MustCompile(`<img\s+src="([^"'<>]*)"/?>`)
	invalidCharExp = regexp.MustCompile(`[\\/*?:><|]`)
	headers        = map[string]string{"Host": "img.mmjpg.com", "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36",
		"Referer": "http://www.mmjpg.com/mm/1059/6"}
)

func main() {
	configFile := "config.json"
	cf := &conf.Config{}

	if len(os.Args) >= 2 {
		configFile = os.Args[1]
	}

	if err := cf.Load(configFile); err != nil {
		panic("some error occurred when loading the config file:" + err.Error())
	}

	fmt.Println("start download...")

	savePath := "./" + cf.Root.Host + "/"

	os.MkdirAll(savePath, 0777)

	ctx := start(savePath, cf)

	stateMonitor(ctx)
}

//启动各种goroutine
func start(savePath string, cf *conf.Config) (ctx *context) {
	ctx = &context{
		pageMap:   make(map[string]int),
		imgMap:    make(map[string]int),
		pageChan:  make(chan *page, chanBufferSize*3),
		imgChan:   make(chan *image, chanBufferSize*5),
		parseChan: make(chan *page, chanBufferSize),
		imgCount:  make(chan int),
		savePath:  savePath,
		rootURL:   cf.Root,
		config:    cf,
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
	ctx.pageChan <- &page{url: cf.Root.String(), parse: true}

	return ctx
}

//状态监控
func stateMonitor(ctx *context) {
	time.Sleep(10 * time.Second)
	ticker := time.NewTicker(statusInterval)
	count := 0
	isDone := true
	for {
		select {
		case <-ticker.C:
			fmt.Printf("========================================================\nqueue:page(%v)\timage(%v)\tparse(%v)\nimage:found(%v)\tdone(%v)\n========================================================\n", len(ctx.pageChan), len(ctx.imgChan), len(ctx.parseChan), len(ctx.imgMap), count)
			//当所有channel都为空，并且所有图片都已下载则退出程序
			if len(ctx.pageChan) == 0 && len(ctx.imgChan) == 0 && len(ctx.parseChan) == 0 {
				isDone = true
				for _, val := range ctx.imgMap {
					if val == ready {
						isDone = false
						break
					}
				}
				if isDone {
					fmt.Println("is done!")
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
	ctx.lockPage.RLock()
	if ctx.pageMap[p.url] == done {
		ctx.lockPage.RUnlock()
		return
	} else {
		ctx.lockPage.RUnlock()
	}
	defer p.retryPage(ctx)

	client := &http.Client{}
	req, err := http.NewRequest("GET", p.url, nil)
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Print("pollPage[1]:" + err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Print("pollPage[2]:" + err.Error())
		return
	}

	ctx.lockPage.Lock()
	ctx.pageMap[p.url] = done
	ctx.lockPage.Unlock()
	p.body = &body
	ctx.parseChan <- p
}

//失败后重新把页面放入channel
func (p *page) retryPage(ctx *context) {
	ctx.lockPage.RLock()
	if ctx.pageMap[p.url] == done {
		ctx.lockPage.RUnlock()
		return
	} else {
		ctx.lockPage.RUnlock()
	}
	//这里很奇葩，写if ++p.retry < maxRetry 会报错
	//写if ++p.retry; p.retry < maxRetry 也不行
	if p.retry++; p.retry < maxRetry {
		go func() {
			ctx.pageChan <- p
		}()
	} else {
		ctx.lockPage.Lock()
		ctx.pageMap[p.url] = fail
		ctx.lockPage.Unlock()
	}
}

//解析页面html
func (p *page) parsePage(ctx *context) {
	//页面解析图片 strings.Index(p.url, "/post/") > 0
	if matchUrl(p.url, ctx.config.ImgPageRegex) {
		log.Println("match ImagePage URL : " + p.url)
		for _, reg := range ctx.config.ImageRegex {
			p.findImage(ctx, reg)
		}
	}

	//只有符合正则表达式的页面才去解析
	if matchUrl(p.url, ctx.config.PageRegex) || p.parse {
		log.Println("match Page URL : " + p.url)
		for _, reg := range ctx.config.HrefRegex {
			p.findURL(ctx, reg)
		}
	}
}

//在页面html中查找图片地址
func (p *page) findImage(ctx *context, reg *conf.MatchExp) {
	body := *(p.body)
	imgIndex := reg.Exp.FindAllSubmatchIndex(body, -1)
	folder := p.createImageFolder(ctx, reg)

	pageURL, err := url.Parse(p.url)
	if err != nil {
		log.Print("findImage[1]:" + err.Error())
		return
	}

	for i, n := range imgIndex {
		idxBegin, idxEnd := 2*reg.Match, 2*reg.Match+1
		imgUrl := strings.TrimSpace(string(body[n[idxBegin]:n[idxEnd]]))
		if imgUrl == "" {
			continue
		}
		absURL := toAbs(pageURL, imgUrl) //转换成绝对地址
		absURL.Fragment = ""             //删除锚点
		imgUrl = absURL.String()
		ctx.lockImg.RLock()
		_, exist := ctx.imgMap[imgUrl] //检查是否已放入队列，这里需要同步
		ctx.lockImg.RUnlock()
		if !exist {
			ctx.lockImg.Lock()
			ctx.imgMap[imgUrl] = ready
			ctx.lockImg.Unlock()
			fileName := path.Base(p.url) + "_" + strconv.Itoa(i) + path.Ext(imgUrl)

			fmt.Println("imgUrl:", imgUrl)

			ctx.imgChan <- &image{imgUrl, fileName, 0, folder}
		}
	}
}

//创建图片文件夹
func (p *page) createImageFolder(ctx *context, reg *conf.MatchExp) string {
	var folder string
	body := *(p.body)

	fd, ok := reg.Folder.(regexp.Regexp)
	if ok {
		loc := fd.FindIndex(body)
		if loc == nil {
			return ctx.savePath
		}
		folder = string(body[loc[0]:loc[1]])
	} else {
		fdstr, ok := reg.Folder.(string)
		if !ok {
			return ctx.savePath
		}
		switch fdstr {
		case "url":
			folder = path.Base(p.url)
			if folder == "" {
				folder = "root"
			}
		case "title":
			loc := titleExp.FindSubmatchIndex(body)
			if loc == nil {
				return ctx.savePath
			}
			folder = string(body[loc[2]:loc[3]])
		case "none":
			return ctx.savePath
		}
	}

	folder = invalidCharExp.ReplaceAllString(folder, "")
	folder = ctx.savePath + "/" + folder + "/"
	err := os.Mkdir(folder, 0777)
	if err != nil {
		return ctx.savePath
	}
	return folder

}

//解析页面上的链接
func (p *page) findURL(ctx *context, reg *conf.MatchExp) {
	body := *(p.body)
	pageURL, err := url.Parse(p.url)
	if err != nil {
		log.Print("findURL[1]:" + err.Error())
		return
	}
	//解析链接
	hrefIndex := reg.Exp.FindAllSubmatchIndex(body, -1)
	for _, n := range hrefIndex {
		idxBegin, idxEnd := 2*reg.Match, 2*reg.Match+1
		linkURL := toAbs(pageURL, string(body[n[idxBegin]:n[idxEnd]])) //转换成绝对地址
		if linkURL == nil || linkURL.Host != ctx.rootURL.Host {        //忽略非本站的地址
			continue
		}
		linkURL.Fragment = "" //删除锚点
		href := linkURL.String()

		ctx.lockPage.RLock()
		_, exist := ctx.pageMap[href] //检查是否已放入队列，需要同步
		ctx.lockPage.RUnlock()
		if !exist {
			ctx.lockPage.Lock()
			ctx.pageMap[href] = ready
			ctx.lockPage.Unlock()
			go func() { //这里必须异步，不然会和pollPage互相等待造成死锁
				ctx.pageChan <- &page{url: href}
			}()
		}
	}
}

//下载图片
func (imgInfo *image) downloadImage(ctx *context) {
	imgUrl := imgInfo.imageURL

	ctx.lockImg.RLock()
	if ctx.imgMap[imgUrl] == done {
		ctx.lockImg.RUnlock()
		return
	} else {
		ctx.lockImg.RUnlock()
	}
	defer imgInfo.imageRetry(ctx) //失败时重试

	client := &http.Client{}
	req, err := http.NewRequest("GET", imgUrl, nil)
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Print("downloadImage[1]:" + err.Error())
		return
	}
	defer resp.Body.Close()

	//fmt.Println("download:" + imgUrl)
	saveFile := imgInfo.folder + imgInfo.fileName //path.Base(imgUrl)

	img, err := os.Create(saveFile)
	if err != nil {
		log.Print("downloadImage[2]:" + err.Error())
		return
	}
	defer img.Close()

	imgWriter := bufio.NewWriterSize(img, bufferSize)

	_, err = io.Copy(imgWriter, resp.Body)
	if err != nil {
		log.Print("downloadImage[3]:" + err.Error())
		return
	}
	imgWriter.Flush()

	ctx.lockImg.Lock()
	ctx.imgMap[imgUrl] = done
	ctx.imgCount <- 1
	ctx.lockImg.Unlock()
}

//失败重试
func (imgInfo *image) imageRetry(ctx *context) {
	ctx.lockImg.RLock()
	if ctx.imgMap[imgInfo.imageURL] == done {
		ctx.lockImg.RUnlock()
		return
	} else {
		ctx.lockImg.RUnlock()
	}
	if imgInfo.retry++; imgInfo.retry < maxRetry {
		go func() { //异步发送，避免阻塞
			ctx.imgChan <- imgInfo
		}()
	} else {
		ctx.lockImg.Lock()
		ctx.imgMap[imgInfo.imageURL] = fail
		ctx.lockImg.Unlock()
	}
}

//转换成绝对地址
func toAbs(pageURL *url.URL, href string) *url.URL {
	//.  ..  /  ? http https
	var buf bytes.Buffer
	if h := strings.ToLower(href); strings.Index(h, "http://") == 0 || strings.Index(h, "https://") == 0 {
		buf.WriteString(href)
	} else {
		buf.WriteString(pageURL.Scheme)
		buf.WriteString("://")
		buf.WriteString(pageURL.Host)

		switch href[0] {
		case '?':
			if len(pageURL.Path) == 0 {
				buf.WriteByte('/')
			} else {
				buf.WriteString(pageURL.Path)
			}
			buf.WriteString(href)
		case '/':
			buf.WriteString(href)
		default:
			p := "/" + path.Dir(pageURL.Path) + "/" + href
			buf.WriteString(path.Clean(p))
		}
	}

	h, err := url.Parse(buf.String())
	if err != nil {
		log.Print("toAbs[1]:" + err.Error())
		return nil
	}
	return h
}

//判断正则表达式是否能匹配制定的url
//匹配则返回true，否则返回false
func matchUrl(url string, reglist []*regexp.Regexp) bool {
	if reglist == nil || len(reglist) == 0 {
		return true
	}
	for _, reg := range reglist {
		if reg.MatchString(url) {
			return true
		}
	}
	return false
}
