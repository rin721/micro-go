# Go 基础设施依赖注入与应用运行治理框架

## 总体架构设计、公共能力契约与使用说明

---

# 1. 文档说明

本文档定义一套面向 Go 项目的基础设施依赖注入与应用运行治理框架。

该框架位于项目底层，负责治理基础设施对象从声明、注册、构造、连接、启动、运行到停止和释放的完整过程，并为未来业务模块提供稳定、显式、可替换的公共基础能力契约。

本文档同时面向：

* 框架使用者；
* 基础设施能力开发者；
* 第三方库适配器开发者；
* 框架内核维护者；
* 未来业务模块开发者。

本文档重点回答：

1. 框架解决什么问题；
2. 哪些能力属于内核；
3. 公共基础能力如何抽象；
4. 第三方实现如何接入；
5. 基础设施和业务如何获得已注入能力；
6. 配置、依赖和生命周期如何治理；
7. 哪些能力当前不应提前建设；
8. 框架如何保证 Go 的直接调用和低运行时开销。

---

# 2. 框架定位

本框架是一套：

> 面向单进程 Go 应用的基础设施依赖注入、公共能力契约和运行治理内核。

框架主要解决：

* 基础设施组件在哪里声明；
* 对象如何被构造；
* 接口与实现如何绑定；
* 模块之间如何通过稳定契约建立依赖；
* 配置如何形成强类型构造事实；
* 组件如何按照依赖顺序启动；
* 构造或启动失败时如何回滚；
* 长期运行任务发生错误时如何上报；
* 应用退出时如何停止活动并释放资源；
* 具体第三方库如何被隔离；
* 未来业务如何按需使用日志、时钟、ID 生成器等公共能力。

该框架适用于：

* 普通 Go 后端项目；
* 模块化单体应用；
* 长时间运行的服务进程；
* 命令行应用；
* 后台任务程序；
* 需要统一启动和优雅退出的基础设施型项目。

该框架不是：

* 微服务平台；
* 分布式运行时；
* 服务注册中心；
* 服务网格；
* 通用通信 SDK；
* 动态插件平台；
* 业务领域框架；
* Java Spring Boot 的完整 Go 版本复制品。

---

# 3. 当前阶段的设计范围

当前阶段只实现以下完整闭环：

```text
配置源接入
    ↓
配置加载、合并和校验
    ↓
显式注册基础设施模块
    ↓
收集 Provider、契约和导出声明
    ↓
编译依赖关系图
    ↓
校验模块可见性和依赖合法性
    ↓
按照拓扑顺序构造实例
    ↓
生成生命周期执行计划
    ↓
执行 Prepare
    ↓
执行 Start
    ↓
监督长期运行组件
    ↓
应用进入 Running
    ↓
等待退出信号或运行错误
    ↓
反向执行 Stop
    ↓
反向执行 Close
    ↓
汇总错误并退出
```

当前阶段重点保证：

* 依赖显式；
* 对象统一构造；
* 模块边界清晰；
* 第三方实现不向上泄露；
* 启动前发现依赖错误；
* 构造失败能够清理；
* 启动失败能够回滚；
* 长期运行错误能够传播；
* 退出过程中尽可能释放全部资源；
* 框架不进入普通业务调用热路径；
* 未来能力可以按统一方式接入。

---

# 4. 当前阶段明确不实现的能力

为了避免框架过早膨胀，以下能力不进入第一阶段。

## 4.1 不内置具体基础设施

核心内核不直接内置：

* HTTP Server；
* CLI；
* Database；
* Cache；
* MQ；
* Worker；
* Cron；
* RPC；
* 文件存储；
* 对象存储；
* 链路追踪；
* 服务发现；
* 分布式锁；
* 后台管理平台。

这些能力未来只能作为独立能力包、实现适配器或入口模块接入。

---

## 4.2 不实现自动扫描

不实现：

* 包扫描；
* 组件扫描；
* 注解扫描；
* 自动发现模块；
* 导入依赖即自动启用；
* 自动猜测实现；
* 自动配置链；
* 条件注解。

所有模块必须由应用组装层显式启用。

---

## 4.3 不提供运行时服务定位

不提供：

```go
app.Get(...)
app.Resolve(...)
runtime.Service(...)
container.Find(...)
```

业务组件和基础设施组件都不得在运行过程中主动查询容器。

依赖必须在应用构建阶段完成连接。

---

## 4.4 不实现字段注入

禁止：

```go
type Service struct {
	Logger logging.Logger `inject:""`
}
```

只允许构造函数注入：

```go
func NewService(
	logger logging.Logger,
) *Service
```

---

## 4.5 不实现复杂作用域

第一阶段只支持：

```text
Application Singleton
```

即每个 Provider 在一次应用构建中只执行一次。

暂不支持：

* Transient；
* Request Scope；
* Job Scope；
* Session Scope；
* Generation Scope；
* Goroutine Local。

---

## 4.6 不实现完整热替换

第一阶段不实现：

* 实例代际；
* 候选依赖子图；
* 局部依赖图重建；
* 新旧实例并行运行；
* 入口原子切换；
* 运行时代码替换；
* 动态代理；
* 可替换实例句柄。

第一阶段只允许：

```text
安全原地更新
或者
请求应用重启
```

---

## 4.7 不实现通用通信 SDK

不实现：

* 进程内消息总线；
* 任意组件消息路由；
* 组件名称调用；
* 通用发布订阅；
* 业务事件总线；
* 跨进程通信协议。

组件之间通过构造函数注入后的普通 Go 接口直接调用。

---

# 5. 核心设计原则

## 5.1 显式优先

模块必须显式注册。

Provider 必须显式声明。

契约绑定必须显式声明。

配置路径必须显式声明。

具体实现必须由应用显式选择。

禁止将依赖隐藏在：

* 全局变量；
* 容器查询；
* 反射字段；
* 包初始化函数；
* 自动扫描；
* 字符串名称调用。

---

## 5.2 构建阶段治理，运行阶段直连

框架主要参与：

```text
应用构建
应用启动
配置变化
应用退出
```

应用进入 Running 后，组件之间使用普通 Go 指针或接口直接调用。

运行热路径不得经过：

* 容器；
* 反射；
* 动态代理；
* 字符串查找；
* 依赖解析器；
* 生命周期拦截器链。

---

## 5.3 统一契约，不统一获取入口

框架应提供公共基础能力契约层，但不得提供万能基础设施访问器。

正确方式：

