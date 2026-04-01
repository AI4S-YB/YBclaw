# Git 工作规范

## Commit 格式

仓库使用 `commit-msg` hook 强制 [Conventional Commits](https://www.conventionalcommits.org/) 风格。

安装 hook：

```bash
bash scripts/install-hooks.sh
```

格式为 `<type>(<scope>): <subject>`，scope 可省略。支持的类型：

| 类型 | 用途 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 文档 |
| `style` | 格式调整（不影响逻辑） |
| `refactor` | 重构 |
| `test` | 测试 |
| `chore` | 杂项维护 |
| `build` | 构建相关 |
| `ci` | CI 配置 |
| `perf` | 性能优化 |
| `revert` | 回滚 |

示例：

```
feat(agent): add tool loop
fix(model): normalize tool schemas in requests
docs: update README
chore: add commit hooks
```
