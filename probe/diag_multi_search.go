// +build ignore

// 多搜索词全面探测：Navigating + Searching 完整流程，
// 测试不同搜索词下 DOM 结构和 clickResultJS 的正确性。
//
// 运行方式：
//   go run probe/diag_multi_search.go
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var queries = []string{
	"react vs vue",       // 通用技术搜索
	"docker tutorial",     // 技术教程
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

	for _, q := range queries {
		fmt.Printf("\n========================================\n")
		fmt.Printf("搜索: %s\n", q)
		fmt.Printf("========================================\n")
		analyzeOne(ctx, q)
	}
	fmt.Println("\n===== 全部完成 =====")
}

func analyzeOne(parentCtx context.Context, query string) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	inputSelector := `textarea[name="q"]`

	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://google.com`),
		chromedp.WaitVisible(inputSelector),
		chromedp.Focus(inputSelector),
		chromedp.SendKeys(inputSelector, query+"\n"),
	)
	if err != nil {
		fmt.Printf("  导航失败: %v\n", err)
		return
	}

	// 等待搜索结果加载
	waitForRSO(ctx)

	var title string
	chromedp.Run(ctx, chromedp.Title(&title))
	fmt.Printf("  标题: %s\n", title)

	// 1. #rso 子元素分析
	var overview string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return JSON.stringify({error:'NO #rso'});
		var r = [];
		for (var i = 0; i < rso.children.length; i++) {
			var c = rso.children[i];
			var h3s = c.querySelectorAll('h3');
			if (h3s.length === 0) continue;
			var text = c.textContent.substring(0, 200);
			var flags = '';
			if (text.indexOf('People also ask') >= 0) flags += ' PAA_EN';
			if (text.indexOf('相关问题') >= 0) flags += ' PAA_CN';
			r.push('['+i+'] <'+c.tagName+'> class="'+(c.className||'(empty)').substring(0,40)+'" h3='+h3s.length+flags);
			for (var j = 0; j < Math.min(h3s.length, 3); j++) {
				var a = h3s[j].closest('a');
				r.push('   "'+h3s[j].textContent.trim().substring(0,60)+'" a='+(a&&a.href?'Y':'N'));
			}
			if (h3s.length > 3) r.push('   ... +'+(h3s.length-3)+' more');
		}
		r.push('totalH3Containers: '+(r.length-1));
		return r.join('\n');
	})()`, &overview))
	fmt.Printf("  #rso 结构:\n%s\n", indent(overview))

	// 2. 异常 h3 检查
	var anomalies string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var rso = document.getElementById('rso');
		if (!rso) return 'NO_RSO';
		var h3s = rso.querySelectorAll('h3');
		var bad = [];
		for (var i = 0; i < h3s.length; i++) {
			var a = h3s[i].closest('a');
			if (!a || !a.href) {
				var chain = [];
				var p = h3s[i].parentElement;
				for (var j=0;j<4&&p&&p!==rso;j++) {
					chain.push('<'+p.tagName.toLowerCase()+'>');
					p = p.parentElement;
				}
				bad.push('h3['+i+'] "'+h3s[i].textContent.trim().substring(0,50)+'" path: '+chain.join('>'));
			}
		}
		return JSON.stringify({totalH3:h3s.length, badCount:bad.length, bad:bad},null,2);
	})()`, &anomalies))
	fmt.Printf("  异常h3: %s\n", anomalies)

	// 3. clickResultJS 模拟
	var simulate string
	chromedp.Run(ctx, chromedp.Evaluate(`(function(){
		var container = document.getElementById('rso');
		if (!container) return JSON.stringify({error:'NO_RSO'});

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
			if (!a) { skipped.push('NO_A'); continue; }
			if (!a.href) { skipped.push('NO_HREF'); continue; }
			if (seen[a.href]) { skipped.push('DUP:'+h3s[i].textContent.trim().substring(0,40)); continue; }
			seen[a.href] = true;
			results.push(results.length+': '+h3s[i].textContent.trim().substring(0,60));
		}
		return JSON.stringify({totalH3:h3s.length, ok:results.length, skipped:skipped.length, results:results, skippedReasons:skipped},null,2);
	})()`, &simulate))
	fmt.Printf("  clickResultJS模拟:\n%s\n", indent(simulate))
}

func waitForRSO(ctx context.Context) {
	const checkJs = `(function(){
		var rso = document.getElementById('rso');
		return rso !== null && rso.children.length > 0;
	})()`
	for {
		var ready bool
		if err := chromedp.Evaluate(checkJs, &ready).Do(ctx); err != nil {
			return
		}
		if ready {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Second):
	}
}

func indent(s string) string {
	var out strings.Builder
	for _, line := range strings.Split(s, "\n") {
		out.WriteString("    ")
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}
