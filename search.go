package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	searchText := flag.String("search_text", "", "要搜索的文本。会返回搜索结果，包含分页页码、每项结果。每项结果包含序号、标题。")
	toPagination := flag.String("to_pagination", "", "搜索结果是带分页的，这是页码，想看第几页就传几，支持1~10(实际取min(10,最大分页))。")
	getInfo := flag.String("get_info", "", "根据搜索结果的序号查看页面详细信息，会将html转换成markdown返回。")
	filter := flag.String("filter", "", "通过传入支持Go regexp2的正则表达式过滤页面详细信息markdown。（推荐，为用户节省token。）")

	_ = filter

	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
通常先传入search_text，拿到搜索结果之后可以决定查看其他分页或者查看详细信息。查看详细信息时可以使用filter过滤文本以节省token（推荐）。
传入to_page、get_info的前提是传入过search_text，因为没搜索过文本自然没有查询结果的分页和详细信息。
如果传入filter则必须传入get_info，因为没有详细信息则不知道要过滤什么东西。
search_text、to_pagination、get_info不能同时传入，一次只能传入一个。
`)
	}

	flag.Parse()

	if strings.TrimSpace(*searchText) == "" && strings.TrimSpace(*toPagination) == "" && strings.TrimSpace(*getInfo) == "" {
		log.Println("用法错误，你没有传入任何参数")
		flag.Usage()
		os.Exit(1)
	}

	allocatorCtx, _ := chromedp.NewRemoteAllocator(
		context.Background(),
		"ws://127.0.0.1:9224",
	)

	// 先创建临时 chromedp context 用于查询已有标签页
	tmpCtx, _ := chromedp.NewContext(allocatorCtx)

	var ctx context.Context
	targets, err := chromedp.Targets(tmpCtx)
	if err != nil {
		log.Fatal(err)
	}

	// 复用已有的 page 标签页，没有才用临时创建的
	for _, t := range targets {
		if t.Type == "page" {
			ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if ctx == nil {
		ctx = tmpCtx
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if strings.TrimSpace(*searchText) != "" {
		inputSelector := `textarea[name="q"]`

		var resultsJSON string
		var maxPage int

		err := chromedp.Run(ctx,
			chromedp.Navigate(`https://google.com`),
			chromedp.WaitVisible(inputSelector),
			chromedp.Focus(inputSelector),
			chromedp.SendKeys(inputSelector, *searchText+"\n"),

			// 等待"Web results" h2 出现，确认搜索结果已加载
			waitForWebResults(),

			// 提取搜索结果：找到每个结果项，获取总数、序号、标题，记录可点击链接的索引
			chromedp.Evaluate(extractSearchResultsJS, &resultsJSON),

			// 提取最大分页数
			chromedp.Evaluate(extractMaxPageJS, &maxPage),
		)
		if err != nil {
			log.Fatal(err)
		}
		parsePrint(resultsJSON, searchText, 1, maxPage)
		return
	}

	if strings.TrimSpace(*getInfo) != "" {
		idx, err := strconv.Atoi(strings.TrimSpace(*getInfo))
		if err != nil || idx < 0 {
			log.Fatalf("get_info 必须是有效的非负整数，收到: %q", *getInfo)
		}

		_ = idx
		// 在已有搜索结果页面上，根据序号点击对应结果的超链接。
		// 点击后会导航到目标页面，之后可将 html 转为 markdown 返回。
		//
		// 点击第 idx 个结果的 JS：
		//   clickResultByIndex(idx)
		//
		// 用法示例：
		//   err := chromedp.Run(ctx,
		//       chromedp.Evaluate(clickResultJS(idx), nil),
		//       chromedp.WaitReady(`body`, chromedp.ByQuery),
		//   )
		//   // 然后获取页面内容、转换 markdown、可选 filter 过滤

		// get_info 的点击与转换逻辑由使用者自行编排，DOM 定位原语已就绪
		return
	}

	if strings.TrimSpace(*toPagination) != "" {
		pageNum, err := strconv.Atoi(strings.TrimSpace(*toPagination))
		if err != nil || pageNum < 1 || pageNum > 10 {
			log.Fatalf("to_pagination 必须是 1~10 的整数，收到: %q", *toPagination)
		}

		var resultsJSON string
		var maxPage int
		// 在已有搜索结果页面上，定位分页并点击对应页码。
		err = chromedp.Run(ctx,
			chromedp.Evaluate(clickPaginationJS(pageNum), nil),
			// 等待"Web results" h2 出现，确认搜索结果已加载
			waitForWebResults(),

			// 提取搜索结果：找到每个结果项，获取总数、序号、标题，记录可点击链接的索引
			chromedp.Evaluate(extractSearchResultsJS, &resultsJSON),

			// 提取最大分页数
			chromedp.Evaluate(extractMaxPageJS, &maxPage),
		)
		if err != nil {
			log.Fatal(err)
		}
		parsePrint(resultsJSON, searchText, pageNum, maxPage)
		return
	}
}

