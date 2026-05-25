// +build ignore

// 测试从非第1页定位 Page 1 的 aria-label 选择器。
// 运行方式: go run probe/test_page1_back.go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	allocatorCtx, _ := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9224")
	tmpCtx, _ := chromedp.NewContext(allocatorCtx)
	targets, _ := chromedp.Targets(tmpCtx)

	var ctx context.Context
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, "google.com/search") {
			fmt.Printf("复用搜索页: %s...\n", t.URL[:60])
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		log.Fatal("无 Google 搜索页")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// 步骤1：检查当前页的 Page 1 是否存在
	fmt.Println("\n===== 当前页 (第1页) =====")
	var r string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var a = document.querySelector('a[aria-label="Page 1"]');
		var t = document.querySelector('a[aria-label="Page 2"]');
		return 'Page1='+(a?'FOUND':'NOT FOUND') + ' Page2='+(t?'FOUND':'NOT FOUND');
	})()`, &r))
	fmt.Println("  " + r)

	// 步骤2：点击 Page 2 并等待加载
	fmt.Println("\n===== 点击 Page 2 =====")
	var clickResult string
	var r2 string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){
			var a = document.querySelector('a[aria-label="Page 2"]');
			if (a) { a.click(); return 'clicked'; }
			return 'NOT FOUND';
		})()`, &clickResult),
		chromedp.Sleep(4*time.Second),
		chromedp.WaitReady(`#rso`, chromedp.ByID),
		chromedp.Sleep(1*time.Second),

		// 步骤3：在第2页检查 Page 1
		chromedp.Evaluate(`(function(){
			var a = document.querySelector('a[aria-label="Page 1"]');
			if (a) return 'FOUND: text="'+a.textContent.trim()+'" href="'+(a.href||'').substring(0,80)+'"';
			return 'NOT FOUND';
		})()`, &r2),
	)
	if err != nil {
		log.Fatalf("点击 Page 2 并检查 Page 1 失败: %v", err)
	}
	fmt.Println("  点击结果: " + clickResult)
	fmt.Println("  Page 1: " + r2)

	// 步骤4：列出第2页上所有分页链接
	fmt.Println("\n===== 第2页分页链接 =====")
	var links string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r = [];
		var as = document.querySelectorAll('a[aria-label^="Page "]');
		for (var i = 0; i < as.length; i++) {
			r.push('  '+as[i].getAttribute('aria-label')+' text="'+as[i].textContent.trim()+'"');
		}
		return r.join('\\n');
	})()`, &links))
	fmt.Println(links)

	// 步骤5：测试 clickPaginationJS 在第2页上点击 Page 1
	fmt.Println("\n===== 测试 clickPaginationJS(1) =====")
	var r3 string
	chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(function(){
		var link = document.querySelector('a[aria-label="Page %d"]');
		if (link) { link.click(); return 'clicked'; }
		return '未找到第 %d 页的链接';
	})()`, 1, 1), &r3))
	fmt.Println("  clickPaginationJS(1): " + r3)

	// 等待跳转回第1页
	time.Sleep(2 * time.Second)
	var title string
	chromedp.Run(ctx, chromedp.Title(&title))
	fmt.Println("  当前标题: " + title)

	fmt.Println("\n===== 测试完成 =====")
}

func waitForResults(ctx context.Context) {
	const checkJs = `(function(){
		var rso = document.getElementById('rso');
		return rso !== null && rso.children.length > 0;
	})()`
	for {
		var ready bool
		if err := chromedp.Evaluate(checkJs, &ready).Do(ctx); err != nil {
			log.Fatal(err)
		}
		if ready {
			break
		}
		select {
		case <-ctx.Done():
			log.Fatal(ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	select {
	case <-ctx.Done():
		log.Fatal(ctx.Err())
	case <-time.After(2 * time.Second):
	}
}
