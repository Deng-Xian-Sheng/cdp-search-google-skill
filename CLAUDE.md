# cdp-search-google-skill

## 架构

通过 chromedp 连接本地 Chrome 浏览器的 9224 调试端口，驱动 Google 搜索页面。
启动浏览器用 `start_browser.sh`，关闭用 `stop_browser.sh`。

## chromedp 连接模式（已验证可靠）

连接已运行的 Chrome 实例并复用已有标签页（调试时）：

```go
// 1. 创建 remote allocator，指向 Chrome DevTools 端口
allocatorCtx, _ := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9224")

// 2. 先创建一个临时 context 用于查询已有 target
tmpCtx, _ := chromedp.NewContext(allocatorCtx)

// 3. 列出所有 target，找到已有的 page 类型标签页
targets, _ := chromedp.Targets(tmpCtx)

// 4. 用 WithTargetID 附加到已有标签页（避免创建多余标签页）
var ctx context.Context
for _, t := range targets {
    if t.Type == "page" {
        ctx, _ = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
        break
    }
}
if ctx == nil {
    ctx = tmpCtx // 没有已有标签页，用临时创建的
}
```

### 连接经验与常见坑

1. **NewRemoteAllocator URL 格式**：`ws://127.0.0.1:9224` 可工作（末尾有无斜杠均可）。不要传 `/devtools/browser/...` 路径，allocator 会拒绝。如果必须传完整 WebSocket URL，需加 `chromedp.NoModifyURL` 选项。

2. **"invalid context" 错误**：通常是 context 链断裂——`chromedp.Run()` 要求 context 中存储了有效的 browser/target handler。直接用 `NewContext(allocatorCtx)` 创建全新标签页总是有效的，但会浪费标签页。用 `WithTargetID` 附加已有标签页需确保 target ID 有效。

3. **Targets() 查询**：需要从一个有效的 chromedp context 调用。模式是先用 allocator 创建临时 context，用其查询 targets，再决定附加哪个。

4. **取消函数**：`NewRemoteAllocator` 和 `NewContext` 都返回 cancel 函数。生产代码应 defer cancel，但注意不要在循环中过早取消 allocator。

5. **诊断 DOM 的技巧**：写独立的临时 Go 文件（放在 `probe/` 目录），直接连接已有标签页，用 `chromedp.Evaluate` 注入 JS 提取 DOM 信息。不要在生产代码中加临时调试逻辑。
遇到dom问题，可以写一些临时的go代码文件,要用golang的chromedp包连接9224端口,打开页面看看实际的dom是什么样的,如果不熟悉chromedp库,可以用go doc命令查看https://github.com/chromedp/chromedp的文档

6. 定位dom的实现鲁棒性要高,不要用html中的那种一看就是随机生成的字符串,那种不稳定

## 分页 DOM 结构（经验证，2026-05）

Google 搜索结果页底部分页导航：
- 分页链接在页面底部某个 `<table>` 中（通常是第 2 个 table）
- 每个页码 `<a>` 标签带 `aria-label="Page N"`（N 从 2 开始，最多到 10）
- 当前页（第 1 页时）用 `<td>` 标记而非 `<a>`，选择器返回 null 是预期行为
- `#foot`、`#navcnt` 在当前 Google 版本中**不存在**
- `#pnnext` 是"下一页"链接的稳定 ID
- 分页容器没有稳定的 id，只能通过 table > a[aria-label] 定位

## 搜索结果 DOM 结构（经验证）

- 结果容器：`div#rso`（ID 稳定）
- 每个结果：`h3` 在 `<a>` 内，通过 `h3.closest('a')` 定位
- 需排除 "People also ask" / "相关问题" 区块中的 h3