func parsePrint(resultsJSON string, searchText *string, currentPage, maxPage int) {
	// 解析并输出搜索结果
	var searchResults struct {
		Count   int    `json:"count"`
		Error   string `json:"error,omitempty"`
		Results []struct {
			Index int    `json:"index"`
			Title string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(resultsJSON), &searchResults); err != nil {
		log.Fatalf("解析搜索结果失败: %v", err)
	}
	if searchResults.Error != "" {
		log.Fatal(searchResults.Error)
	}

	if *searchText != "" {
		fmt.Printf("搜索\"%s\"共 %d 条结果（第%d页/共%d页）：\n", *searchText, searchResults.Count, currentPage, maxPage)
	} else {
		fmt.Printf("共 %d 条结果（第%d页/共%d页）：\n", searchResults.Count, currentPage, maxPage)
	}
	for _, r := range searchResults.Results {
		fmt.Printf("[%d] %s\n", r.Index, r.Title)
	}
}

// 以下为 DOM 定位原语，使用语义化选择器和 DOM 层级关系，
// 避免依赖 Google 页面中一看就是随机生成的 class 名。

// waitForWebResults 等待搜索结果容器 #rso 出现后，给页面足够时间完成渲染。
// 不依赖任何文本内容（如 "Web results" h2），只依赖稳定的 DOM 结构锚点 div#rso。
func waitForWebResults() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		// 先等待 #rso 出现
		const checkJs = `(function(){
			var rso = document.getElementById('rso');
			return rso !== null && rso.children.length > 0;
		})()`
		for {
			var ready bool
			if err := chromedp.Evaluate(checkJs, &ready).Do(ctx); err != nil {
				return err
			}
			if ready {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
		// #rso 出现后等待 2 秒让所有结果渲染完成
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		return nil
	}
}

// extractSearchResultsJS 定位搜索结果并提取标题列表。
//
// 定位策略（不依赖任何文本内容或随机 class 名）：
//  1. Google 搜索结果容器 div#rso 的 ID 稳定
//  2. 每个搜索结果都是一个 a 标签包裹（或包含）一个 h3
//  3. 通过 h3.closest('a') 找到包装链接，用 href 去重
//  4. 排除 "People also ask"（相关问题）区域中的 h3，它们不是真正的搜索结果
const extractSearchResultsJS = `(function(){
	var container = document.getElementById('rso');
	if (!container) return JSON.stringify({count:0,results:[],error:"找不到搜索结果容器div#rso"});

	var h3s = container.querySelectorAll('h3');
	var results = [];
	var seen = {};

	for (var i = 0; i < h3s.length; i++) {
		// 跳过 "People also ask" / "相关问题" 块中的 h3
		if (isInsidePeopleAlsoAsk(h3s[i], container)) continue;

		var a = h3s[i].closest('a');
		if (!a) continue;
		var href = a.href;
		if (!href || seen[href]) continue;
		seen[href] = true;
		results.push({index: results.length, title: h3s[i].textContent.trim()});
	}
	return JSON.stringify({count: results.length, results: results});

	function isInsidePeopleAlsoAsk(el, rso) {
		var block = el.parentElement;
		while (block && block.parentElement !== rso) {
			block = block.parentElement;
		}
		if (!block) return false;
		var text = block.textContent;
		return text.indexOf('People also ask') !== -1 || text.indexOf('相关问题') !== -1;
	}
})()`

// extractMaxPageJS 从分页 DOM 中提取最大页码。
// 定位策略：查找所有带 aria-label="Page N" 的链接，取最大 N。
// 如果页面只有一页结果（没有分页链接），返回 1。
const extractMaxPageJS = `(function(){
	var links = document.querySelectorAll('a[aria-label^="Page "]');
	var max = 1;
	for (var i = 0; i < links.length; i++) {
		var n = parseInt(links[i].getAttribute('aria-label').replace('Page ', ''));
		if (!isNaN(n) && n > max) max = n;
	}
	return max;
})()`

// clickResultJS 生成点击第 index 个搜索结果链接的 JavaScript。
// index 从 0 开始，与 extractSearchResultsJS 返回的 index 一致。
func clickResultJS(index int) string {
	return fmt.Sprintf(`(function(){
		var container = document.getElementById('rso');
		if (!container) return '找不到搜索结果容器div#rso';
		var h3s = container.querySelectorAll('h3');
		var count = 0;
		var seen = {};
		for (var i = 0; i < h3s.length; i++) {
			var a = h3s[i].closest('a');
			if (!a) continue;
			var href = a.href;
			if (!href || seen[href]) continue;
			seen[href] = true;
			if (count === %d) { a.click(); return 'clicked'; }
			count++;
		}
		return '未找到序号为 %d 的结果';
	})()`, index, index)
}

// clickPaginationJS 生成点击分页中第 pageNum 页的 JavaScript。
// pageNum 从 1 开始。
// 定位策略：Google 分页链接使用 aria-label="Page N"，语义稳定。
func clickPaginationJS(pageNum int) string {
	return fmt.Sprintf(`(function(){
		var link = document.querySelector('a[aria-label="Page %d"]');
		if (link) { link.click(); return 'clicked'; }
		return '未找到第 %d 页的链接';
	})()`, pageNum, pageNum)
}
