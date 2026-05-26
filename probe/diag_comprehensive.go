// +build ignore

// 全面探测工具：测试多种搜索词和多个页面，发现 Google 搜索结果页中各种未知的 DOM 结构。
//
// 运行方式：
//   go run probe/diag_comprehensive.go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var searchQueries = []string{
	"python tutorial",     // 通用搜索
	"apple stock",         // 可能触发知识面板/股票卡片
	"weather beijing",     // 可能触发天气卡片
	"golang vs rust",      // 可能触发对比/讨论类
	"pizza near me",       // 可能触发地图/本地结果
}

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

	for _, query := range searchQueries {
		fmt.Printf("\n========================================\n")
		fmt.Printf("搜索词: %s\n", query)
		fmt.Printf("========================================\n")
		analyzeSearch(ctx, query)
	}
	fmt.Println("\n===== 全部诊断完成 =====")
}

func analyzeSearch(ctx context.Context, query string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var rsoCheck string
	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://google.com`),
		chromedp.WaitVisible(`textarea[name="q"]`),
		chromedp.SendKeys(`textarea[name="q"]`, query+"\n"),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`(function(){
			var rso = document.getElementById('rso');
			if (!rso) return 'NOT_FOUND';
			return 'FOUND:' + rso.children.length + ' children';
		})()`, &rsoCheck),
	)
	if err != nil {
		fmt.Printf("  错误: %v\n", err)
		return
	}
	fmt.Printf("  #rso: %s\n", rsoCheck)

	if strings.Contains(rsoCheck, "NOT_FOUND") {
		fmt.Println("  跳过（无 #rso）")
		return
	}

	// 1. 扫描 #rso 的所有直接子元素类型
	var childrenTypes string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		var r = [];
		for (var i = 0; i < rso.children.length; i++) {
			var c = rso.children[i];
			var h3s = c.querySelectorAll('h3');
			var hasH3 = h3s.length > 0;
			var hasA = c.querySelector('a') !== null;
			var info = 'child['+i+'] <'+c.tagName.toLowerCase()+'>';
			if (c.id) info += ' id='+c.id;
			info += ' class="'+(c.className||'').substring(0,40)+'"';
			info += ' h3='+h3s.length;
			if (hasH3) {
				info += ' h3text="'+h3s[0].textContent.trim().substring(0,40)+'"';
				var a = h3s[0].closest('a');
				info += ' h3_in_a='+(a?'Y':'N');
			}
			r.push(info);
		}
		return r.join('\\n');
	})()`, &childrenTypes))
	fmt.Printf("  #rso 子元素类型:\\n%s\n", indent(childrenTypes))

	// 2. 检查特殊区块
	var special string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		var r = [];
		for (var i = 0; i < rso.children.length; i++) {
			var c = rso.children[i];
			var text = c.textContent;
			var flags = [];
			if (text.indexOf('People also ask') !== -1) flags.push('PAA');
			if (text.indexOf('相关问题') !== -1) flags.push('PAA_CN');
			if (text.indexOf('Featured snippet') !== -1) flags.push('FEATURED');
			if (text.indexOf('Videos') !== -1 && c.querySelectorAll('video').length>0) flags.push('VIDEOS');
			if (text.indexOf('Images for') !== -1) flags.push('IMAGES');
			if (c.querySelector('table') && c.querySelectorAll('td').length>5) flags.push('TABLE');
			if (c.querySelector('ul') || c.querySelector('ol')) flags.push('LIST');
			if (c.querySelector('g-img') || c.querySelector('img[src]')) flags.push('HAS_IMG');
			if (c.className && c.className.indexOf('ULSxyf') !== -1) flags.push('MAP_BLOCK');
			if (flags.length > 0) {
				r.push('child['+i+']: ' + flags.join(', '));
			}
		}
		// 也检查 #rso 外部的元素
		var main = document.querySelector('#main') || document.querySelector('#search');
		if (main) {
			var paa = main.querySelector('[data-hveid]');
			// 检查全局 PAA
			var allH3 = document.querySelectorAll('h3');
			var outsideH3 = 0;
			for (var j=0; j<allH3.length; j++) {
				if (!rso.contains(allH3[j])) outsideH3++;
			}
			if (outsideH3 > 0) r.push('OUTSIDE_RSO_H3: ' + outsideH3);
		}
		return r.join('\\n') || '(无特殊区块)';
	})()`, &special))
	fmt.Printf("  特殊区块:\\n%s\n", indent(special))

	// 3. 测试 clickResultJS 的 extract 逻辑一致性
	var consistency string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		var h3s = container.querySelectorAll('h3');
		var results = [];
		var seen = {};

		function isInsidePeopleAlsoAsk(el, rso) {
			var block = el.parentElement;
			while (block && block.parentElement !== rso) {
				block = block.parentElement;
			}
			if (!block) return false;
			var text = block.textContent;
			return text.indexOf('People also ask') !== -1 || text.indexOf('相关问题') !== -1;
		}

		for (var i = 0; i < h3s.length; i++) {
			if (isInsidePeopleAlsoAsk(h3s[i], container)) {
				results.push({index: results.length, status: 'SKIP_PAA', title: h3s[i].textContent.trim().substring(0,40)});
				continue;
			}
			var a = h3s[i].closest('a');
			if (!a) {
				results.push({index: results.length, status: 'NO_A', title: h3s[i].textContent.trim().substring(0,40)});
				continue;
			}
			var href = a.href;
			if (!href) {
				results.push({index: results.length, status: 'NO_HREF', title: h3s[i].textContent.trim().substring(0,40)});
				continue;
			}
			if (seen[href]) {
				results.push({index: results.length, status: 'DUP_HREF', title: h3s[i].textContent.trim().substring(0,40)});
				continue;
			}
			seen[href] = true;
			results.push({index: results.length, status: 'OK', title: h3s[i].textContent.trim().substring(0,60), href: href.substring(0,80)});
		}
		return JSON.stringify({total_h3: h3s.length, usable: results.filter(function(r){return r.status==='OK'}).length, details: results}, null, 2);
	})()`, &consistency))
	fmt.Printf("  extract逻辑一致性:\\n%s\n", indent(consistency))
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

func init() {
	_ = fmt.Sprintf
	_ = log.Println
	_ = time.Now
}
