// +build ignore

// 临时诊断工具：检查 Google 搜索结果页的 DOM 结构，验证 clickResultJS 的正确性。
//
// 运行方式：
//   go run probe/diag_search_results.go
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
	targets, err := chromedp.Targets(tmpCtx)
	if err != nil {
		log.Fatal("获取 targets 失败:", err)
	}

	fmt.Println("可用 target 列表:")
	for i, t := range targets {
		fmt.Printf("  [%d] type=%s title=%s\n", i, t.Type, t.Title)
	}

	var ctx context.Context
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, "google.com/search") {
			fmt.Printf("\n复用已有搜索页: %s\n", t.URL)
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		log.Fatal("没有找到 Google 搜索页，请先在浏览器中搜索一次")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 确认页面状态
	var title string
	chromedp.Run(ctx, chromedp.Title(&title))
	fmt.Printf("页面标题: %s\n\n", title)

	// ===== 步骤 1：检查 #rso 是否存在 =====
	fmt.Println("===== 步骤 1：#rso 容器检查 =====")
	var rsoCheck string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NOT FOUND';
		return 'FOUND: tag=' + rso.tagName + ' children=' + rso.children.length;
	})()`, &rsoCheck))
	fmt.Println("  " + rsoCheck)

	// ===== 步骤 2：检查 #rso 的直接子元素结构 =====
	fmt.Println("\n===== 步骤 2：#rso 直接子元素（前15个）=====")
	var children string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NO #rso';
		var children = rso.children;
		var r = [];
		var limit = Math.min(children.length, 15);
		for (var i = 0; i < limit; i++) {
			var c = children[i];
			var info = '['+i+'] <'+c.tagName.toLowerCase()+'>';
			if (c.id) info += ' id="'+c.id+'"';
			if (c.className) info += ' class="'+c.className.substring(0,60)+'"';
			// 检查是否包含 h3
			var h3 = c.querySelector('h3');
			if (h3) info += ' HAS_H3="'+h3.textContent.trim().substring(0,60)+'"';
			// 检查是否包含链接
			var a = c.querySelector('a');
			if (a && a.href) info += ' HAS_A';
			r.push(info);
		}
		r.push('Total children: ' + children.length);
		return r.join('\\n');
	})()`, &children))
	fmt.Println(children)

	// ===== 步骤 3：检查搜索结果中每个 h3 的包装结构 =====
	fmt.Println("\n===== 步骤 3：每个搜索结果的 DOM 结构（前5个）=====")
	var h3detail string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NO #rso';
		var h3s = rso.querySelectorAll('h3');
		var r = [];
		var limit = Math.min(h3s.length, 5);
		for (var i = 0; i < limit; i++) {
			r.push('--- h3['+i+'] ---');
			r.push('  text: "' + h3s[i].textContent.trim().substring(0,80) + '"');
			// 查找最近的 a 标签
			var a = h3s[i].closest('a');
			if (a) {
				r.push('  closest("a"): FOUND');
				r.push('    href: ' + (a.href||'null').substring(0,100));
				r.push('    tagName: ' + a.tagName);
				r.push('    h3是否是a的直接子元素: ' + (h3s[i].parentElement === a));
				r.push('    a的直接子元素数量: ' + a.children.length);
				// 列出 a 的所有直接子元素
				for (var j = 0; j < a.children.length; j++) {
					r.push('    a.children['+j+']: <'+a.children[j].tagName.toLowerCase()+'>');
				}
			} else {
				r.push('  closest("a"): NOT FOUND');
				// 往上找 5 层看结构
				var p = h3s[i].parentElement;
				for (var level = 1; level <= 5 && p; level++) {
					r.push('  parent['+level+']: <'+p.tagName.toLowerCase()+'> class="'+(p.className||'').substring(0,60)+'"');
					if (p.tagName === 'A') {
						r.push('    >>> FOUND A at parent['+level+']: href='+(p.href||'null').substring(0,100));
					}
					p = p.parentElement;
				}
			}
		}
		r.push('Total h3 in #rso: ' + h3s.length);
		return r.join('\\n');
	})()`, &h3detail))
	fmt.Println(h3detail)

	// ===== 步骤 4：检查 "People also ask" 区块 =====
	fmt.Println("\n===== 步骤 4：People also ask / 相关问题 检查 =====")
	var paa string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NO #rso';
		var h3s = rso.querySelectorAll('h3');
		var r = [];
		var paaCount = 0;
		for (var i = 0; i < h3s.length; i++) {
			var el = h3s[i];
			var block = el.parentElement;
			while (block && block.parentElement !== rso) {
				block = block.parentElement;
			}
			if (block) {
				var text = block.textContent;
				if (text.indexOf('People also ask') !== -1 || text.indexOf('相关问题') !== -1) {
					paaCount++;
					if (paaCount <= 3) {
						r.push('PAA h3['+i+']: "'+el.textContent.trim().substring(0,60)+'"');
						r.push('  block: <'+block.tagName.toLowerCase()+'> class="'+(block.className||'').substring(0,60)+'"');
					}
				}
			}
		}
		r.push('Total h3 in PAA blocks: ' + paaCount);
		r.push('Total h3 in #rso: ' + h3s.length);
		r.push('Usable h3 (non-PAA): ' + (h3s.length - paaCount));
		return r.join('\\n');
	})()`, &paa))
	fmt.Println(paa)

	// ===== 步骤 5：测试 clickResultJS 的定位逻辑（不实际点击）=====
	fmt.Println("\n===== 步骤 5：模拟 clickResultJS 定位逻辑（不点击）=====")
	var simulate string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		if (!container) return 'NO #rso';
		var h3s = container.querySelectorAll('h3');
		var results = [];
		var seen = {};
		for (var i = 0; i < h3s.length; i++) {
			// 跳过 PAA
			var block = h3s[i].parentElement;
			while (block && block.parentElement !== container) {
				block = block.parentElement;
			}
			if (block) {
				var t = block.textContent;
				if (t.indexOf('People also ask') !== -1 || t.indexOf('相关问题') !== -1) continue;
			}
			var a = h3s[i].closest('a');
			if (!a) continue;
			var href = a.href;
			if (!href || seen[href]) continue;
			seen[href] = true;
			results.push({
				index: results.length,
				title: h3s[i].textContent.trim().substring(0,80),
				href: href.substring(0,100),
				clickable: typeof a.click === 'function' ? 'YES' : 'NO'
			});
		}
		return JSON.stringify({count: results.length, results: results}, null, 2);
	})()`, &simulate))
	fmt.Println(simulate)

	// ===== 步骤 6：测试实际点击第0个结果（可选，谨慎）=====
	fmt.Println("\n===== 步骤 6：click() 可用性检查 =====")
	var clickable string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		if (!container) return 'NO #rso';
		var h3s = container.querySelectorAll('h3');
		// 找第一个非 PAA 的 h3
		for (var i = 0; i < h3s.length; i++) {
			var a = h3s[i].closest('a');
			if (!a || !a.href) continue;
			var block = h3s[i].parentElement;
			while (block && block.parentElement !== container) block = block.parentElement;
			if (block) {
				var t = block.textContent;
				if (t.indexOf('People also ask') !== -1 || t.indexOf('相关问题') !== -1) continue;
			}
			// 检查 link 的各种属性
			return JSON.stringify({
				tagName: a.tagName,
				hasHref: !!a.href,
				hrefPreview: a.href.substring(0,120),
				hasClick: typeof a.click === 'function',
				target: a.target || '(none)',
				rel: a.rel || '(none)',
				offsetWidth: a.offsetWidth,
				offsetHeight: a.offsetHeight,
				isVisible: a.offsetWidth > 0 && a.offsetHeight > 0
			}, null, 2);
		}
		return 'NO valid result link found';
	})()`, &clickable))
	fmt.Println(clickable)

	fmt.Println("\n===== 诊断完成 =====")
}
