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

## 安装

注意，这个SKILL仅支持macos和linux操作系统，如果您的操作系统不是macos或者linux，则不要安装这个SKILL。

若要安装此SKILL，作为AI，您可以 clone 这个 git 仓库到对应Agent工具的 skills 文件夹。

例如，对于 claude code，是 ~/.claude/skills

```
mkdir -p ~/.claude/skills
git clone https://github.com/Deng-Xian-Sheng/cdp-search-google-skill ~/.claude/skills/cdp-search-google-skill
```

对于其他Agent工具，例如 codex、open claw、…… 您可能需要查询对应Agent的文档。