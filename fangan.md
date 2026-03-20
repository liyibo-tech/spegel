# Spegel 支持 CRI-O 方案（fangan）

## 1. 现状分析（为什么当前只支持 containerd）

从源码看，当前实现在两处强绑定了 containerd：

1. 数据面（本地镜像/内容读取 + 事件）
- `main.go` 中 `registry` 子命令固定创建 `oci.NewContainerd(...)`。
- `pkg/oci/containerd.go` 是唯一 `oci.Store` 实现，承担：
  - `ListImages/ListContent/Resolve/Descriptor/Open/Subscribe`
  - 基于 containerd EventService 的增量事件订阅。

2. 配置面（将 Spegel 写入运行时镜像镜像配置）
- `configuration` 子命令调用 `oci.AddMirrorConfiguration(...)`。
- `pkg/oci/containerd.go` 中实现的是 containerd `certs.d/*/hosts.toml` 的写入和恢复。
- `cleanup` 子命令也只清理 containerd 配置。

结论：要支持 CRI-O，必须同时补齐“数据面 + 配置面”。

## 2. 目标

在不破坏现有 containerd 行为的前提下，新增 CRI-O 支持：

1. 支持给 CRI-O 写入/恢复镜像镜像配置。
2. 支持从 CRI-O 所在节点读取本地镜像元数据与内容，参与 Spegel 广告与对等分发。
3. 保持现有 `registry/routing/state` 主链路不变（优先复用 `oci.Store` 抽象）。

## 3. 设计原则

1. 最小侵入：优先新增实现，不重写核心链路。
2. 双运行时并存：containerd 与 CRI-O 由统一运行时参数切换。
3. 渐进交付：先可用，再优化性能（尤其是事件订阅）。

## 4. 方案总览

### 4.1 运行时抽象扩展

在 `main.go` 增加运行时参数（建议）：

- `--runtime-kind`：`containerd`（默认）| `crio`
- containerd 现有参数保留：
  - `--containerd-sock`
  - `--containerd-namespace`
  - `--containerd-content-path`
- 新增 CRI-O 参数：
  - `--crio-sock`（默认 `/var/run/crio/crio.sock`）
  - `--crio-storage-path`（默认 `/var/lib/containers/storage`）
  - `--crio-registries-conf`（默认 `/etc/containers/registries.conf`）
  - `--crio-registries-conf-dir`（默认 `/etc/containers/registries.conf.d`）

在 `registryCommand(...)` 中按 `runtime-kind` 创建对应 `oci.Store`。

### 4.2 数据面：新增 CRI-O Store

新增文件（建议）：

- `pkg/oci/crio.go`
- `pkg/oci/crio_test.go`

实现 `oci.Store` 接口，命名 `type CRIO struct`。

建议实现路径（推荐）：

1. 元数据读取：
- 通过 CRI-O / CRI 或 `containers/storage` 获取本地镜像列表（tag/digest/repo）。
- 映射为 `[]oci.Image` 与 `[][]oci.Reference`。

2. 内容读取（关键）：
- 通过 `containers/storage`/OCI blob layout 能力定位 digest 对应内容。
- 实现 `Descriptor/Open`，保证 `registry` 能直接返回 manifest/blob。

3. 事件订阅：
- CRI-O 不一定有与 containerd 等价的事件流接口。
- 第一阶段使用“轮询 + diff”实现 `Subscribe(ctx)`：
  - 周期扫描镜像集合
  - 生成 `Create/Delete` 事件
  - 满足 `state.Track(...)` 现有契约
- 第二阶段再评估接入 CRI-O 事件源（若可行）。

### 4.3 配置面：新增 CRI-O mirror 配置器

当前 `AddMirrorConfiguration/CleanupMirrorConfiguration` 语义是 containerd 专用，建议抽象为运行时配置器：

新增接口（建议）：

```go
type MirrorConfigurator interface {
    Add(ctx context.Context, req MirrorConfigRequest) error
    Cleanup(ctx context.Context, req MirrorCleanupRequest) error
}
```

新增文件（建议）：

