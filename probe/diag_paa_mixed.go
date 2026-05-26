// +build ignore

// 深入探测 PAA 混合容器结构：检查 Google 将正常结果和 PAA 放在同一 #rso 子 div 中时，
// PAA 的标记方式，以便改进 isInsidePeopleAlsoAsk 的判断逻辑。
//
// 运行方式：
//   go run probe/diag_paa_mixed.go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var _ = strings.TrimSpace

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

	// 这次用一个会产生 PAA 的搜索词
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// 先导航到 google 并搜索
	var rsoCheck string
	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://google.com`),
		chromedp.WaitVisible(`textarea[name="q"]`),
		chromedp.SendKeys(`textarea[name="q"]`, "python tutorial"+"\n"),
		chromedp.Sleep(4*time.Second),
		chromedp.Evaluate(`(function(){
			var rso = document.getElementById('rso');
			return rso ? 'FOUND:'+rso.children.length : 'NOT_FOUND';
		})()`, &rsoCheck),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("#rso: %s\n\n", rsoCheck)

	// 1. 详细分析 child[3]（PAA 混合容器）的层级结构
	fmt.Println("===== 步骤1: PAA混合容器详细结构 =====")
	var structure string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		var paablock = rso.children[3];
		if (!paablock) return 'NO child[3]';

		var r = [];
		r.push('=== child[3] 宏观结构 ===');
		r.push('tagName: ' + paablock.tagName);
		r.push('className: "' + paablock.className + '"');
		r.push('id: "' + paablock.id + '"');
		r.push('directChildren: ' + paablock.children.length);

		// 列出直接子元素
		for (var i = 0; i < Math.min(paablock.children.length, 8); i++) {
			var c = paablock.children[i];
			var info = '  ['+i+'] <'+c.tagName.toLowerCase()+'>';
			if (c.className) info += ' class="'+c.className.substring(0,60)+'"';
			if (c.id) info += ' id="'+c.id+'"';
			// 是否包含 h3
			var h3c = c.querySelectorAll('h3').length;
			if (h3c > 0) info += ' h3_count='+h3c;
			// 是否包含文本 "People also ask" / "相关问题"
			var txt = c.textContent.trim().substring(0, 80);
			if (txt.indexOf('People also ask') !== -1 || txt.indexOf('相关') !== -1) {
				info += ' PAA_TITLE';
			}
			r.push(info);
		}

		// 搜索文本标记
		r.push('');
		r.push('=== 搜索 "People also ask" / "相关问题" 文本标记 ===');
		function findTextMarker(el, depth) {
			if (depth > 6) return;
			for (var i = 0; i < el.children.length; i++) {
				var c = el.children[i];
				if (c.children.length === 0) {
					var t = c.textContent.trim();
					if (t === 'People also ask' || t.indexOf('相关问题') === 0) {
						r.push('FOUND at depth='+depth+': <'+c.tagName.toLowerCase()+'> class="'+(c.className||'')+'" text="'+t+'"');
						// 打印父元素信息
						var p = c.parentElement;
						r.push('  parent: <'+p.tagName.toLowerCase()+'> class="'+(p.className||'').substring(0,60)+'" id="'+(p.id||'')+'"');
					}
				}
				findTextMarker(c, depth+1);
			}
		}
		findTextMarker(paablock, 0);

		return r.join('\\n');
	})()`, &structure))
	fmt.Println(structure)

	// 2. 检查每个 h3 与 PAA 标记的相对位置关系
	fmt.Println("\n===== 步骤2: 每个h3与PAA标记的位置关系 =====")
	var h3Positions string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		var paablock = rso.children[3];
		if (!paablock) return 'NO child[3]';

		// 找到 "People also ask" / "相关问题" 文本节点所在的祖先元素
		var paaAncestor = null;
		function findPAAAncestor(el) {
			for (var i = 0; i < el.children.length; i++) {
				var c = el.children[i];
				if (c.children.length === 0) {
					var t = c.textContent.trim();
					if (t === 'People also ask' || t.indexOf('相关问题') === 0) {
						paaAncestor = c;
						return;
					}
				}
				findPAAAncestor(c);
			}
		}
		findPAAAncestor(paablock);

		// 找到包含 PAA 标题的 section/div
		var paaContainer = paaAncestor;
		while (paaContainer && !paaContainer.querySelector('h3')) {
			paaContainer = paaContainer.parentElement;
		}
		// 再往上找几层
		if (paaContainer) paaContainer = paaContainer.parentElement;

		var r = [];
		r.push('PAA标记最近祖先: ' + (paaAncestor ? '<'+paaAncestor.tagName+'>' : 'NOT FOUND'));
		if (paaContainer) {
			r.push('PAA容器: <'+paaContainer.tagName.toLowerCase()+'> class="'+(paaContainer.className||'').substring(0,60)+'"');
		}

		// 检查每个h3
		var h3s = paablock.querySelectorAll('h3');
		r.push('');
		r.push('共'+h3s.length+'个h3:');
		for (var i = 0; i < Math.min(h3s.length, 12); i++) {
			var h3 = h3s[i];
			var title = h3.textContent.trim().substring(0, 60);
			var isInsidePAA = paaAncestor ? h3.compareDocumentPosition(paaAncestor) : -1;
			// compareDocumentPosition: 如果 h3 在 paaAncestor 之后则是 2 (DOCUMENT_POSITION_FOLLOWING)，之前是4

			// 用 contains 判断
			var insidePAA = false;
			if (paaContainer) {
				// 往上走到 direct parent
				var p = h3;
				while (p && p !== paablock) {
					if (p === paaContainer) {
						insidePAA = true;
						break;
					}
					p = p.parentElement;
				}
			}

			r.push('h3['+i+']: insidePAA='+insidePAA+' title="'+title+'"');
		}
		return r.join('\\n');
	})()`, &h3Positions))
	fmt.Println(h3Positions)

	// 3. 改进版 PAA 检测逻辑测试
	fmt.Println("\n===== 步骤3: 测试改进的PAA检测逻辑 =====")
	var improved string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		var results = [];
		var seen = {};
		var allH3s = rso.querySelectorAll('h3');

		for (var i = 0; i < allH3s.length; i++) {
			var ok = !isInsidePAA(allH3s[i], rso);
			if (ok) {
				var a = allH3s[i].closest('a');
				if (a && a.href && !seen[a.href]) {
					seen[a.href] = true;
					results.push({index: results.length, title: allH3s[i].textContent.trim().substring(0, 60)});
				}
			}
		}
		return JSON.stringify({count: results.length, results: results}, null, 2);

		// 改进的 PAA 检测：找到 PAA 标题所在的 g-expandable-container 或类似容器，
		// 只排除该容器内部的 h3，不影响同一级 div 中的正常结果
		function isInsidePAA(el, rso) {
			// 找到 #rso 的直接子元素
			var block = el.parentElement;
			while (block && block.parentElement !== rso) {
				block = block.parentElement;
			}
			if (!block) return false;

			// 在这个 block 内查找 "People also ask" / "相关问题" 标题
			// 如果找到了，需要区分 h3 是在 PAA 区域内还是区域外
			// 策略: 找到包含 PAA 标题的最内层容器，只排除该容器内的 h3
			var paaSection = findPAASection(block);
			if (!paaSection) return false;
			return paaSection.contains(el);
		}

		function findPAASection(block) {
			// 在 block 内搜索 "People also ask" / "相关问题" 文本
			// 找到包含它的、有实际内容边界意义的祖先
			var walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
			var node;
			while (node = walker.nextNode()) {
				var t = node.textContent.trim();
				if (t === 'People also ask' || t.indexOf('相关问题') === 0) {
					// 从这个文本节点往上找，找到有意义的边界容器
					var p = node.parentElement;
					while (p && p !== block) {
						// 如果这个元素包含多个 h3（除了当前已遍历的），说明它是 PAA 容器
						var tag = p.tagName.toLowerCase();
						// g-expandable-container, div with many h3 children, etc.
						if (p.querySelectorAll('h3').length >= 2) {
							return p;
						}
						p = p.parentElement;
					}
					return node.parentElement; // fallback
				}
			}
			return null;
		}
	})()`, &improved))
	fmt.Println(improved)

	fmt.Println("\n===== 诊断完成 =====")
}