```go
func NewService(
	logger logging.Logger,
	clock clock.Clock,
) *Service
```

禁止方式：

```go
func NewService(
	infra Infrastructure,
) *Service
```

其中：

```go
type Infrastructure interface {
	Logger() logging.Logger
	Clock() clock.Clock
	Database() Database
	Cache() Cache
}
```

这种聚合接口会隐藏真实依赖，并退化为 Service Locator。

---

## 5.4 小接口组合

生命周期不定义一个包含所有方法的大接口。

组件只实现自己真正需要的能力：

```go
type Preparer interface {
	Prepare(context.Context) error
}

type Starter interface {
	Start(context.Context) error
}

type Runner interface {
	Run(context.Context) error
}

type Stopper interface {
	Stop(context.Context) error
}

type Closer interface {
	Close(context.Context) error
}
```

---

## 5.5 模块内部默认私有

模块中的具体实现、辅助对象和内部 Provider 默认只能在模块内部使用。

模块必须显式导出契约。

其他模块只能依赖导出的契约，不能依赖内部具体类型。

---

## 5.6 资源所有权唯一

一个资源只能有一个生命周期所有者。

契约绑定不会创建新实例，也不会产生新的关闭责任。

组件不得主动关闭从外部注入的独立组件。

---

## 5.7 可选能力零负担

未启用配置监听时：

* 不启动监听 Goroutine；
* 不创建轮询器。

未启用重载时：

* 不创建重载管理器；
* 不维护动态引用。

未配置 Observer 时：

* 不创建事件队列；
* 不产生额外 Goroutine。

---

# 6. 总体分层架构

框架分为四个主要层次：

```text
┌──────────────────────────────────────────────┐
│        Application Composition Root          │
│       选择模块、配置来源和具体实现            │
├──────────────────────────────────────────────┤
│            Adapter Implementation            │
│        Zap / Slog / UUID / System Clock      │
├──────────────────────────────────────────────┤
│            Capability Contracts              │
│       Logger / Clock / ID Generator          │
├──────────────────────────────────────────────┤
│                   Kernel                     │
│ DI / Module / Config / Lifecycle / Runtime   │
└──────────────────────────────────────────────┘
```

---

# 7. Kernel：治理内核

Kernel 只负责基础治理机制。

建议包含：

```text
kernel/app
kernel/module
kernel/di
kernel/config
kernel/lifecycle
kernel/reload
```

Kernel 只识别：

* Module；
* Registry；
* Provider；
* Contract；
* Binding；
* Export；
* Config；
* Lifecycle；
* Application。

Kernel 不感知：

* Zap；
* Slog；
* Gin；
* GORM；
* Redis；
* Kafka；
* Cobra；
* Prometheus；
* Kubernetes。

---

# 8. Capability Contract Layer：公共能力契约层

公共能力契约层用于定义基础设施和未来业务共同依赖的稳定基础能力。

建议目录：

```text
capability/
├── logging/
├── clock/
├── idgen/
├── runtimeinfo/
└── random/
```

公共能力契约层可以包含：

* 接口；
* 值对象；
* 与实现无关的枚举；
* 基础错误语义；
* 测试或缺省使用的 Noop 实现。

公共能力契约层不得包含：

* 第三方库；
* Provider；
* 模块注册；
* 具体实现选择；
* DI Registry；
* Application Runtime；
* 全局实现映射；
* `GetLogger()` 等运行时获取方法。

---

# 9. Adapter Layer：实现适配器层

Adapter 负责使用第三方库实现公共能力契约。

建议目录：

```text
adapter/
├── logging/
│   ├── zap/
│   └── slog/
├── clock/
│   └── system/
└── idgen/
    └── uuid/
```

Adapter 负责：

* 具体配置定义；
* 第三方对象构造；
* 类型转换；
* 契约实现；
* 生命周期实现；
* Provider 注册；
* 契约绑定；
* 契约导出。

依赖方向必须保持：

```text
使用者组件
    ↓
公共能力契约
    ↑
具体实现适配器
```

公共契约不得依赖具体适配器。

---

# 10. Application Composition Root：应用组装根

应用组装根负责决定：

* 启用哪些模块；
* 使用哪些配置源；
* 选择哪个日志实现；
* 选择哪个时钟实现；
* 设置启动和关闭超时；
* 是否启用配置监听；
* 是否启用 Observer。

示例：

```go
func Modules() []module.Module {
	return []module.Module{
		zaplog.Module{},
		systemclock.Module{},
		runtimebase.Module{},
	}
}
```

应用组装根是具体实现选择发生的唯一位置。

消费者组件不决定具体实现。

Kernel 也不决定具体实现。

---

# 11. Module：模块

Module 是一组基础设施组件声明和注册的治理边界。

```go
type Module interface {
	Name() string
	Register(Registry) error
}
```

Module 负责：

* 注册模块配置；
* 注册 Provider；
* 声明契约绑定；
* 导出公共契约。

Module 不负责：

* 创建 Application；
* 启动组件；
* 控制其他模块；
* 查询实例；
* 保存 Registry；
* 执行业务流程。

示例：

```go
type Module struct{}

func (Module) Name() string {
	return "logging-zap"
}

func (Module) Register(
	registry module.Registry,
) error {
	// 注册配置、Provider、Binding 和 Export。
	return nil
}
```

模块名称必须唯一。

---

# 12. Provider：构造提供者

Provider 是普通 Go 构造函数。

第一阶段只允许以下形式：

```go
func(Dependencies...) Component
```

或者：

```go
func(Dependencies...) (Component, error)
```

示例：

```go
func NewLogger(
	cfg Config,
) (*Logger, error)
```

```go
func NewRuntimeMetadata(
	logger logging.Logger,
	clock clock.Clock,
	cfg Config,
) (*RuntimeMetadata, error)
```

Provider 规则：

* 只能返回一个主要组件；
* 第二个返回值只能是 `error`；
* 不允许可变参数；
* 不允许一次返回多个组件；
* 不允许接收 Registry；
* 不允许接收 Application；
* 不允许接收容器；
* 不允许注入 `context.Context`；
* 不启动长期 Goroutine；
* 不调用 `os.Exit`；
* 不调用 `log.Fatal`。

Provider 应主要负责：

* 分配对象；
* 注入不可变依赖；
* 执行轻量参数校验；
* 建立纯内存状态。

需要 Context、I/O 或外部资源检查的操作，应放入 `Prepare`。

---

