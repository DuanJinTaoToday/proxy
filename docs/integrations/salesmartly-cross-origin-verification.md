# SaleSmartly 跨源客服联验

[Issue #102](https://github.com/DuanJinTaoToday/proxy/issues/102) 的 Free 验收仍待完成。2026-11-26 在真实 Pro 项目验证了跨源嵌入和访客发消息。工作台拦截了人工回复。生产接入由 [#112](https://github.com/DuanJinTaoToday/proxy/issues/112) 决定。

## 环境与结果

在 Windows 上用 Chrome 157.0.8103.51 联验。官网父来源是 `http://www.rocproxy.localhost:55928`，控制台父来源是 `http://console.rocproxy.localhost:55928`，客服 iframe 来源是 `http://support.rocproxy-chat.localhost:55928`。这些地址由本地测试服务生成，正式 HTTPS 域名和其他浏览器尚待验证。控制台联验页使用合成会话和订单。iframe 不接收访客身份、RocProxy 会话或订单字段。

安装代码与项目工作台「网页代码安装」一致。工作台显示 **Pro 版**，到期日为 2026-11-29。以下编号均为测试消息。

| 验收项 | 实测 |
| --- | --- |
| 官网发送消息 | 插件在 iframe 内打开并发送 `ROC102-WEB-1718`，工作台收到。 |
| 控制台发送消息 | 插件在 iframe 内打开并发送 `ROC102-CONSOLE-1718`，工作台收到。 |
| 人工回复 | 两条会话均可接入。工作台提示与中国大陆用户联络需完成企业认证，回复区不可用。访客收到人工回复待验收。 |
| 刷新后保留会话 | 首次并行打开两入口，工作台形成两条会话。刷新并重新打开聊天时，两个入口都只显示控制台消息，官网首条消息仍可在工作台查到。随后从官网发送 `ROC102-WEB-RELOAD-1718`，工作台将它显示在控制台消息所在的会话。单独刷新官网后重开聊天，两条消息均恢复。首次并行加载后的完整留存未通过。 |
| 隔离主站数据 | Playwright 在真实脚本创建插件节点后，从客服来源读取两个父页面的 DOM 和 `localStorage` 均得到 `SecurityError`。客服来源自身没有主站会话。向模拟订单接口发起跨源读请求被 CSP 阻止，直接匿名请求返回 `401` 且无 CORS 许可。控制台同源携带合成凭据可读订单。 |
| 限制嵌入来源 | 官网和控制台的 `frame-src` 只允许客服来源。两边嵌入未授权来源均被浏览器阻止。第三个父来源尝试嵌入客服来源时，客服响应的 `frame-ancestors` 阻止加载。 |

首次收紧客服来源 CSP 时，安装脚本可加载，聊天按钮等待配置和访客初始化。补齐实际使用的 SaleSmartly 静态资源与 API 来源后，两入口都能发送测试消息。规则和域名在[隔离测试页](../../tests/verify/fixtures/salesmartly.mjs)中。实时收取人工回复、第三方存储设置和正式域名下的 CSP 仍需实测。

## 复跑

在 Issue worktree 根目录运行隔离检查。默认不加载真实插件，浏览器使用合成父页面和订单接口。

```sh
npm --prefix tests/verify ci
npm --prefix tests/verify run browser:install
npm --prefix tests/verify test -- salesmartly-isolation.spec.mjs
```

人工联验前，在工作台确认项目显示 **Free 版**，并确认工作台能回复目标地区访客。从该项目复制专属安装脚本 URL，传给本地测试服务。

```powershell
$env:SALESMARTLY_WIDGET_SCRIPT = Read-Host 'SaleSmartly 安装脚本 URL'
node tests/verify/fixtures/salesmartly.mjs
```

在设有同一环境变量的终端运行 `npm --prefix tests/verify test -- salesmartly-isolation.spec.mjs`，可在真实脚本创建插件节点后重跑隔离断言。该模式仍使用合成主站数据。

1. 在同一新浏览器配置中打开测试服务输出的 `website` 地址，点击聊天按钮和 **Enter chat**，发送一条带唯一编号的消息。
2. 在 SaleSmartly 工作台进入 **聊天**、**全部**，找到唯一编号并接入会话。发一条带不同编号的回复，在访客页核对回复。
3. 刷新父页面后重开聊天，核对原发送和回复。用另一条编号消息从 `console` 地址重复流程。分别记录两入口的回复、刷新结果和浏览器控制台的 CSP 违规。
4. 在另一个新浏览器配置中同时首次打开两个地址，再分别发送、刷新并核对两条消息。记录工作台会话是否分成两条、访客各自能恢复哪些消息。

## 交接 #112

用可对目标地区访客回复的 Free 项目复跑。记录两个入口的人工回复与刷新结果，再用拟部署的 HTTPS 来源跑同样的隔离验证。如果 Free 或并行留存仍不能满足要求，由 #112 评审独立客服页等替代体验。
