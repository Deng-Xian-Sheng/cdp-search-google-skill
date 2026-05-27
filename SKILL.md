---
name: cdp-search-google
description: 通过 chromedp 驱动本地 Chrome 浏览器执行 Google 搜索，返回结构化结果。支持搜索、翻页、查看详情和过滤详情。
metadata:
  type: skill
---

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

## 二进制

| 文件 | 平台 |
|------|------|
| `bin/search-google-linux-amd64` | Linux x86_64 |
| `bin/search-google-darwin-amd64` | macOS Intel |
| `bin/search-google-darwin-arm64` | macOS Apple Silicon |

以下示例以 Linux 为例。

## 用法

### 1. 搜索

```bash
./bin/search-google-linux-amd64 -search_text "搜索关键词"
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
./bin/search-google-linux-amd64 -to_pagination 2
```

页码范围 1~10（实际取 min(10, 最大分页)）。会输出该页的搜索结果。

### 3. 查看详情

根据搜索结果中的序号查看页面详细内容：

```bash
./bin/search-google-linux-amd64 -get_info 0
```

会新开标签页导航到目标 URL，获取完整 HTML 并转换为 Markdown 输出。

### 4. 过滤详情（推荐）

查看详情时使用正则表达式过滤 Markdown 内容，大幅节省 token：

```bash
./bin/search-google-linux-amd64 -get_info 0 -filter "正则表达式"
```

正则引擎为 Go regexp2（支持零宽断言等高级特性）。

## 约束

- `search_text`、`to_pagination`、`get_info` 不能同时传入，一次只能用一个操作
- `-filter` 必须与 `-get_info` 一起使用
- 翻页和查看详情的前提是先执行过搜索（复用已有标签页中的搜索结果）

## 注意事项

- 仅支持macos和linux
- 如果用户说谷歌浏览器已经安装了，但是你发现 start_browser.sh 提示找不到谷歌浏览器，那说明不在PATH中，你可以查找谷歌浏览器的二进制路径并编辑 start_browser.sh 修改 GoogleChromePath 变量。