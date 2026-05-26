# Google 搜索技能

通过 chromedp 驱动本地 Chrome 浏览器执行 Google 搜索，返回结构化结果。

## 前置条件

需要先启动调试浏览器：
```bash
./start_browser.sh          # 启动 Chrome，监听 9224 调试端口
```

用完后可关闭：
```bash
./stop_browser.sh <PID>     # 关闭浏览器
```

## 用法

### 1. 搜索

```bash
go run search.go -search_text "搜索关键词"
```

输出格式：
```
搜索"xxx"共 N 条结果（第1页/共M页）：
[0] 结果标题1
[1] 结果标题2
...
```

### 2. 翻页

拿到搜索结果后可以跳转到其他分页：

```bash
go run search.go -to_pagination 2
```

页码范围 1~10（实际取 min(10, 最大分页)）。会输出该页的搜索结果。

### 3. 查看详情

根据搜索结果中的序号查看页面详细内容：

```bash
go run search.go -get_info 0
```

会新开标签页导航到目标 URL，获取完整 HTML 并转换为 Markdown 输出。

### 4. 过滤详情（推荐）

查看详情时使用正则表达式过滤 Markdown 内容，大幅节省 token：

```bash
go run search.go -get_info 0 -filter "正则表达式"
```

正则引擎为 Go regexp2（支持零宽断言等高级特性）。

## 约束

- `search_text`、`to_pagination`、`get_info` 不能同时传入，一次只能用一个操作
- `-filter` 必须与 `-get_info` 一起使用
- 翻页和查看详情的前提是先执行过搜索（复用已有标签页中的搜索结果）

## 技术实现

- 通过 `chromedp.NewRemoteAllocator` 连接 `ws://127.0.0.1:9224`
- 复用已有 page 类型标签页，避免创建多余标签页
- 搜索结果从 `div#rso` 容器中通过 `h3.closest('a')` 定位
- 分页通过 `a[aria-label="Page N"]` 定位
- HTML→Markdown 转换使用 `JohannesKaufmann/html-to-markdown`，关闭转义
- 详情页面在新标签页中打开，获取 `document.documentElement.outerHTML`

## 诊断 DOM

遇到 DOM 定位问题时，在 `probe/` 目录下写临时 Go 文件连接 9224 端口查看实际 DOM 结构。优先使用语义化选择器和稳定的 ID/aria 属性，避免依赖 Google 随机生成的 class 名。
