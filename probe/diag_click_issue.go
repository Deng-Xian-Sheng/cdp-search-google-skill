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

	// 诊断搜索结果链接
	var info string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){
			var rso = document.getElementById('rso');
			if (!rso) return 'no #rso found';

			var h3s = rso.querySelectorAll('h3');
			var lines = [];
			for (var i = 0; i < h3s.length; i++) {
				var a = h3s[i].closest('a');
				if (!a) {
					lines.push('['+i+'] h3 has no closest a');
					continue;
				}
				lines.push('['+i+'] href=' + a.href + ', target=' + a.target + ', rel=' + a.rel + ', onclick=' + (a.onclick ? 'yes' : 'no'));
			}
			return lines.join('\n');
		})()`, &info),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(info)

	// 测试不带 target 直接点击
	var clickResult string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){
			var rso = document.getElementById('rso');
			var a = rso.querySelector('h3').closest('a');
			if (!a) return 'no a found';
			a.target = '_blank';
			a.click();
			return 'clicked';
		})()`, &clickResult),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("click result:", clickResult)

	time.Sleep(3 * time.Second)

	// 检查是否有新标签页
	var targetCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){
			// Just check if we're still on the same page
			return document.URL;
		})()`, &info),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("current URL:", info)

	targets2, _ := chromedp.Targets(tmpCtx)
	fmt.Printf("total targets: %d\n", len(targets2))
	for _, t := range targets2 {
		if t.Type == "page" {
			targetCount++
			fmt.Printf("  page target: %s url=%s\n", t.TargetID, t.URL)
		}
	}
	_ = targetCount
}
