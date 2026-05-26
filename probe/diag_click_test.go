// +build ignore

// 临时诊断工具：测试点击搜索结果后是否打开新标签页。
//
// 运行方式：
//   go run probe/diag_click_test.go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func main() {
	allocatorCtx, _ := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9224")
	tmpCtx, _ := chromedp.NewContext(allocatorCtx)
	targets, err := chromedp.Targets(tmpCtx)
	if err != nil {
		log.Fatal("获取 targets 失败:", err)
	}

	fmt.Println("当前所有 targets:")
	for i, t := range targets {
		fmt.Printf("  [%d] type=%s title=%s url=%s\n", i, t.Type, t.Title, t.URL)
	}

	var ctx context.Context
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, "google.com/search") {
			fmt.Printf("\n复用 Google 搜索页: %s\n", t.URL)
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		log.Fatal("没有找到 Google 搜索页")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 步骤1：先提取第0个结果的 href，看看是什么
	fmt.Println("\n===== 步骤1：提取第0个结果的 href =====")
	var href string
	err = chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		if (!container) return 'NO_RSO';
		var h3s = container.querySelectorAll('h3');
		for (var i = 0; i < h3s.length; i++) {
			var a = h3s[i].closest('a');
			if (!a || !a.href) continue;
			// 跳过 PAA
			var block = h3s[i].parentElement;
			while (block && block.parentElement !== container) block = block.parentElement;
			if (block) {
				var t = block.textContent;
				if (t.indexOf('People also ask') !== -1 || t.indexOf('相关问题') !== -1) continue;
			}
			return JSON.stringify({
				href: a.href,
				target: a.getAttribute('target') || '(none)',
				rel: a.getAttribute('rel') || '(none)',
				tag: a.tagName,
				onclick: typeof a.onclick,
				hasJsHandler: typeof a.onclick === 'function' || a.getAttribute('onclick') || a.getAttribute('jsaction') || '(none)'
			});
		}
		return 'NO_RESULT';
	})()`, &href))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("  " + href)

	// 步骤2：注册 WaitNewTarget 然后点击，观察是否触发
	fmt.Println("\n===== 步骤2：点击 + WaitNewTarget 测试 =====")

	// 先统计当前 target 数量
	var beforeCount int
	chromedp.Run(ctx, chromedp.Evaluate(`1`, nil)) // no-op to ensure context is valid
	targetsBefore, _ := chromedp.Targets(ctx)
	beforeCount = len(targetsBefore)
	fmt.Printf("  点击前 target 数量: %d\n", beforeCount)

	ch := chromedp.WaitNewTarget(ctx, func(info *target.Info) bool {
		fmt.Printf("  WaitNewTarget 回调: type=%s url=%s title=%s\n", info.Type, info.URL, info.Title)
		return info.Type == "page"
	})

	var clickResult string
	err = chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		if (!container) return 'NO_RSO';
		var h3s = container.querySelectorAll('h3');
		var count = 0;
		var seen = {};
		for (var i = 0; i < h3s.length; i++) {
			var block = h3s[i].parentElement;
			while (block && block.parentElement !== container) block = block.parentElement;
			if (block) {
				var t = block.textContent;
				if (t.indexOf('People also ask') !== -1 || t.indexOf('相关问题') !== -1) continue;
			}
			var a = h3s[i].closest('a');
			if (!a) continue;
			var h = a.href;
			if (!h || seen[h]) continue;
			seen[h] = true;
			if (count === 0) {
				a.target = '_blank';
				a.click();
				return 'clicked href=' + h.substring(0, 80);
			}
			count++;
		}
		return 'NO_RESULT';
	})()`, &clickResult))
	if err != nil {
		log.Fatal("click 执行失败:", err)
	}
	fmt.Printf("  点击结果: %s\n", clickResult)

	// 等待新标签页
	fmt.Println("  等待新标签页（5秒超时）...")
	var targetID target.ID
	select {
	case targetID = <-ch:
		fmt.Printf("  捕获到新标签页: %s\n", targetID)
	case <-time.After(5 * time.Second):
		fmt.Println("  *** 超时！未检测到新标签页 ***")
		// 检查当前 targets 变化
		targetsAfter, _ := chromedp.Targets(ctx)
		fmt.Printf("  点击后 target 数量: %d\n", len(targetsAfter))
		for i, t := range targetsAfter {
			fmt.Printf("    [%d] type=%s title=%s url=%s\n", i, t.Type, t.Title, t.URL)
		}
	}
}
