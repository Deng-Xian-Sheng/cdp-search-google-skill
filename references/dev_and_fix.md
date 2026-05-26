## 技术实现

- 通过 `chromedp.NewRemoteAllocator` 连接 `ws://127.0.0.1:9224`
- 复用已有 page 类型标签页，避免创建多余标签页
- 搜索结果从 `div#rso` 容器中通过 `h3.closest('a')` 定位
- 分页通过 `a[aria-label="Page N"]` 定位
- HTML→Markdown 转换使用 `JohannesKaufmann/html-to-markdown`，关闭转义
- 详情页面在新标签页中打开，获取 `document.documentElement.outerHTML`

## 诊断 DOM

遇到 DOM 定位问题时，在 `probe/` 目录下写临时 Go 文件连接 9224 端口查看实际 DOM 结构。优先使用语义化选择器和稳定的 ID/aria 属性，避免依赖 Google 随机生成的 class 名。
