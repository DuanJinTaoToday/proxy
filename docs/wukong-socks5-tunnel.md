# 悟空 SOCKS5 隧道转发接入

调用方连接悟空的 SOCKS5 入口并提交账密。悟空通过动态出口访问目标，目标服务看到的是实际出口 IP。这份文档覆盖上游隧道的配置和验证。

## 接入参数

| 字段 | 取值 |
| --- | --- |
| 协议 | SOCKS5。curl 使用 `socks5h` 时，目标域名由代理解析。 |
| 入口 | 联调可从 `154.81.157.72:12347` 开始。同事提供的地址范围是 `154.81.157.72`–`154.81.157.87`，本次只测了 `12347`。 |
| 基础用户名 | `JTG5iYU3` |
| 用户名模板 | `{username}-res-any-sid-{sid}-sidtime-{sidtime}`，认证时发送展开后的完整用户名。 |
| `sid` | 恰好 9 位数字，例如 `112312345`。原配置约定同一个 URL 生成同一个 SID，这是调用方的映射规则。 |
| `sidtime` | 按同事提供的样例传 `3600`。参数的有效期语义和到期行为待供应商确认。 |
| 密码 | 从已授权的凭据来源注入 `SEEDPIPE_PROXY_PASSWORD`。仓库和诊断输出只记录字段名。 |

## 联调步骤

1. 用 PowerShell 执行以下命令。已有 `SEEDPIPE_PROXY_PASSWORD` 时直接使用，否则交互输入。执行前确认 `NO_PROXY` 未匹配测试域名。

   ```powershell
   if (-not $env:SEEDPIPE_PROXY_PASSWORD) {
       $env:SEEDPIPE_PROXY_PASSWORD = Read-Host '悟空代理密码' -MaskInput
   }

   $target = 'https://api.ipify.org'
   $proxyEntry = '154.81.157.72:12347'
   $sid = '112312345'
   $proxyUser = "JTG5iYU3-res-any-sid-$sid-sidtime-3600"

   # 直连基线
   curl.exe -q -4 -fsS --noproxy '*' --connect-timeout 10 --max-time 20 `
       -w "\nHTTP=%{http_code}\n" $target

   # 经悟空隧道访问同一个目标
   curl.exe -q -fsS --connect-timeout 10 --max-time 20 `
       --proxy "socks5h://$proxyEntry" `
       --proxy-user "${proxyUser}:$env:SEEDPIPE_PROXY_PASSWORD" `
       -w "\nHTTP=%{http_code} peer=%{remote_ip}\n" $target
   ```

2. 核对代理请求返回 `HTTP=200`，`peer` 是选定的入口 IP，响应正文是查 IP 服务报告的出口 IP。与直连结果对照，并记录入口、目标、SID 和响应或错误。联调业务目标时，把 `$target` 换成业务 URL，按该目标的预期响应验收。
3. 遇到超时，先区分 TCP 是否建立。必要时加 `-w "tcp=%{time_connect} peer=%{remote_ip}"`（替换示例中的 `-w`），再用相同 SID 和目标复测或换一个已成功过的入口。若报 `curl: (97) Failed to receive SOCKS response, proxy closed connection`，记录为 SOCKS 握手期间被关闭，原因待查。

## 实测记录

2026-11-10 从本机测试，使用 `socks5h`、SID `112312345` 和 `sidtime=3600`。首轮目标为 `https://ip.sb/ip`，复测和 SID 对照使用 `https://api.ipify.org`。

- **转发成功**：12 个入口至少一次返回 HTTP 200，尾号为 `.72`、`.73`、`.74`、`.75`、`.78`、`.79`、`.80`、`.81`、`.84`、`.85`、`.86`、`.87`。`.73` 首轮超时，复测成功。以 `api.ipify.org` 为例，直连返回 `167.17.68.82`，经 `.72` 返回 `136.158.117.70`。
- **入口超时**：`.76`、`.77`、`.82`、`.83` 两轮均超时，复测时 TCP 未建立。接入机器或网络环境变化后重新验证。
- **会话粘性**：固定入口、目标和 SID 时，`.72` 两次成功请求返回同一个出口，中间有一次 SOCKS 连接被关闭。`.86` 两次成功请求分别返回 `160.30.69.12` 和 `160.30.69.1`。将 SID 视为会话标识。固定出口、粘性时长以及切换入口能否保留出口，均需供应商确认和单独验收。
