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