# 13. Contract：契约

Contract 表示组件向其他模块提供的稳定能力。

通常使用 Go 接口定义。

示例：

```go
package clock

import "time"

type Clock interface {
	Now() time.Time
}
```

不是所有组件都必须创建接口。

只有以下情况适合定义契约：

* 需要跨模块使用；
* 需要隔离第三方实现；
* 存在替换实现；
* 测试需要替身；
* 能力语义长期稳定。

模块内部固定实现可以直接依赖具体类型。

---

# 14. Binding：契约绑定

Binding 描述公共契约与具体实现之间的映射关系。

例如：

```text
logging.Logger
        ↑
zaplog.Logger
```

Binding 不负责：

* 创建新实例；
* 复制实例；
* 包装动态代理；
* 运行时切换实现。

Binding 只声明：

> 当前具体实例同时满足某个公共契约。

建议提供泛型辅助函数：

```go
module.Bind[
	logging.Logger,
	*Logger,
](registry)
```

---

# 15. Export：模块导出

Export 表示模块允许其他模块依赖的契约。

Zap 日志模块内部可能存在：

```text
Config
EncoderBuilder
CoreBuilder
Logger
```

对外只导出：

```text
logging.Logger
```

其他模块不得直接依赖：

```go
*zaplog.Logger
```

而应依赖：

```go
logging.Logger
```

建议 API：

```go
module.Export[logging.Logger](registry)
```

---

# 16. Registry：注册表

Registry 只在模块注册阶段存在。

建议内部公开模型：

```go
type Registry interface {
	RegisterProvider(ProviderDeclaration) error
	RegisterBinding(BindingDeclaration) error
	RegisterExport(ExportDeclaration) error
	RegisterConfig(ConfigDeclaration) error
}
```

为使用者提供更简洁的辅助函数：

```go
module.Provide(registry, NewLogger)

module.Bind[
	logging.Logger,
	*Logger,
](registry)

module.Export[logging.Logger](registry)

module.Config[Config](
	registry,
	"logging",
)
```

模块注册完成后，Registry 必须冻结。

业务和基础设施组件不得保存 Registry。

---

# 17. 模块可见性规则

模块内部 Provider 产生的具体类型默认私有。

其他模块只能依赖当前模块显式导出的契约。

依赖图编译阶段必须检查：

* 跨模块依赖的契约是否被导出；
* 是否直接依赖其他模块私有实现；
* 契约是否有唯一实现；
* 导出是否指向有效绑定；
* 是否存在重复导出。

示例：

```text
logging-zap 模块
├── Config
├── EncoderBuilder
├── CoreBuilder
├── Logger
└── 导出 logging.Logger
```

其他模块可以依赖：

```go
logging.Logger
```

不能依赖：

```go
*zaplog.Logger
```

---

# 18. 依赖图编译

所有模块注册完成后，DI 编译器生成依赖图。

示例：

```text
LoggingConfig
      ↓
ZapLogger
      ↓ logging.Logger
RuntimeMetadata
      ↓
ManagedRuntime
```

编译阶段至少检查：

* Provider 签名是否合法；
* 参数依赖是否存在；
* 返回类型是否重复；
* 契约是否存在绑定；
* 契约是否存在多个未消歧义实现；
* 跨模块契约是否已导出；
* 是否存在循环依赖；
* 是否存在重复组件 ID；
* 配置声明是否完整；
* 生命周期组件是否可构造。

依赖图编译失败时，应用不得进入实例构造阶段。

---

# 19. 稳定执行顺序

有明确依赖时，按照依赖拓扑顺序执行。

多个组件之间没有直接依赖时，使用稳定顺序：

1. 按模块注册顺序；
2. 同一模块内按 Provider 声明顺序；
3. 不依赖 Map 遍历顺序；
4. 相同输入必须生成相同执行计划。

这可以保证：

* 启动日志稳定；
* 测试结果稳定；
* 错误顺序稳定；
* 行为可复现。

---

# 20. 实例构造

依赖图校验成功后，按照拓扑顺序调用 Provider。

框架执行结果应等价于以下普通 Go 代码：

```go
logger, err := zaplog.New(loggingConfig)
if err != nil {
	return err
}

clock := systemclock.New()

metadata, err := runtimebase.NewMetadata(
	logger,
	clock,
	runtimeConfig,
)
if err != nil {
	return err
}
```

框架只自动完成：

* Provider 调用顺序；
* 参数匹配；
* 应用单例实例缓存；
* 错误上下文包装；
* 构造失败清理。

应用进入 Running 后不再进行依赖解析。

---

# 21. 事务化 Build

应用构建过程必须具有事务语义：

```text
要么全部组件成功构造
要么清理全部已经构造的组件
```

假设：

```text
A 构造成功
B 构造成功
C 构造失败
```

框架必须执行：

```text
B.Close
A.Close
```

随后返回：

* C 的构造错误；
* B.Close 错误；
* A.Close 错误。

Build 失败时不得返回半构造 Application。

---

# 22. 配置管理

## 22.1 配置管理器职责

配置管理器负责：

* 接入配置源；
* 加载配置；
* 合并配置；
* 强类型转换；
* 应用默认值；
* 配置校验；
* 生成不可变配置快照；
* 可选监听配置变化。

配置管理器不负责：

* 构造组件；
* 启动组件；
* 修改运行对象；
* 重新注入字段；
* 修改依赖图；
* 修改契约绑定。

---

## 22.2 配置来源

配置来源可以包括：

```go
config.FromValues(...)
config.FromFile(...)
config.FromEnvironment(...)
config.FromFlags(...)
```

合并顺序由应用显式决定：

```go
app.WithConfigSources(
	config.FromValues(defaults),
	config.FromFile("configs/app.yaml"),
	config.FromEnvironment("APP"),
)
```

后面的来源覆盖前面的来源。

---

## 22.3 强类型配置

每个能力实现定义自己的配置：

```go
type Config struct {
	Level       string `yaml:"level"`
	Development bool   `yaml:"development"`
	Output      string `yaml:"output"`
}

func (c Config) Validate() error {
	if c.Level == "" {
		return fmt.Errorf("logging level is required")
	}

	if c.Output == "" {
		return fmt.Errorf("logging output is required")
	}

	return nil
}
```

模块声明配置路径：

```go
module.Config[Config](
	registry,
	"logging",
)
```

Provider 直接接收配置：

```go
func New(
	cfg Config,
) (*Logger, error)
```

