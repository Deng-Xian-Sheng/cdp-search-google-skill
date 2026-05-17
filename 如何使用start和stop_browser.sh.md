**start_browser.sh / stop_browser.sh 说明**

- **作用**  
  启动/停止一个专供 CDP 调试的 Chrome 浏览器（调试端口 9224）。

- **启动**  
  ```bash
  ./start_browser.sh
  ```
  无需参数，后台运行 Chrome，成功后显示：
  `调试端口9224。浏览器的PID: <PID>`  
  ⚠️ 端口 9224 被占用时会拒绝启动。

- **停止**  
  ```bash
  ./stop_browser.sh <PID>
  ```
  使用 `kill -9` 强制终止对应 PID 的浏览器进程。

- **典型流程**  
  ```bash
  ./start_browser.sh          # 启动并记下输出的 PID
  # 通过 9224 端口操控浏览器……
  ./stop_browser.sh <PID>     # 用完后关闭
  ```