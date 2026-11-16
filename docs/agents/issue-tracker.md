# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --json number,title,body,labels,comments`.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Make an issue a sub-issue of a parent**: `gh issue create --parent <parent> ...`, or `gh issue edit <parent> --add-sub-issue <child>` afterwards (`gh` 2.94+). Older `gh`: `gh api --method POST repos/<owner>/<repo>/issues/<parent>/sub_issues -F sub_issue_id=<child-db-id>` (database id, as in **Blocking** below). Without sub-issues, put `Part of #<parent>` at the top of the child body.
- **Blocking**: GitHub's **native issue dependencies**, the canonical, UI-visible representation. Add an edge with `gh issue create --blocked-by <n>,<n> ...` or `gh issue edit <child> --add-blocked-by <blocker>` (`gh` 2.94+). Older `gh`: `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`, where `<blocker-db-id>` is the blocker's numeric **database id** (`gh api repos/<owner>/<repo>/issues/<n> --jq .id`, _not_ the `#number` or `node_id`). Where dependencies aren't available, fall back to a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked when every blocker is closed.
- **Count open blockers**: `gh api repos/<owner>/<repo>/issues/<n> --jq .issue_dependencies_summary.blocked_by` (open blockers only; 0 means startable). `gh issue view <n> --json blockedBy` also lists closed blockers, so filter it: `--jq '[.blockedBy.nodes[] | select(.state == "OPEN")] | length'`.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: post the explanation with `gh issue comment <number> --body "..."`, then `gh issue close <number>`. On an already-closed issue, `gh issue close --comment` drops the comment and still exits 0.

Infer the repo from `git remote -v`; `gh` does this automatically when run inside a clone.

File issues only into the user's own repo. Never file into `mattpocock/skills` (the repo these skills come from) unless the user owns it. In a fork, `gh` can default to the upstream parent, so before the first `gh issue create` of a session, check the target with `gh repo view --json nameWithOwner` and confirm it with the user if it is not their repo.

## 拆票模板

拆分 Spec 时，按交付内容选用下面的段落，替换 `<…>` 占位符后贴进 Issue。影响下游开工的前置条件用 GitHub 原生依赖记录。公共能力可以先实现时，把下游验证写进关票条件。

### 供应商实测与下游交接

拆供应商验证票时加入本节。实测后更新结论，由实现票链接本节。若产品承诺有变化，同步父 Spec。

```markdown
## 实测结论与下游交接

- **已证实**：<观察到的行为和适用条件>。证据：<脱敏记录链接，含环境、版本和复跑步骤>。
- **产品承诺**：<客户可依赖的行为，以及由平台或供应商负责的边界>。
- **待验证与去向**：<未验证场景和原因>。#<实现票> 验收 <实现票负责的行为>。<其余待验证项的归属及是否影响开售>。
```

**填写示例（#40）**

```markdown
## 实测结论与下游交接

- **已证实**：悟空接受 `sidtime`，部分取值已实测连接成功。环境、版本和复跑步骤见 PR #57。
- **产品承诺**：Proxy 透传显式 `sidtime`，省略时沿用悟空默认行为。同一会话按约定选择悟空入口。实际会话时长、到期处理和出口 IP 由悟空决定。
- **待验证与去向**：时长单位、有效范围、起算点及到期行为仍待供应商侧确认。#53 验收透传、入口选择及客户说明。普通 SOCKS5 由 #51 独立验收。
```

### 跨 Issue 验收

拆票时，把依赖下游入口或流程才能验证的验收项写在公共能力票里。执行票引用对应条目，验收后回填证据链接。

```markdown
## 跨 Issue 验收

| 验收项 | 执行票 | 操作与预期 | 验收证据 |
| --- | --- | --- | --- |
| <待验证行为> | #<执行票> | <前置条件、操作入口和可观察结果> | <待回填或证据链接> |

**关票条件**：<本票验收条件>，且上表所列项目均已贴出通过证据。
```

**填写示例（#43）**

```markdown
## 跨 Issue 验收

| 验收项 | 执行票 | 操作与预期 | 验收证据 |
| --- | --- | --- | --- |
| 动态未开售时拒绝直接购买 | #46 | 关闭动态开售开关，用有效请求直接调用动态购买接口。服务端拒绝，钱包余额和订单记录无变化。 | 待贴测试结果 |
| 静态未开售时拒绝直接购买 | #48 | 关闭静态开售开关，用有效请求直接调用静态购买接口。服务端拒绝，钱包余额和订单记录无变化。 | 待贴测试结果 |

**关票条件**：#43 的本票验收项通过，且上表两项均已贴出通过证据。
```

## Pull requests as a triage surface

**PRs as a request surface: no.** _(Set to `yes` if this repo treats external PRs as feature requests; `/triage` reads this flag.)_

When set to `yes`, PRs run through the same labels and states as issues, using the `gh pr` equivalents:

- **Read a PR**: `gh pr view <number>`, then `gh pr view <number> --comments` (off a TTY, `--comments` prints only the comments, not the body), and `gh pr diff <number>` for the diff.
- **List external PRs for triage**: `gh api --paginate 'repos/{owner}/{repo}/pulls?state=open' --jq '.[] | select(.author_association | IN("OWNER","MEMBER","COLLABORATOR") | not) | {number, title, author: .user.login, author_association, labels: [.labels[].name]}'`.
- **Comment / label / close**: `gh pr comment`, `gh pr edit --add-label`/`--remove-label`, `gh pr close`.

GitHub shares one number space across issues and PRs, so a bare `#42` may be either: resolve with `gh pr view 42` and fall back to `gh issue view 42`.

## When a skill says "publish to the issue tracker"

Create a GitHub issue.

## When a skill says "fetch the relevant ticket"

Read it as in **Read an issue** above.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes / Decisions-so-far / Fog body. `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (see **Make an issue a sub-issue of a parent**). Where sub-issues aren't enabled, add the child to a task list in the map body and put `Part of #<map>` at the top of the child body. Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev.
- **Blocking**: see **Blocking** under Conventions.
- **Frontier query**: list the map's open children (`gh issue list --state open`, scoped to the map's sub-issues / task list), drop any with an open blocker (see **Count open blockers**, or an open issue in the `Blocked by` line) or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --add-assignee @me`, the session's first write.
- **Resolve**: `gh issue comment <n> --body "<answer>"`, then `gh issue close <n>`, then append a context pointer (gist + link) to the map's Decisions-so-far.
