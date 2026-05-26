// +build ignore

// 深度探测：分析当前搜索页面的 DOM 结构，验证 clickResultJS 正确性。
//
// 运行方式：
//   go run probe/diag_deep.go
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
			fmt.Printf("复用搜索页: %s...\n", t.URL[:80])
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		log.Fatal("无 Google 搜索页，请先在浏览器中搜索")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var title string
	chromedp.Run(ctx, chromedp.Title(&title))
	fmt.Printf("页面: %s\n\n", title)

	// ==== 1. #rso 每个子元素概览 ====
	fmt.Println("===== 1. #rso 子元素概览（有h3的） =====")
	var overview string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NO #rso';
		var r = [];
		for (var i = 0; i < rso.children.length; i++) {
			var c = rso.children[i];
			var h3s = c.querySelectorAll('h3');
			if (h3s.length === 0) continue;

			// 检测 PAA 标记
			var text = c.textContent.substring(0, 200);
			var flags = '';
			if (text.indexOf('People also ask') >= 0) flags += ' PAA_EN';
			if (text.indexOf('相关问题') >= 0) flags += ' PAA_CN';
			if (text.indexOf('Video') >= 0 && c.querySelector('video')) flags += ' VIDEO';
			if (text.indexOf('Images for') >= 0) flags += ' IMAGES';

			r.push('['+i+'] <'+c.tagName+'> class="'+(c.className||'(empty)').substring(0,30)+'" h3='+h3s.length+flags);
			for (var j = 0; j < Math.min(h3s.length, 3); j++) {
				var a = h3s[j].closest('a');
				r.push('   h3['+j+']: "'+h3s[j].textContent.trim().substring(0,60)+'" a='+(a&&a.href?'Y':'N'));
			}
			if (h3s.length > 3) r.push('   ... +'+(h3s.length-3)+' more');
		}
		r.push('totalChildren: '+rso.children.length);
		return r.join('\n');
	})()`, &overview))
	fmt.Println(overview)

	// ==== 2. PAA 文本节点位置 ====
	fmt.Println("\n===== 2. PAA 标记位置（全页面搜索） =====")
	var paa string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r = [];
		var walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
		var node;
		while (node = walker.nextNode()) {
			var t = node.textContent.trim();
			if (t === 'People also ask' || t.indexOf('相关问题') === 0) {
				// 检查它的祖先是否在 #rso 内部
				var insideRSO = false;
				var p = node.parentElement;
				var depth = 0;
				while (p && p !== document.body) {
					depth++;
					if (p.id === 'rso') { insideRSO = true; break; }
					p = p.parentElement;
				}
				r.push('text="'+t+'" insideRSO='+insideRSO+' depthFromBody='+depth);
			}
		}
		return r.join('\n') || '(未找到PAA文本标记)';
	})()`, &paa))
	fmt.Println(paa)

	// ==== 3. 用 clickResultJS 逻辑模拟（含PAA排除）====
	fmt.Println("\n===== 3. clickResultJS 模拟定位 =====")
	var simulate string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		if (!container) return JSON.stringify({error:'NO #rso'});

		function isPAA(el, rso) {
			var block = el.parentElement;
			while (block && block.parentElement !== rso) block = block.parentElement;
			if (!block) return false;
			var text = block.textContent;
			return text.indexOf('People also ask') !== -1 || text.indexOf('相关问题') !== -1;
		}

		var h3s = container.querySelectorAll('h3');
		var results = [], skipped = [], seen = {};
		for (var i = 0; i < h3s.length; i++) {
			if (isPAA(h3s[i], container)) { skipped.push('PAA:'+h3s[i].textContent.trim().substring(0,40)); continue; }
			var a = h3s[i].closest('a');
			if (!a) { skipped.push('NO_A:'+h3s[i].textContent.trim().substring(0,40)); continue; }
			if (!a.href) { skipped.push('NO_HREF'); continue; }
			if (seen[a.href]) { skipped.push('DUP:'+h3s[i].textContent.trim().substring(0,40)); continue; }
			seen[a.href] = true;
			results.push((results.length)+': '+h3s[i].textContent.trim().substring(0,60));
		}
		return JSON.stringify({totalH3:h3s.length, ok:results.length, skipped:skipped.length, results:results.slice(0,15), skippedReasons:skipped.slice(0,10)},null,2);
	})()`, &simulate))
	fmt.Println(simulate)

	// ==== 4. 检查不在 a 标签中的异常 h3 ====
	fmt.Println("\n===== 4. 异常h3检查 =====")
	var anomalies string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NO #rso';
		var h3s = rso.querySelectorAll('h3');
		var r = [];
		for (var i = 0; i < h3s.length; i++) {
			var a = h3s[i].closest('a');
			if (!a || !a.href) {
				// 往上走5层
				var chain = [];
				var p = h3s[i].parentElement;
				for (var j=0;j<5&&p&&p!==rso;j++) {
					chain.push('<'+p.tagName.toLowerCase()+'>');
					p = p.parentElement;
				}
				r.push('h3['+i+'] "'+h3s[i].textContent.trim().substring(0,50)+'" chain: '+chain.join(' > '));
			}
		}
		return r.join('\n') || '(所有h3都在a标签中)';
	})()`, &anomalies))
	fmt.Println(anomalies)

	fmt.Println("\n===== 完成 =====")
}
