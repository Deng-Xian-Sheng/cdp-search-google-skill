// +build ignore

// 诊断导航问题：检查 Google 首页状态，解决搜索不跳转的问题。
//
// 运行方式：
//   go run probe/diag_navigation.go
package main

import (
	"context"
	"fmt"
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

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// 1. 导航到 Google 首页
	fmt.Println("===== 导航到 google.com =====")
	var navErr error
	navErr = chromedp.Run(ctx, chromedp.Navigate(`https://google.com`))
	if navErr != nil {
		fmt.Printf("导航失败: %v\n", navErr)
		return
	}

	time.Sleep(3 * time.Second)

	// 2. 检查当前页面状态
	var pageInfo string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r = [];
		r.push('url: '+window.location.href.substring(0,100));
		r.push('title: '+document.title);

		// 检查 textarea
		var ta = document.querySelector('textarea[name="q"]');
		r.push('textarea[name=q]: '+(ta?'FOUND':'NOT FOUND'));

		// 检查各种可能的输入框
		var inputs = document.querySelectorAll('input[name="q"], textarea[name="q"], input[type="text"], textarea, [role="combobox"]');
		r.push('all search inputs: '+inputs.length);
		for (var i=0; i<Math.min(inputs.length,5); i++) {
			r.push('  ['+i+'] <'+inputs[i].tagName+'> name='+(inputs[i].name||'')+' role='+(inputs[i].getAttribute('role')||''));
		}

		// 检查是否有同意页/cookie弹窗
		var body = document.body.textContent;
		r.push('page contains "同意": '+(body.indexOf('同意')>=0));
		r.push('page contains "Accept all": '+(body.indexOf('Accept all')>=0));
		r.push('page contains "Before you continue": '+(body.indexOf('Before you continue')>=0));
		r.push('page contains "consent.google": '+(window.location.href.indexOf('consent')>=0));

		// 检查搜索按钮
		var submitButtons = document.querySelectorAll('input[type="submit"], button[type="submit"], button[aria-label*="search" i], button[aria-label*="Search" i], button[aria-label*="搜索" i]');
		r.push('submit/search buttons: '+submitButtons.length);
		for (var j=0; j<Math.min(submitButtons.length, 5); j++) {
			var b = submitButtons[j];
			r.push('  ['+j+'] <'+b.tagName+'> text="'+b.textContent.trim().substring(0,30)+'" aria-label="'+(b.getAttribute('aria-label')||'')+'"');
		}

		return r.join('\n');
	})()`, &pageInfo))
	fmt.Println(pageInfo)

	// 3. 如果有同意页，先处理
	var needsConsent string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		if (window.location.href.indexOf('consent') >= 0) {
			return 'CONSENT_PAGE: '+window.location.href.substring(0,100);
		}

		// 检查页面上是否有同意按钮
		var buttons = document.querySelectorAll('button');
		for (var i=0; i<buttons.length; i++) {
			var t = buttons[i].textContent.trim();
			if (t === 'Accept all' || t === 'I agree' || t.indexOf('同意')>=0 || t.indexOf('接受')>=0) {
				return 'CONSENT_BTN: <'+buttons[i].tagName+'> text="'+t+'"';
			}
		}
		return 'No consent dialog detected';
	})()`, &needsConsent))
	fmt.Println("\n同意页检查: " + needsConsent)

	// 4. 尝试搜索
	fmt.Println("\n===== 尝试搜索 =====")
	inputSelector := `textarea[name="q"]`

	// 先检查 textarea 是否可见
	var visible string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var ta = document.querySelector('textarea[name="q"]');
		if (!ta) return 'NOT FOUND';
		return 'visible='+!!ta.offsetParent+' rect='+JSON.stringify(ta.getBoundingClientRect());
	})()`, &visible))
	fmt.Println("textarea状态: " + visible)

	// 尝试用 Focus + SendKeys + Submit
	err := chromedp.Run(ctx,
		chromedp.Focus(inputSelector),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.SendKeys(inputSelector, "golang tutorial"),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Submit(inputSelector),
	)
	if err != nil {
		fmt.Printf("Submit 失败: %v\n", err)
	}

	// 等待页面变化
	time.Sleep(5 * time.Second)

	var finalURL string
	chromedp.Run(ctx, chromedp.Evaluate(`window.location.href`, &finalURL))
	fmt.Printf("最终URL: %s\n", finalURL[:120])

	var finalRSO string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		return rso ? 'FOUND:'+rso.children.length+' children' : 'NOT FOUND';
	})()`, &finalRSO))
	fmt.Println("#rso: " + finalRSO)

	fmt.Println("\n===== 完成 =====")
}