禁止普通组件接收全局配置管理器：

```go
func New(
	manager *config.Manager,
) *Logger
```

---

## 22.4 不可变配置快照

配置加载和校验完成后生成不可变快照。

```go
type Snapshot struct {
	Version  uint64
	LoadedAt time.Time
}
```

具体强类型配置由框架内部提供给 Provider。

组件不得修改共享配置。

配置包含 Slice、Map 或 Pointer 时，应避免多个组件共享可变内部状态。

---

## 22.5 敏感配置边界

配置错误、Observer 事件和日志中不得默认输出：

* 密码；
* Token；
* API Key；
* 私钥；
* 完整连接字符串；
* Cookie Secret。

第一阶段不必实现完整 Secret Manager，但必须避免主动泄露敏感配置。

---

# 23. 生命周期模型

第一阶段支持：

```go
type Preparer interface {
	Prepare(context.Context) error
}

type Starter interface {
	Start(context.Context) error
}

type Runner interface {
	Run(context.Context) error
}

type Stopper interface {
	Stop(context.Context) error
}

type Closer interface {
	Close(context.Context) error
}
```

组件只实现需要的接口。

---

# 24. Prepare

Prepare 用于执行启动前准备。

适合：

* 检查本地目录；
* 创建必要目录；
* 加载静态资源；
* 检查运行条件；
* 建立尚未正式运行的资源；
* 执行需要 Context 的初始化。

Prepare 不应：

* 启动永久 Goroutine；
* 接收外部请求；
* 长期阻塞；
* 控制其他组件。

---

# 25. Start

Start 用于将组件切换到已启动状态。

Start 必须快速返回。

不推荐：

```go
func (s *Server) Start(
	ctx context.Context,
) error {
	return s.server.Serve()
}
```

长期阻塞行为应放入 Runner：

```go
func (s *Server) Start(
	ctx context.Context,
) error {
	return s.bindListener()
}

func (s *Server) Run(
	ctx context.Context,
) error {
	return s.server.Serve()
}
```

---

# 26. Runner

Runner 表示长期运行任务。

```go
type Runner interface {
	Run(context.Context) error
}
```

生命周期管理器负责：

* 启动 Runner；
* 监督 Runner；
* 捕获 Runner 返回错误；
* 在应用退出时取消 Runner；
* 等待 Runner 结束。

Runner 返回非空错误时：

1. 记录运行错误；
2. 取消应用运行 Context；
3. 请求其他 Runner 退出；
4. 执行 Stop；
5. 执行 Close；
6. 汇总并返回错误。

Runner 在应用 Context 尚未取消时正常返回，也应触发应用退出，避免组件已停止但应用仍显示 Running。

---

# 27. Stop

Stop 用于停止组件运行活动。

适合：

* 停止接收新任务；
* 通知后台循环退出；
* 等待内部活动结束；
* 停止持续运行状态。

Stop 必须尽可能幂等。

重复调用不得：

* 重复关闭 Channel；
* 重复释放资源；
* Panic；
* 破坏内部状态。

---

# 28. Close

Close 用于释放最终资源。

适合：

* 关闭文件；
* 关闭连接；
* 刷新缓冲区；
* 释放句柄；
* 清理临时资源。

语义必须保持：

```text
Stop
停止运行行为。

Close
释放最终资源。
```

即使组件未成功 Start，只要已经构造并实现 Close，也可能需要执行 Close。

---

# 29. 资源所有权

默认规则：

> 由 DI 创建并登记的独立组件，由生命周期管理器统一关闭。

组件不得主动关闭外部注入的独立组件。

错误示例：

```go
type Service struct {
	logger logging.Logger
}

func (s *Service) Close(
	ctx context.Context,
) error {
	return s.logger.Close(ctx)
}
```

如果 Logger 是独立生命周期组件，应由框架关闭。

组件只负责关闭：

* 自己内部创建；
* 未单独注册；
* 不与其他组件共享；

的私有资源。

---

# 30. 生命周期执行顺序

假设：

```text
A
↓
B
↓
C
```

构造顺序：

```text
A
B
C
```

Prepare：

```text
A.Prepare
B.Prepare
C.Prepare
```

Start：

```text
A.Start
B.Start
C.Start
```

Stop：

```text
C.Stop
B.Stop
A.Stop
```

Close：

```text
C.Close
B.Close
A.Close
```

---

# 31. 失败回滚

## 31.1 构造失败

```text
A 构造成功
B 构造成功
C 构造失败
```

执行：

```text
B.Close
A.Close
```

---

## 31.2 Prepare 失败

```text
A.Prepare 成功
B.Prepare 失败
```

此时没有组件成功 Start，因此不执行 Stop。

所有已构造组件反向 Close。

---

## 31.3 Start 失败

```text
A.Start 成功
B.Start 成功
C.Start 失败
```

执行：

```text
B.Stop
A.Stop
```

然后：

```text
C.Close
B.Close
A.Close
```

---

## 31.4 Runner 失败

Runner 返回错误时：

```text
Running
    ↓
取消全部 Runner
    ↓
Stop
    ↓
Close
    ↓
Failed
```

---

# 32. Panic 边界

框架调用以下用户代码时必须建立 Panic 边界：

* Module.Register；
* Provider；
* Prepare；
* Start；
* Run；
* Stop；
* Close；
* Observer。

发生 Panic 时：

* 捕获 Panic；
* 保存堆栈；
* 转换为结构化错误；
* 进入失败清理流程。

框架无法捕获组件自行创建且未受监督的所有 Goroutine Panic。

长期运行任务应优先通过 Runner 交给框架监督。

---

# 33. 应用状态机

建议状态：

```text
Created
    ↓
Registering
    ↓
Compiling
    ↓
Constructing
    ↓
Built
    ↓
Preparing
    ↓
Starting
    ↓
Running
    ↓
Stopping
    ↓
Closing
    ↓
Closed
```

异常状态：

```text
Failed
RestartRequired
```

状态规则：

* 一个 Application 只能 Build 一次；
* 一个 Application 只能 Run 一次；
* Running 后禁止修改依赖图；
* Running 后禁止注册模块；
* Stopping 后拒绝重载；
* Closed 后不能重新运行；
* Stop 和 Close 必须幂等；
* 重载与关停冲突时关停优先。

---

# 34. 最小重载能力

第一阶段只定义：

```go
type Result uint8

const (
	Applied Result = iota
	Ignored
	RestartRequired
)
```

可选接口：

