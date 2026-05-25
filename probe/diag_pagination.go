// +build ignore

// 临时诊断工具 v3：使用与 search.go 相同的连接模式，附加到已有搜索页检查分页 DOM。
//
// 运行方式：
//   go run diag_pagination.go
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
	var err error

	// 与 search.go 完全相同的连接方式
	allocatorCtx, _ := chromedp.NewRemoteAllocator(
		context.Background(),
		"ws://127.0.0.1:9224",
	)

	tmpCtx, _ := chromedp.NewContext(allocatorCtx)

	targets, err := chromedp.Targets(tmpCtx)
	if err != nil {
		log.Fatal("获取 targets 失败:", err)
	}

	fmt.Println("可用 target 列表:")
	for i, t := range targets {
		fmt.Printf("  [%d] type=%s id=%s title=%s\n", i, t.Type, t.TargetID, t.Title)
	}

	// 找到 golang 搜索结果页
	var ctx context.Context
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, "google.com/search") {
			fmt.Printf("\n复用已有搜索页: %s\n", t.URL[:80])
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		log.Fatal("没有找到 Google 搜索页，请先在浏览器中搜索一次")
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// ---- 确认页面状态 ----
	var title string
	if err := chromedp.Run(ctx, chromedp.Title(&title)); err != nil {
		log.Fatalf("获取标题失败: %v", err)
	}
	fmt.Printf("页面标题: %s\n", title)

	// ---- 步骤 1：检查分页关键元素 ----
	fmt.Println("\n===== 步骤 1：关键分页元素 =====")
	var info string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r = [];
		r.push('pnnext: ' + (document.getElementById('pnnext')?'FOUND':'NOT FOUND'));
		r.push('pnprev: ' + (document.getElementById('pnprev')?'FOUND':'NOT FOUND'));
		r.push('#foot: ' + (document.getElementById('foot')?'FOUND':'NOT FOUND'));
		r.push('#navcnt: ' + (document.getElementById('navcnt')?'FOUND':'NOT FOUND'));
		r.push('#botstuff: ' + (document.getElementById('botstuff')?'FOUND':'NOT FOUND'));
		// 搜索所有 table，看哪个包含分页链接
		var tables = document.querySelectorAll('table');
		for (var i=0; i<tables.length; i++) {
			var links = tables[i].querySelectorAll('a');
			if (links.length >= 5) {
				r.push('Table['+i+']: '+links.length+' links');
			}
		}
		return r.join('\\n');
	})()`, &info))
	fmt.Println(info)

	// ---- 步骤 2：分页链接详情 ----
	fmt.Println("\n===== 步骤 2：分页 a 标签详情 =====")
	var links string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r = [];
		var tables = document.querySelectorAll('table');
		for (var ti=0; ti<tables.length; ti++) {
			var as = tables[ti].querySelectorAll('a');
			if (as.length < 5) continue;
			r.push('=== Table['+ti+'] ('+as.length+' links) ===');
			for (var i=0; i<as.length; i++) {
				var a=as[i];
				var line = '['+i+']';
				line += ' text="'+a.textContent.trim()+'"';
				line += ' class="'+(a.className||'')+'"';
				line += ' id="'+(a.id||'')+'"';
				var al = a.getAttribute('aria-label');
				if (al) line += ' aria-label="'+al+'"';
				var role = a.getAttribute('role');
				if (role) line += ' role="'+role+'"';
				var href = a.getAttribute('href');
				if (href) {
					if (href.length > 100) href = href.substring(0,100)+'...';
					line += ' href="'+href+'"';
				}
				r.push(line);
			}
		}
		return r.join('\\n');
	})()`, &links))
	fmt.Println(links)

	// ---- 步骤 3：aria-label 选择器测试 ----
	fmt.Println("\n===== 步骤 3：aria-label='Page N' 选择器测试 =====")
	for page := 2; page <= 5; page++ {
		var r string
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(
			`(function(){
			var a=document.querySelector('a[aria-label="Page %d"]');
			return a ? 'FOUND: text="'+a.textContent.trim()+'"' : 'NOT FOUND';
		})()`, page), &r))
		fmt.Printf("  Page %d: %s\n", page, r)
	}

	// ---- 步骤 4：textContent 精确匹配测试 ----
	fmt.Println("\n===== 步骤 4：textContent === 页码 测试 =====")
	var textMatch string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r=[];
		var tbls = document.querySelectorAll('table');
		for (var ti=0; ti<tbls.length; ti++) {
			var as=tbls[ti].querySelectorAll('a');
			if (as.length<5) continue;
			for (var i=0; i<as.length; i++) {
				var t=as[i].textContent.trim();
				if (t.match(/^\\d+$/)) {
					r.push('Table['+ti+']['+i+']: text="'+t+'"');
				}
			}
		}
		return r.join('\\n')||'NO pure-number text links';
	})()`, &textMatch))
	fmt.Println(textMatch)

	// ---- 步骤 5：clickPaginationJS 诊断版 ----
	fmt.Println("\n===== 步骤 5：clickPaginationJS 定位诊断 =====")
	for page := 2; page <= 5; page++ {
		var r string
		chromedp.Run(ctx, chromedp.Evaluate(diagJS(page), &r))
		fmt.Printf("  Page %d: %s\n", page, r)
	}

	// ---- 步骤 6：foot 区域 HTML ----
	fmt.Println("\n===== 步骤 6：#foot 内完整 HTML =====")
	var html string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var el = document.getElementById('foot') || document.getElementById('navcnt');
		if (!el) return 'NOT FOUND';
		return el.innerHTML.substring(0, 4000);
	})()`, &html))
	fmt.Println(html)

	// ---- 步骤 7：边界测试 ----
	fmt.Println("\n===== 步骤 7：边界测试 =====")
	// Page 1
	var r7 string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var a = document.querySelector('a[aria-label="Page 1"]');
		if (a) return 'FOUND: text="'+a.textContent.trim()+'" class="'+a.className+'"';
		return 'NOT FOUND (当前在第1页，Page 1通常是当前页标记，不是链接)';
	})()`, &r7))
	fmt.Println("Page 1:", r7)

	// Page 10, 11
	for _, page := range []int{10, 11} {
		var rr string
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(function(){
			var a=document.querySelector('a[aria-label="Page %d"]');
			return a ? 'FOUND: text="'+a.textContent.trim()+'"' : 'NOT FOUND';
		})()`, page), &rr))
		fmt.Printf("Page %d: %s\n", page, rr)
	}

	// 分页 table 容器层级
	var structure string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r=[];
		var tbls=document.querySelectorAll('table');
		for (var i=0;i<tbls.length;i++) {
			if (tbls[i].querySelectorAll('a').length>=5) {
				var p=tbls[i].parentElement;
				r.push('Table.parent: <'+p.tagName.toLowerCase()+'> id='+(p.id||'""')+' class='+(p.className||'""'));
				var gp=p.parentElement;
				r.push('Table.grandparent: <'+gp.tagName.toLowerCase()+'> id='+(gp.id||'""')+' class='+(gp.className||'""'));
			}
		}
		return r.join('\\n');
	})()`, &structure))
	fmt.Println("\n分页 table 容器层级:")
	fmt.Println(structure)

	// 当前页标记
	var current string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var r=[];
		var tbls=document.querySelectorAll('table');
		for (var i=0;i<tbls.length;i++) {
			var tds=tbls[i].querySelectorAll('td,span,b,strong');
			for (var j=0;j<tds.length;j++) {
				var t=tds[j].textContent.trim();
				if (t==='1' || t==='2') {
					r.push('['+tds[j].tagName+'] text="'+t+'" class="'+tds[j].className+'"');
				}
			}
		}
		return r.join('\\n')||'无带1或2文本的td/span/b/strong';
	})()`, &current))
	fmt.Println("\n当前页标记 (td/span/b/strong 含1或2):")
	fmt.Println(current)

	fmt.Println("\n===== 诊断完成 =====")
}

// diagJS 返回诊断信息而不是实际点击
func diagJS(pageNum int) string {
	return fmt.Sprintf(`(function(){
		// 策略1：aria-label
		var a = document.querySelector('a[aria-label="Page %d"]');
		if (a) return 'S1(aria): HIT text="'+a.textContent.trim()+'"';

		// 策略2：textContent 精确匹配
		var all = document.querySelectorAll('a');
		for (var i=0; i<all.length; i++) {
			if (all[i].textContent.trim() === '%d') {
				return 'S2(text): HIT a['+i+'] text="'+all[i].textContent.trim()+'"';
			}
		}
		return 'MISS';
	})()`, pageNum, pageNum)
}

func init() {
	_ = strings.TrimSpace
	_ = fmt.Sprintf
	_ = log.Println
}
