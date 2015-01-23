// main_test
package main

import (
	"net/url"
	"testing"
)

type toAbsTest struct {
	pageURL, href, result string
}

var toAbsTestData = []toAbsTest{
	{"http://blog.hoday.cn", "?page=1", "http://blog.hoday.cn/?page=1"},
	{"http://blog.hoday.cn", "/tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},
	{"http://blog.hoday.cn", "tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},
	{"http://blog.hoday.cn", "http://blog.hoday.cn/post/java.html", "http://blog.hoday.cn/post/java.html"},
	{"http://blog.hoday.cn", "../tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},

	{"http://blog.hoday.cn/index.html", "?page=1", "http://blog.hoday.cn/index.html?page=1"},
	{"http://blog.hoday.cn/index.html", "/tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},
	{"http://blog.hoday.cn/index.html", "tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},
	{"http://blog.hoday.cn/index.html", "http://blog.hoday.cn/post/java.html", "http://blog.hoday.cn/post/java.html"},
	{"http://blog.hoday.cn/index.html", "../tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},

	{"http://blog.hoday.cn/post/golang.html", "?page=1", "http://blog.hoday.cn/post/golang.html?page=1"},
	{"http://blog.hoday.cn/post/golang.html", "/tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},
	{"http://blog.hoday.cn/post/golang.html", "tag/coding.html", "http://blog.hoday.cn/post/tag/coding.html"},
	{"http://blog.hoday.cn/post/golang.html", "http://blog.hoday.cn/post/java.html", "http://blog.hoday.cn/post/java.html"},
	{"http://blog.hoday.cn/post/golang.html", "../tag/coding.html", "http://blog.hoday.cn/tag/coding.html"},
}

var toAbsBenchmark = []string{"?page=1", "/tag/coding.html", "tag/coding.html","http://blog.hoday.cn/post/java.html","../tag/coding.html"}

func TestToAbs(t *testing.T) {
	for _, test := range toAbsTestData {
		p, err := url.Parse(test.pageURL)
		if err != nil {
			t.Fatalf("url.Parse(%q) %q", test.pageURL, err) //调用Fatal后会中断当前的测试函数
		}
		if u := toAbs(p, test.href); u.String() != test.result {
			t.Errorf("toAbs(%q, %q) = %q,  期望值 %q", p, test.href, u, test.result) //调用Error不会中断
		}
	}
}

func BenchmarkToAbs(b *testing.B) {
	b.StopTimer()
	pageURL := "http://blog.hoday.cn/post/golang.html"
	p, err := url.Parse(pageURL)
	if err != nil {
		b.Fatalf("url.Parse(%q) %q", pageURL, err) //调用Fatal后会中断当前的测试函数
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
        for _, val := range toAbsBenchmark{
        	if u := toAbs(p, val); u == nil {
				b.Fatalf("toAbs(%q, %q) 出错", p, val) //调用Error不会中断
			}
        }
    }
}