```go
type Reloader interface {
	Reload(
		context.Context,
		config.Snapshot,
	) (Result, error)
}
```

只有能够保证以下条件的组件才实现 Reload：

* 并发安全；
* 新配置已校验；
* 更新操作完整；
* 失败后旧状态仍然有效；
* 不需要替换外部依赖实例。

无法安全原地更新时返回：

```go
RestartRequired
```

由外部进程管理器重新启动应用。

---

# 35. Observer

框架可以提供只读观察接口：

```go
type Observer interface {
	Observe(Event)
}
```

Event 可以描述：

* 模块开始注册；
* 模块注册完成；
* 配置加载成功；
* 配置校验失败；
* 依赖图编译完成；
* 组件开始构造；
* 组件构造失败；
* 生命周期阶段开始；
* 生命周期阶段结束；
* Runner 失败；
* 应用开始退出；
* 组件关闭失败。

Observer 只能观察。

Observer 不允许：

* 查询组件；
* 修改运行状态；
* 调用其他组件；
* 发送业务命令；
* 充当消息总线；
* 阻塞生命周期主流程。

---

# 36. Bootstrap 诊断与应用日志

日志模块本身也需要由 DI 构造。

但是在日志模块构造之前，可能已经发生：

* 配置加载错误；
* 模块注册错误；
* 依赖图编译错误；
* Provider 构造错误。

因此 Kernel 不能强制依赖 `logging.Logger`。

应区分：

## Bootstrap Diagnostics

用于应用构建早期：

* 结构化错误返回；
* 可选 Bootstrap Observer；
* 标准错误输出。

## Application Logger

日志模块构造完成后，供基础设施和业务组件使用。

最外层负责输出构建错误：

```go
application, err := app.Build(...)
if err != nil {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
```

---

# 37. 公共能力契约准入标准

一个能力进入公共契约层前，应满足大部分条件：

1. 被多个无关组件共同依赖；
2. 语义长期稳定；
3. 与第三方实现无关；
4. 接口可以保持较小；
5. 多种实现可以合理映射；
6. 不暴露厂商专有类型；
7. 不会退化为最低公共能力集合；
8. 测试替换具有价值；
9. 生命周期可以独立治理；
10. 不需要运行时容器查询。

适合优先公共化：

* Logger；
* Clock；
* IDGenerator；
* RandomSource；
* RuntimeMetadata。

需要谨慎判断：

* Metrics；
* Tracer；
* SecretProvider；
* TransactionManager；
* FileStorage。

通常不应提前公共化：

* Database；
* ORM；
* Cache；
* MQ；
* HTTP Router；
* RPC Client。

---

# 38. 公共接口与消费者局部接口

框架采用两级接口体系。

## 38.1 平台公共能力契约

适用于跨基础设施和业务广泛复用、语义稳定的能力：

```text
capability/logging.Logger
capability/clock.Clock
capability/idgen.Generator
```

## 38.2 消费者局部契约

数据库、缓存、MQ 等语义差异较大的能力，由消费者定义最小接口。

例如用户模块定义：

```go
type UserReader interface {
	FindUser(
		ctx context.Context,
		id string,
	) (User, error)
}
```

订单模块定义：

```go
type TransactionRunner interface {
	WithinTransaction(
		ctx context.Context,
		fn func(context.Context) error,
	) error
}
```

会话模块定义：

```go
type SessionStore interface {
	Get(
		ctx context.Context,
		token string,
	) (Session, bool, error)

	Set(
		ctx context.Context,
		token string,
		session Session,
		ttl time.Duration,
	) error
}
```

这些接口属于消费者，不应统一塞进公共能力层。

---

# 39. 为什么不定义统一 Infrastructure 接口

不建议：

```go
type Infrastructure interface {
	Logger() logging.Logger
	Clock() clock.Clock
	IDGenerator() idgen.Generator
	Database() Database
	Cache() Cache
}
```

这种设计会：

* 隐藏组件真实依赖；
* 让依赖图失去准确性；
* 让接口持续膨胀；
* 允许组件获取不需要的能力；
* 增加测试替身成本；
* 退化为 Service Locator。

正确方式是按需注入：

```go
func NewService(
	logger logging.Logger,
	clock clock.Clock,
	idgen idgen.Generator,
) *Service
```

---

# 40. Zap 日志能力完整示例

## 40.1 公共日志契约

```go
package logging

import "context"

type Logger interface {
	Debug(
		ctx context.Context,
		message string,
		fields ...Field,
	)

	Info(
		ctx context.Context,
		message string,
		fields ...Field,
	)

	Warn(
		ctx context.Context,
		message string,
		fields ...Field,
	)

	Error(
		ctx context.Context,
		message string,
		fields ...Field,
	)

	With(fields ...Field) Logger
	Named(name string) Logger
}
```

---

## 40.2 公共字段模型

```go
package logging

import "time"

type Field struct {
	Key   string
	Value any
}

func String(
	key string,
	value string,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Int(
	key string,
	value int,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Time(
	key string,
	value time.Time,
) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Error(err error) Field {
	return Field{
		Key:   "error",
		Value: err,
	}
}
```

公共日志契约不提供：

```go
Fatal(...)
Panic(...)
Raw() *zap.Logger
```

避免组件主动结束进程或绕过抽象。

---

## 40.3 Zap 配置

```go
package zaplog

type Config struct {
	Level       string `yaml:"level"`
	Development bool   `yaml:"development"`
	Output      string `yaml:"output"`
}

func (c Config) Validate() error {
	if c.Level == "" {
		return fmt.Errorf(
			"logging level is required",
		)
	}

	if c.Output == "" {
		return fmt.Errorf(
			"logging output is required",
		)
	}

	return nil
}
```

---

## 40.4 Zap 实现

```go
package zaplog

type Logger struct {
	logger *zap.Logger
}
```

```go
func New(
	cfg Config,
) (*Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf(
			"validate logging config: %w",
			err,
		)
	}

	zapLogger, err := buildZap(cfg)
	if err != nil {
		return nil, fmt.Errorf(
			"build zap logger: %w",
			err,
		)
	}

	return &Logger{
		logger: zapLogger,
	}, nil
}
```

```go
func (l *Logger) Info(
	ctx context.Context,
	message string,
	fields ...logging.Field,
) {
	l.logger.Info(
		message,
		convertFields(fields)...,
	)
}
```