- `pkg/oci/mirror_config.go`（接口与工厂）
- `pkg/oci/mirror_config_containerd.go`（迁移现有实现）
- `pkg/oci/mirror_config_crio.go`（新增 CRI-O 实现）

CRIO 配置实现建议：

1. 读写 `/etc/containers/registries.conf` 或 drop-in `registries.conf.d/*.conf`。
2. 为目标 registry 写 `[[registry]]` + `[[registry.mirror]]` 段。
3. 支持：
- 指定镜像目标（mirrorTargets）
- `resolveTags` 语义映射（与 short-name/镜像策略配合）
- `prependExisting`（通过 drop-in 文件保证低侵入，避免重写主配置）
4. cleanup 删除 Spegel 生成的 drop-in 文件并恢复。

## 5. 代码改造清单（按文件）

1. `main.go`
- 新增 `runtime-kind` 与 CRI-O 参数。
- `configuration` / `cleanup` 按 `runtime-kind` 调用不同配置器。
- `registryCommand` 按 `runtime-kind` 创建不同 store。

2. `pkg/oci/oci.go`
- `Store` 接口可保持不变（推荐）。

3. 新增 `pkg/oci/crio.go`
- 实现 `Store` 全接口。

4. 从 `pkg/oci/containerd.go` 中拆分配置逻辑
- 把 `AddMirrorConfiguration/CleanupMirrorConfiguration` 拆到 runtime-configurator 模块。

5. `internal/cleanup/cleanup.go`
- 改为调用统一 `MirrorConfigurator.Cleanup(...)`，不再固定 containerd。

6. 测试
- `pkg/oci/crio_test.go`：单元测试（解析、事件 diff、配置生成）。
- `test/integration/crio/`：新增 CRI-O 集成场景。

## 6. 分阶段落地计划

### Phase 1（低风险，先跑通配置面）

目标：
- `configuration/cleanup` 支持 `runtime-kind=crio`。
- 能正确生成与清理 CRI-O mirror 配置。

收益：
- 先解决运行时接入门槛，便于环境联调。

### Phase 2（核心能力，补齐数据面）

目标：
- `CRIO Store` 实现 `List*/Resolve/Descriptor/Open/Subscribe`。
- 节点可广告本地镜像并对等返回内容。

策略：
- 先用轮询 `Subscribe`，后续再优化事件机制。

### Phase 3（性能与稳定性）

目标：
- 减少 CRI-O 扫描开销（索引缓存、增量刷新）。
- 补充 HA、大规模镜像集、并发拉取压测。

## 7. 风险与应对

1. 风险：CRI-O 内容路径/元数据组织与 containerd 差异大，`Open/Descriptor` 实现复杂。  
应对：优先选官方库（`containers/storage`、`containers/image`）而非手搓文件布局解析。

2. 风险：缺少稳定事件流导致状态延迟。  
应对：Phase 2 使用轮询+diff，允许秒级最终一致；Phase 3 再优化。

3. 风险：直接改写 `registries.conf` 可能影响现网。  
应对：优先 drop-in 文件；写入前备份；cleanup 只删除 Spegel 自有文件。

## 8. 验收标准（Definition of Done）

1. containerd 现有行为回归通过（所有既有单测/集测通过）。
2. CRI-O 模式下：
- `configuration` 成功生成 mirror 配置。
- `cleanup` 成功恢复/清理。
- 本地已存在镜像可被 `state.Track` 广告。
- 其他节点可通过 Spegel 从该节点拉取 manifest/blob。
3. 在 3 节点 CRI-O 集群完成端到端验证：
- A 节点预热镜像，B/C 无镜像；
- B/C 通过 Spegel 成功拉取；
- 外部 registry 不可达时仍可从集群内拉取（命中本地/对等缓存）。

## 9. 推荐实施顺序（执行建议）

1. 先做 Phase 1（配置面），快速验证 CRI-O 运行时接入。  
2. 再做 Phase 2 的最小可用 `CRIO Store`（先轮询事件）。  
3. 最后做性能优化与完整集成测试。  

这个顺序可以把技术风险拆开，避免一次性改太多导致排障困难。
