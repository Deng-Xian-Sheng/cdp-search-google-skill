package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	searchText := flag.String("search_text", "", "要搜索的文本。会返回搜索结果，包含分页页码、每项结果。每项结果包含序号、标题。")
	toPagination := flag.String("to_pagination", "", "搜索结果是带分页的，这是页码，想看第几页就传几，支持1~10。")
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

	ctx, _ := chromedp.NewContext(allocatorCtx)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if strings.TrimSpace(*searchText) != "" {
		inputSelector := `textarea[name="q"]`

		err := chromedp.Run(ctx,
			chromedp.Navigate(`https://google.com`),
			chromedp.WaitVisible(inputSelector),
			chromedp.Focus(inputSelector),
			chromedp.SendKeys(inputSelector, *searchText+"\n"),
			// 寻找h2标签，特征是，标签`><`中间的内容是`Web results`
			// 然后找到和它同级的div，通常只有一个，在它的下方（html代码位置）
			// 这个div里面的内容如 tmp 文件 div 部分所示。
			// 我们需要搞定三件事：
			// 1、从搜索结果中找出每个结果，也就是每个这样的div，能拿到总数和每个的顺序，能操作任何一个。
			// 2、获取标题的文本。如 tmp 文件中的标题部分所示。
			// 3、找出每个超链接，我们不需要知道链接的url，但是需要能够点击到它。如 tmp 文件中的链接部分所示。
		)
		if err != nil {
			log.Fatal(err)
		}
	}
}
