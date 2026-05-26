// +build ignore

// 最终验证：搜索 + 多分页分析，验证 clickResultJS 定位正确性。
//
// 运行方式：
//   go run probe/diag_final.go
package main

import (
	"context"
	"fmt"
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
		if t.Type == "page" {
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		ctx = tmpCtx
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// 步骤1: 搜索
	fmt.Println("===== 搜索: golang tutorial =====")
	inputSelector := `textarea[name="q"]`

	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://google.com`),
		chromedp.WaitVisible(inputSelector),
		chromedp.Focus(inputSelector),
		chromedp.SendKeys(inputSelector, "golang tutorial"),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Submit(inputSelector),
	)
	if err != nil {
		fmt.Printf("搜索失败: %v\n", err)
		return
	}

	// 等待结果
	waitRSO(ctx)

	// 确认在结果页
	var u string
	chromedp.Run(ctx, chromedp.Evaluate(`window.location.href`, &u))
	fmt.Printf("URL: %.90s\n", u)
	if !strings.Contains(u, "/search") {
		fmt.Println("不在搜索结果页!")
		return
	}

	// 步骤2: 分析第1页
	fmt.Println("\n===== 第1页 =====")
	analyzePage(ctx)

	// 步骤3: 跳到第2页
	fmt.Println("\n===== 第2页 =====")
	var r string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){
			var a = document.querySelector('a[aria-label="Page 2"]');
			if (a) { a.click(); return 'clicked'; }
			return 'NOT FOUND';
		})()`, &r),
	)
	if err != nil || r != "clicked" {
		fmt.Printf("跳转失败: r=%s err=%v\n", r, err)
		return
	}
	waitRSO(ctx)
	chromedp.Run(ctx, chromedp.Evaluate(`window.location.href`, &u))
	fmt.Printf("URL: %.90s\n", u)
	analyzePage(ctx)

	// 步骤4: 跳到第3页
	fmt.Println("\n===== 第3页 =====")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){
			var a = document.querySelector('a[aria-label="Page 3"]');
			if (a) { a.click(); return 'clicked'; }
			return 'NOT FOUND';
		})()`, &r),
	)
	if err != nil || r != "clicked" {
		fmt.Printf("跳转失败: r=%s err=%v\n", r, err)
	} else {
		waitRSO(ctx)
		analyzePage(ctx)
	}

	fmt.Println("\n===== 全部完成 =====")
}

func analyzePage(ctx context.Context) {
	// 1. 容器结构
	var overview string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NO #rso';
		var r = [];
		for (var i = 0; i < rso.children.length; i++) {
			var c = rso.children[i];
			var h3s = c.querySelectorAll('h3');
			if (h3s.length === 0) continue;
			var txt = c.textContent.substring(0,200);
			var flags = '';
			if (txt.indexOf('People also ask')>=0 || txt.indexOf('相关问题')>=0) flags += ' PAA';
			r.push('['+i+'] '+h3s.length+'h3' + flags + ' class="'+(c.className||'')+'"');
		}
		return r.join(' | ');
	})()`, &overview))
	fmt.Printf("  容器: %s\n", overview)

	// 2. clickResultJS 模拟
	var sim string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		function isPAA(el, rso) {
			var block = el.parentElement;
			while (block && block.parentElement !== rso) block = block.parentElement;
			if (!block) return false;
			var t = block.textContent;
			return t.indexOf('People also ask')>=0 || t.indexOf('相关问题')>=0;
		}
		var h3s = container.querySelectorAll('h3');
		var ok=[], skipped=[], seen={};
		for (var i=0;i<h3s.length;i++) {
			if (isPAA(h3s[i],container)) { skipped.push('PAA'); continue; }
			var a=h3s[i].closest('a');
			if (!a||!a.href) { skipped.push('NO_A'); continue; }
			if (seen[a.href]) { skipped.push('DUP'); continue; }
			seen[a.href]=true;
			ok.push((ok.length)+': '+h3s[i].textContent.trim().substring(0,50));
		}
		return JSON.stringify({total:h3s.length, usable:ok.length, skipped:skipped.length, top5:ok.slice(0,5), skipReasons:skipped.slice(0,5)});
	})()`, &sim))
	fmt.Printf("  clickResultJS: %s\n", sim)

	// 3. 异常h3
	var anom string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso=document.getElementById('rso');
		var h3s=rso.querySelectorAll('h3'), bad=0;
		for (var i=0;i<h3s.length;i++) {
			var a=h3s[i].closest('a');
			if (!a||!a.href) bad++;
		}
		return bad===0?'ALL_OK':(bad+' no-a h3');
	})()`, &anom))
	fmt.Printf("  异常h3: %s\n", anom)
}

func waitRSO(ctx context.Context) {
	const checkJs = `(function(){
		var rso = document.getElementById('rso');
		return rso !== null && rso.children.length > 0;
	})()`
	for i := 0; i < 30; i++ {
		var ready bool
		if err := chromedp.Evaluate(checkJs, &ready).Do(ctx); err == nil && ready {
			time.Sleep(2 * time.Second)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
