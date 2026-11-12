# proxy-console

本指南适用于 `apps/proxy-console/`。控制台面向租户和运营人员，通过控制面 API 访问平台数据。当前处于目录预留阶段，前端框架和构建工具随控制台任务确定。

| 目录 | 职责 |
| --- | --- |
| 应用根目录 | 前端项目清单、构建配置和应用独立构建入口。 |
| `src/` | 页面、组件及应用逻辑，按实际功能建立子目录。 |
| `src/api/` | 控制面 API 调用和响应映射。接口来源为 [控制面 API](../../api/proxycontrol/v1/)。 |

租户和运营操作的权限由控制面执行，界面根据 API 返回渲染可用状态。产品范围和验收见[一期 Spec](../../docs/features/proxy-phase-one/proxy-phase-one-spec.md)。
