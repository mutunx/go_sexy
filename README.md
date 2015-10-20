#go_sexy

Go语言实现sexy.faceks.com妹子图爬虫

纯粹是一个练手的项目，里面用到很多Go的特性，例如goroutine、channel、自定义类型、错误处理等等

放上来给大家参考一下，我也是初学golang，有哪些地方写得不好请指正

##更新说明
- 2015年10月20日 增加了配置文件的功能，把要抓取的网站地址和相关的正则表达式放在json配置文件里
    {
    	"root":"sexy.faceks.com",
    	"regex":{
    		"image":[
    			{
    				"exp":"bigimgsrc=\"([^\"?]+)",
    				"match":1,
    				"folder":"none"##可选值url,title,none,正则表达式
    			}
    		],
    		"page":[],
    		"imgInPage":["\S+/post/\S+"],
    		"href":[
    			{
    				"exp":"\s+href=\"([a-zA-Z0-9_\-/:\.%?=]+)\"",
    				"match":1
    			}
    		]
    	}
    }
	**配置文件使用json格式：**
	- root:字符串，要抓取的站点地址
	- regex.image:数组，用于匹配页面上的图片地址
	- regex.image.exp:字符串，匹配图片的正则表达式
	- regex.image.match:整数，指定图片地址在正则表达式里的哪个分组，0表示整个表达式匹配的内容，1表示第一个分组
	- regex.image.folder:字符串，可输入url，title，none或正则表达式
	 - url：使用图片所在页面的url的name(源码为path.Base(url))做文件夹名称
	 - title：使用页面的title
	 - none：不建文件夹，所有图片都放在一起
	 - 正则表达式：可以匹配页面上的内容来生成文件夹名称
	- regex.page:数组，存放正则表达式，只有符合正则表达式的页面才会被抓取并解析，留空表示所有页面都抓取并解析
	- regex.imgInPage:数组，存放正则表达式，用于指定图片存在于哪些页面里
	- regex.href:数组，用于匹配页面上的超链接
	 - regex.href.exp:字符串，存放匹配超链接的正则表达式
	 - regex.href.match:整数，指定超链接在正则表达式里的哪个分组，0表示整个表达式匹配的内容，1表示第一个分组

##实现原理
![实现原理](http://git.oschina.net/xpan-lu/go_sexy/raw/master/theory.png)