```go
func (l *Logger) Error(
	ctx context.Context,
	message string,
	fields ...logging.Field,
) {
	l.logger.Error(
		message,
		convertFields(fields)...,
	)
}
```

关闭能力：

```go
func (l *Logger) Close(
	ctx context.Context,
) error {
	if err := l.logger.Sync(); err != nil {
		return fmt.Errorf(
			"sync zap logger: %w",
			err,
		)
	}

	return nil
}
```

编译期接口检查：

```go
var _ logging.Logger = (*Logger)(nil)
var _ lifecycle.Closer = (*Logger)(nil)
```

---

## 40.5 Zap 模块

```go
package zaplog

type Module struct{}

func (Module) Name() string {
	return "logging-zap"
}

func (Module) Register(
	registry module.Registry,
) error {
	if err := module.Config[Config](
		registry,
		"logging",
	); err != nil {
		return fmt.Errorf(
			"register logging config: %w",
			err,
		)
	}

	if err := module.Provide(
		registry,
		New,
	); err != nil {
		return fmt.Errorf(
			"provide zap logger: %w",
			err,
		)
	}

	if err := module.Bind[
		logging.Logger,
		*Logger,
	](registry); err != nil {
		return fmt.Errorf(
			"bind logging contract: %w",
			err,
		)
	}

	if err := module.Export[logging.Logger](
		registry,
	); err != nil {
		return fmt.Errorf(
			"export logging contract: %w",
			err,
		)
	}

	return nil
}
```

---

# 41. 基础设施如何使用公共能力

运行时元数据组件需要日志和时钟：

```go
package runtimebase

type Metadata struct {
	logger logging.Logger
	clock  clock.Clock
	name   string
}

func NewMetadata(
	logger logging.Logger,
	clock clock.Clock,
	cfg Config,
) (*Metadata, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Metadata{
		logger: logger.Named("runtime-metadata"),
		clock:  clock,
		name:   cfg.Name,
	}, nil
}
```

基础设施组件只依赖公共契约，不依赖 Zap 或系统时钟实现。

---

# 42. 未来业务如何使用公共能力

未来业务服务通过构造函数获得需要的能力：

```go
package user

type Service struct {
	logger logging.Logger
	clock  clock.Clock
	idgen  idgen.Generator
	repo   Repository
}

func NewService(
	logger logging.Logger,
	clock clock.Clock,
	idgen idgen.Generator,
	repo Repository,
) *Service {
	return &Service{
		logger: logger.Named("user-service"),
		clock:  clock,
		idgen:  idgen,
		repo:   repo,
	}
}
```

业务方法直接调用已注入实例：

```go
func (s *Service) Create(
	ctx context.Context,
	command CreateCommand,
) error {
	userID := s.idgen.New()
	now := s.clock.Now()

	s.logger.Info(
		ctx,
		"creating user",
		logging.String("user_id", userID),
		logging.Time("created_at", now),
	)

	return s.repo.Create(
		ctx,
		User{
			ID:        userID,
			CreatedAt: now,
		},
	)
}
```

业务代码不需要知道：

* 日志实现是不是 Zap；
* 时钟是不是系统时钟；
* ID 是否来自 UUID；
* 实例由哪个容器构造；
* 模块如何注册；
* 生命周期如何管理。

---

# 43. 应用选择具体实现

应用组装层选择 Zap 和系统时钟：

```go
func Modules() []module.Module {
	return []module.Module{
		zaplog.Module{},
		systemclock.Module{},
		runtimebase.Module{},
	}
}
```

切换为 Slog：

```go
func Modules() []module.Module {
	return []module.Module{
		slogadapter.Module{},
		systemclock.Module{},
		runtimebase.Module{},
	}
}
```

消费者无需修改。

---

# 44. 框架公开 API 建议

## 44.1 构建应用

```go
application, err := app.Build(
	ctx,
	app.WithModules(
		Modules()...,
	),
	app.WithConfigSources(
		config.FromFile(
			"configs/app.yaml",
		),
		config.FromEnvironment("APP"),
	),
	app.WithStartupTimeout(
		15*time.Second,
	),
	app.WithShutdownTimeout(
		15*time.Second,
	),
)
```

---

## 44.2 运行应用

```go
return application.Run(ctx)
```

---

## 44.3 只编译不构造

用于测试和诊断：

```go
plan, err := app.Compile(
	app.WithModules(
		Modules()...,
	),
	app.WithConfigSources(...),
)
```

Compile 负责：

* 注册模块；
* 加载并校验配置；
* 编译依赖图；
* 校验契约；
* 生成诊断结果。

Compile 不构造实例，不执行生命周期。

---

## 44.4 导出依赖图

```go
graph := plan.DependencyGraph()
```

可以支持输出：

* 文本；
* DOT；
* JSON。

用于：

* 架构诊断；
* 测试；
* 文档生成；
* CI 检查。

---

# 45. 应用入口示例

```go
package application

func Run(
	ctx context.Context,
) error {
	application, err := app.Build(
		ctx,
		app.WithModules(
			Modules()...,
		),
		app.WithConfigSources(
			config.FromFile(
				"configs/app.yaml",
			),
			config.FromEnvironment("APP"),
		),
		app.WithStartupTimeout(
			15*time.Second,
		),
		app.WithShutdownTimeout(
			15*time.Second,
		),
	)
	if err != nil {
		return fmt.Errorf(
			"build application: %w",
			err,
		)
	}

	if err := application.Run(ctx); err != nil {
		return fmt.Errorf(
			"run application: %w",
			err,
		)
	}

	return nil
}
```

进程入口：

