<!-- 此文件和SKILL.md基本是相同的 -->
<!-- 但建议AI读取SKILL.md而不是此文件 -->

# Google 搜索技能

通过 chromedp 驱动本地 Chrome 浏览器执行 Google 搜索，返回结构化结果。

## 特点

### 身轻如燕

- 除了谷歌浏览器，**0依赖**
- 二进制分发，无需python uv、venv、conda环境

### 易用、透明
- 好安装，**SKILL一键**到手，无需顺丰发货，无需docker compose
- 透明，没几行代码，随便丢给一个AI都能给你解释清楚，功能简单纯粹不臃肿
- 好项目就得一眼看到头，这样才能安心使用

### 真正，不花钱，无限制

- 无需API，不用配置Key，没有次数限制，站起来蹬

### 省，还是省

- 省token，支持过滤搜索结果，不让任何一分钱花到刀把上

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