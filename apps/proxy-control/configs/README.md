# 控制面租户登录与授权

`access.example.yaml` 预置两个租户、双租户成员、停用成员关系和独立运营账号。部署时为每个账号注入独立的密码摘要。摘要格式为 `<16 字节随机盐的 hex>:<32 字节摘要的 hex>`，算法为 PBKDF2-HMAC-SHA256，迭代 600,000 次。例如在可信终端生成一个摘要，按配置将结果注入对应环境变量。

```sh
python -c "import getpass, hashlib, secrets; salt = secrets.token_bytes(16); key = hashlib.pbkdf2_hmac('sha256', getpass.getpass().encode(), salt, 600000); print(salt.hex() + ':' + key.hex())"
```

```sh
go run ./apps/proxy-control/cmd -conf apps/proxy-control/configs/access.example.yaml
```

| HTTP 接口 | 用途 |
| --- | --- |
| `POST /v1/access/login` | 用 JSON 的 `username`、`password` 换取 `access_token` 和 `expires_in_seconds`。 |
| `GET /v1/access/tenants` | 返回当前身份的有效租户列表。 |
| `GET /v1/access/tenants/{tenant_id}` | 验证并返回选中的租户。控制台保存选择，每个后续业务请求仍需独立授权。 |
| `POST /v1/access/check` | 用 JSON 的 `permission` 和 `tenant_id` 检查当前身份。普通 JSON 的权限值为 1（读租户用量）、2（管理代理凭据）、3（读平台运营数据）。权限 3 省略 `tenant_id`。 |

登录后通过 `Authorization: Bearer <access_token>` 传递令牌。无有效身份返回 401，无权限返回 403。租户管理员只获得其有效成员关系内的权限。业务接口调用 `access.Service.Require`，传入本次实际操作的租户和服务端指定的权限。

默认配置允许启动健康检查，但没有预置登录账号。预置成员关系由启动配置加载，变更后重启控制面生效。会话有效期为八小时，签名密钥随进程启动生成，因此重启后需要重新登录。对外访问由部署入口提供 HTTPS 和登录频率限制。
