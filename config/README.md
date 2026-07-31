# 应用配置

[`app.yaml`](app.yaml) 是脚手架默认配置文件，由 `internal/bootstrap` 在代码默认值之后、环境
变量之前加载。`APP_CONFIG_FILE` 可以选择其他文件，`APP_` 环境变量仍拥有最高优先级。

本目录只保存部署配置事实，不保存 Go 类型或配置加载逻辑。强类型结构归拥有它的组合模块，
合并、校验和 Snapshot 生成由 Kernel 配置 Adapter 负责。
