// +build ignore

// 诊断：测试 NewContext 创建和 closeTab 关闭标签页的行为。
//
// 运行方式：
//   go run probe/diag_close_tab.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func main() {
	allocatorCtx, allocatorCancel := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9224")
	defer allocatorCancel()

	tmpCtx, _ := chromedp.NewContext(allocatorCtx)

	// 找已有搜索页
	targets, err := chromedp.Targets(tmpCtx)
	if err != nil {
		log.Fatal("获取 targets 失败:", err)
	}

	fmt.Println("===== 初始 targets =====")
	printTargets(targets)

	var searchCtx context.Context
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, "google.com/search") {
			fmt.Printf("\n复用搜索页: %s\n", t.Title)
			searchCtx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
			break
		}
	}
	if searchCtx == nil {
		fmt.Println("没有找到 Google 搜索页，使用第一个 page")
		for _, t := range targets {
			if t.Type == "page" {
				searchCtx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
				break
			}
		}
	}
	if searchCtx == nil {
		log.Fatal("没有任何 page 标签页")
	}
	defer func() {
		// 不 cancel searchCtx，避免关闭搜索页
		_ = searchCtx
	}()

	// ===== 测试 1：NewContext 创建标签页 =====
	fmt.Println("\n===== 测试 1：NewContext 创建标签页 =====")
	targetsBefore, _ := chromedp.Targets(searchCtx)
	fmt.Printf("创建前 targets 数量: %d\n", len(targetsBefore))

	newCtx, closeTab := chromedp.NewContext(allocatorCtx)
	fmt.Println("NewContext 已调用")

	// 给一点时间让标签页创建完成
	time.Sleep(500 * time.Millisecond)

	targetsMid, _ := chromedp.Targets(searchCtx)
	fmt.Printf("创建后 targets 数量: %d\n", len(targetsMid))
	printTargets(targetsMid)

	// 找到新创建的标签页（不在创建前列表中的）
	var newTargetID string
	for _, t := range targetsMid {
		isNew := true
		for _, old := range targetsBefore {
			if t.TargetID == old.TargetID {
				isNew = false
				break
			}
		}
		if isNew {
			newTargetID = string(t.TargetID)
			fmt.Printf("新标签页: type=%s title=%s url=%s\n", t.Type, t.Title, t.URL)
		}
	}

	// ===== 测试 2：在新标签页中导航 =====
	fmt.Println("\n===== 测试 2：导航到 about:blank（验证 context 有效）=====")
	newCtx, navCancel := context.WithTimeout(newCtx, 10*time.Second)
	defer navCancel()

	err = chromedp.Run(newCtx,
		chromedp.Navigate("about:blank"),
	)
	if err != nil {
		fmt.Printf("导航失败: %v\n", err)
	} else {
		fmt.Println("导航成功")
	}

	// ===== 测试 3：closeTab =====
	fmt.Println("\n===== 测试 3：closeTab() =====")
	fmt.Println("调用 closeTab() 前...")
	targetsBeforeClose, _ := chromedp.Targets(searchCtx)
	fmt.Printf("targets 数量: %d\n", len(targetsBeforeClose))

	fmt.Println("执行 closeTab()...")
	closeTab()
	fmt.Println("closeTab() 返回")

	time.Sleep(500 * time.Millisecond)

	targetsAfterClose, err := chromedp.Targets(searchCtx)
	if err != nil {
		fmt.Printf("closeTab 后获取 targets 失败: %v\n", err)
	} else {
		fmt.Printf("closeTab 后 targets 数量: %d\n", len(targetsAfterClose))
		printTargets(targetsAfterClose)

		// 检查新标签页是否还在
		found := false
		for _, t := range targetsAfterClose {
			if string(t.TargetID) == newTargetID {
				found = true
				fmt.Printf("*** 新标签页仍在! type=%s title=%s ***\n", t.Type, t.Title)
				break
			}
		}
		if !found {
			fmt.Println("新标签页已成功关闭")
		}
	}

	// ===== 测试 4：NewContext + 立即 closeTab（不导航）=====
	fmt.Println("\n===== 测试 4：创建后立即关闭（不导航）=====")
	t4Before, _ := chromedp.Targets(searchCtx)

	t4Ctx, t4Close := chromedp.NewContext(allocatorCtx)
	time.Sleep(300 * time.Millisecond)

	t4Mid, _ := chromedp.Targets(searchCtx)
	fmt.Printf("创建后 targets: %d -> %d\n", len(t4Before), len(t4Mid))

	fmt.Println("立即 closeTab()...")
	t4Close()
	time.Sleep(300 * time.Millisecond)

	t4After, _ := chromedp.Targets(searchCtx)
	fmt.Printf("关闭后 targets: %d\n", len(t4After))

	// 检查是否有关不掉的标签页
	_ = t4Ctx

	// ===== 测试 5：检查浏览器是否还活着 =====
	fmt.Println("\n===== 测试 5：浏览器存活检查 =====")
	finalTargets, err := chromedp.Targets(searchCtx)
	if err != nil {
		fmt.Printf("*** 浏览器似乎已关闭: %v ***\n", err)
		os.Exit(1)
	}
	fmt.Printf("浏览器存活，剩余 targets: %d\n", len(finalTargets))
	printTargets(finalTargets)

	fmt.Println("\n===== 诊断完成 =====")
}

func printTargets(targets []*target.Info) {
	for i, t := range targets {
		fmt.Printf("  [%d] type=%-6s title=%s\n", i, t.Type, t.Title)
	}
}