```go
package main

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := application.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

操作系统信号由应用入口处理。

Kernel 只依赖 `context.Context`。

---

# 46. 推荐框架仓库结构

```text
foundation/
├── kernel/
│   ├── app/
│   │   ├── build.go
│   │   ├── compile.go
│   │   ├── application.go
│   │   ├── run.go
│   │   ├── shutdown.go
│   │   ├── state.go
│   │   ├── options.go
│   │   └── observer.go
│   │
│   ├── module/
│   │   ├── module.go
│   │   ├── registry.go
│   │   ├── provide.go
│   │   ├── bind.go
│   │   ├── export.go
│   │   └── config.go
│   │
│   ├── di/
│   │   ├── provider.go
│   │   ├── contract.go
│   │   ├── descriptor.go
│   │   ├── graph.go
│   │   ├── compiler.go
│   │   ├── validator.go
│   │   ├── constructor.go
│   │   └── instances.go
│   │
│   ├── config/
│   │   ├── source.go
│   │   ├── loader.go
│   │   ├── merge.go
│   │   ├── decoder.go
│   │   ├── validator.go
│   │   ├── snapshot.go
│   │   └── change.go
│   │
│   ├── lifecycle/
│   │   ├── interfaces.go
│   │   ├── descriptor.go
│   │   ├── plan.go
│   │   ├── executor.go
│   │   ├── supervisor.go
│   │   ├── rollback.go
│   │   └── error.go
│   │
│   └── reload/
│       ├── interface.go
│       └── result.go
│
├── capability/
│   ├── logging/
│   │   ├── logger.go
│   │   ├── field.go
│   │   ├── level.go
│   │   └── noop.go
│   │
│   ├── clock/
│   │   └── clock.go
│   │
│   ├── idgen/
│   │   └── generator.go
│   │
│   └── runtimeinfo/
│       └── provider.go
│
├── adapter/
│   ├── logging/
│   │   ├── zap/
│   │   └── slog/
│   │
│   ├── clock/
│   │   └── system/
│   │
│   └── idgen/
│       └── uuid/
│
└── internal/
    ├── graph/
    ├── multierror/
    ├── panicguard/
    └── stableorder/
```

---

# 47. 使用框架的项目结构

```text
project/
├── cmd/
│   └── app/
│       └── main.go
│
├── internal/
│   ├── application/
│   │   ├── run.go
│   │   ├── modules.go
│   │   └── config.go
│   │
│   ├── infrastructure/
│   │   └── runtimebase/
│   │       ├── module.go
│   │       ├── config.go
│   │       ├── provider.go
│   │       ├── contract.go
│   │       └── component.go
│   │
│   └── business/
│       ├── user/
│       └── order/
│
├── configs/
│   └── app.yaml
│
├── docs/
├── go.mod
└── go.sum
```

框架不强制业务必须使用：

* Domain；
* UseCase；
* Repository；
* Controller；
* Handler。

业务架构由具体项目自行决定。

---

# 48. 测试策略

## 48.1 组件单元测试

直接调用 Provider：

```go
logger := &FakeLogger{}
clock := &FakeClock{}

