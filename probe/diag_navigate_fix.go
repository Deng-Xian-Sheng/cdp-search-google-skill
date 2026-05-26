package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	allocatorCtx, _ := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9224")
	tmpCtx, _ := chromedp.NewContext(allocatorCtx)

	var ctx context.Context
	targets, err := chromedp.Targets(tmpCtx)
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range targets {
		if t.Type == "page" {
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		ctx = tmpCtx
	}

	// 方案 A: 提取 href 然后用 chromedp 在新 context 中导航
	var href string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){
			var rso = document.getElementById('rso');
			var h3s = rso.querySelectorAll('h3');
			// 跳过 "People also ask" 区域的 h3
			for (var i = 0; i < h3s.length; i++) {
				var block = h3s[i].parentElement;
				while (block && block.parentElement !== rso) {
					block = block.parentElement;
				}
				if (block) {
					var text = block.textContent;
					if (text.indexOf('People also ask') !== -1 || text.indexOf('相关问题') !== -1) continue;
				}
				var a = h3s[i].closest('a');
				if (a && a.href) return a.href;
			}
			return '';
		})()`, &href),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("href:", href)
	fmt.Println("href length:", len(href))

	// 方案 B: 在当前页面通过 JS 修改 window.location（不打开新标签页）
	// 然后把 HTML 发回来
	if href != "" {
		// 创建新 context 并导航到 href
		newCtx, cancel := chromedp.NewContext(allocatorCtx)
		defer cancel()
		newCtx, newCancel := context.WithTimeout(newCtx, 30*time.Second)
		defer newCancel()

		var html string
		err = chromedp.Run(newCtx,
			chromedp.Navigate(href),
			chromedp.WaitReady(`body`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
			chromedp.Evaluate(`document.documentElement.outerHTML`, &html),
		)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("HTML length:", len(html))
		fmt.Println("HTML first 200:", html[:200])
	}
}