component, err := NewComponent(
	logger,
	clock,
	testConfig,
)
```

组件单元测试不需要启动完整框架。

---

## 48.2 模块编译测试

```go
plan, err := app.Compile(
	app.WithModules(
		zaplog.Module{},
		runtimebase.Module{},
	),
	app.WithConfigSources(
		config.FromValues(testValues),
	),
)
```

验证：

* Provider 签名；
* 配置声明；
* 契约绑定；
* 模块导出；
* 缺失依赖；
* 循环依赖；
* 多实现冲突。

---

## 48.3 构造回滚测试

设计一个 Provider 在中途失败，验证：

* 已构造组件反向 Close；
* 原始构造错误保留；
* Close 错误被汇总；
* 不返回半成品 Application。

---

## 48.4 生命周期测试

验证：

* Prepare 顺序；
* Start 顺序；
* Stop 顺序；
* Close 顺序；
* Start 失败回滚；
* Runner 错误传播；
* Panic 转换；
* Context 取消；
* 启动和关闭超时。

---

## 48.5 Adapter 契约测试

每个适配器执行编译期检查：

```go
var _ logging.Logger = (*Logger)(nil)
var _ lifecycle.Closer = (*Logger)(nil)
```

并验证：

* 第三方类型不泄露；
* 字段映射正确；
* 配置校验正确；
* Close 行为幂等；
* 契约语义一致。

---

# 49. 错误模型

框架错误至少携带：

```go
type ComponentError struct {
	Module    string
	Component string
	Provider  string
	Phase     Phase
	Cause     error
}
```

阶段至少包括：

```text
ModuleRegister
ConfigLoad
ConfigDecode
ConfigValidate
GraphCompile
Construct
Prepare
Start
Run
Stop
Close
Observe
```

多个关闭错误使用：

```go
errors.Join(errs...)
```

任何组件关闭失败都不能阻止后续组件继续关闭。

---

# 50. 性能边界

## 50.1 Build 阶段允许

允许在 Build 阶段使用：

* 少量反射解析 Provider；
* 配置反射解码；
* 接口绑定检查；
* 依赖图构建；
* 拓扑排序；
* 生命周期接口断言。

这些操作只发生在启动阶段。

---

## 50.2 Running 阶段禁止

普通业务调用过程中禁止：

* 容器查询；
* 反射方法调用；
* 动态代理；
* 字符串服务查找；
* 依赖图遍历；
* Provider 调用；
* 生命周期拦截器链。

组件之间直接调用：

```go
result, err := service.Execute(ctx)
```

---

## 50.3 空载成本

未启用可选能力时：

* 不创建额外 Goroutine；
* 不创建 Ticker；
* 不创建消息队列；
* 不维护动态句柄；
* 不运行配置监听；
* 不启动健康检查循环。

---

# 51. 使用禁令

禁止保存 Application：

```go
type Component struct {
	app *app.Application
}
```

禁止保存 Registry：

```go
type Component struct {
	registry module.Registry
}
```

禁止运行时查询依赖：

```go
logger := container.Resolve("logger")
```

禁止字段注入：

```go
type Service struct {
	Logger logging.Logger `inject:""`
}
```

禁止 Provider 启动 Goroutine：

```go
func New() *Component {
	component := &Component{}
	go component.Run()
	return component
}
```

禁止组件关闭外部注入组件：

```go
func (s *Service) Close(
	ctx context.Context,
) error {
	return s.logger.Close(ctx)
}
```

禁止组件调用：

```go
os.Exit(...)
log.Fatal(...)
```

禁止建立集中式能力映射：

```go
var Capabilities = map[string]any{
	"logger":   zaplog.New,
	"database": gormdb.New,
	"cache":    redis.New,
}
```

禁止定义万能基础设施访问器：

```go
type Infrastructure interface {
	Logger() logging.Logger
	Database() Database
	Cache() Cache
}
```

---

# 52. 新增基础能力的标准流程

## 第一步：判断是否需要公共契约

检查：

* 是否被多个无关组件使用；
* 语义是否稳定；
* 是否需要隔离第三方库；
* 是否存在替换实现；
* 接口是否能保持较小。

不满足时，优先使用消费者局部接口。

---

## 第二步：定义公共契约

例如：

```text
capability/logging
```

契约包只包含接口和值对象。

---

## 第三步：实现第三方适配器

例如：

```text
adapter/logging/zap
```

封装：

* Zap 配置；
* Zap 构造；
* 字段转换；
* Close；
* 可选 Reload。

---

## 第四步：创建实现模块

模块负责：

* 注册配置；
* 注册 Provider；
* 绑定契约；
* 导出契约。

---

## 第五步：应用显式启用

```go
func Modules() []module.Module {
	return []module.Module{
		zaplog.Module{},
	}
}
```

---

## 第六步：消费者依赖公共契约

```go
func NewComponent(
	logger logging.Logger,
) *Component
```

消费者不得依赖 Zap。

---

# 53. 实施路线

## 第一阶段：可运行内核

必须实现：

* Module；
* Registry；
* Provider；
* Binding；
* Export；
* Config；
* Dependency Graph；
* Application Singleton；
* Transactional Build；
* Prepare；
* Start；
* Runner；
* Stop；
* Close；
* Startup Rollback；
* Graceful Shutdown；
* Structured Error；
* Stable Ordering；
* Panic Boundary。

---

## 第二阶段：开发体验

增加：

* Compile Only；
* 依赖图导出；
* Test Registry；
* Test Observer；
* 更清晰的诊断信息；
* Provider 签名错误提示；
* 模块边界诊断；
* 配置来源扩展。

---

## 第三阶段：公共能力

按实际需求增加：

* Logging Contract；
* Zap Adapter；
* Slog Adapter；
* Clock Contract；
* System Clock Adapter；
* ID Generator Contract；
* UUID Adapter。

---

## 第四阶段：最小动态能力

真实需求出现后增加：

* 配置监听；
* Reload；
* RestartRequired；
* Ready；
* Drain。

---

## 暂不进入核心

* 实例代际；
* 依赖子图重建；
* 动态代理；
* 多作用域；
* 命名依赖；
* 集合注入；
* 通信 SDK；
* 业务事件总线；
* 动态插件；
* 分布式治理。

---

# 54. 框架文档体系

建议仓库采用：

```text
docs/
├── README.md
│
├── 01-overview/
│   ├── introduction.md
│   ├── goals.md
│   ├── non-goals.md
│   ├── architecture.md
│   ├── terminology.md
│   └── design-boundaries.md
│
├── 02-getting-started/
│   ├── installation.md
│   ├── first-application.md
│   ├── project-layout.md
│   ├── create-module.md
│   ├── create-component.md
│   ├── configuration.md
│   └── run-and-shutdown.md
│
├── 03-core-concepts/
│   ├── application.md
│   ├── module.md
│   ├── provider.md
│   ├── contract.md
│   ├── binding.md
│   ├── export.md
│   ├── module-visibility.md
│   ├── dependency-graph.md
│   ├── resource-ownership.md
│   ├── lifecycle.md
│   ├── runner-supervision.md
│   ├── configuration-snapshot.md
│   └── error-model.md
│
├── 04-capabilities/
│   ├── capability-contracts.md
│   ├── adapter-design.md
│   ├── logging.md
│   ├── clock.md
│   ├── id-generator.md
│   └── capability-admission.md
│
├── 05-guides/
│   ├── create-infrastructure-capability.md
│   ├── create-adapter.md
│   ├── export-contract.md
│   ├── inject-configuration.md
│   ├── manage-running-component.md
│   ├── manage-resource.md
│   ├── handle-construction-failure.md
│   ├── handle-startup-failure.md
│   ├── graceful-shutdown.md
│   ├── observe-runtime-events.md
│   └── testing.md
│
├── 06-reference/
│   ├── app-api.md
│   ├── module-api.md
│   ├── registry-api.md
│   ├── provider-signatures.md
│   ├── lifecycle-api.md
│   ├── configuration-api.md
│   ├── observer-api.md
│   ├── errors.md
│   └── state-machine.md
│
├── 07-examples/
│   ├── minimal/
│   ├── contract-binding/
│   ├── module-visibility/
│   ├── logging-zap/
│   ├── logging-slog/
│   ├── dependency-chain/
│   ├── lifecycle/
│   ├── runner/
│   ├── construction-rollback/
│   ├── startup-rollback/
│   └── graceful-shutdown/
│
├── 08-internals/
│   ├── registry-design.md
│   ├── graph-compiler.md
│   ├── graph-validation.md
│   ├── instance-constructor.md
│   ├── lifecycle-planner.md
│   ├── lifecycle-executor.md
│   ├── panic-boundary.md
│   ├── runtime-coordinator.md
│   └── performance.md
│
└── 09-decisions/
    ├── adr-001-explicit-module-registration.md
    ├── adr-002-constructor-injection.md
    ├── adr-003-no-service-locator.md
    ├── adr-004-application-singleton-scope.md
    ├── adr-005-module-export-boundary.md
    ├── adr-006-transactional-build.md
    ├── adr-007-runner-supervision.md
    ├── adr-008-capability-contract-layer.md
    ├── adr-009-no-central-capability-map.md
    ├── adr-010-no-universal-infrastructure-interface.md
    └── adr-011-minimal-reload-boundary.md
```

---

# 55. 最终职责边界

```text
Capability Contract
定义公共基础能力的稳定语义。

Adapter
使用第三方库实现公共能力契约。

Module
声明一组基础设施组件及其导出能力。

Provider
描述组件如何创建。

Registry
在构建阶段收集声明。

Dependency Compiler
校验并编译依赖关系。

Instance Constructor
按照拓扑顺序构造实例。

Configuration Manager
提供经过校验的配置事实。

Lifecycle Manager
管理准备、启动、运行、停止和关闭。

Application Runtime
统一协调应用运行和退出。

Composition Root
选择当前应用实际使用的模块和具体实现。
```

---

# 56. 最终核心原则

框架必须始终遵循：

> 契约归属公共能力，实现归属适配器，绑定归属实现模块，选择归属应用组装，生命周期归属运行时，治理内核不感知具体第三方基础设施。

同时遵循：

> 统一公共能力契约的位置和设计规则，但不统一运行时获取入口；基础设施和业务通过构造函数按需接收小接口，不通过万能 Infrastructure、Application 或容器查询能力。

依赖在构建阶段解析。

组件在运行阶段直接调用。

模块只声明基础设施能力。

运行时只协调治理流程。

公共契约不得演变为所有基础设施接口的集中垃圾桶。

具体实现不得泄露第三方类型。

框架不得为尚未出现的需求提前建设复杂运行时平台。
