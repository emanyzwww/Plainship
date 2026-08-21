# shared - 跨端共享模块

放客户端与服务端共同依赖的契约与工具, 当前只有:

- `proto`: 部署协议 (DeployRequest / DeployResponse / DeployPath), 见 `core/distribution` 与 `server`.

规则: 只放两边都认的纯数据类型与常量, 不引入任一端的具体实现; 客户端内部共享用 `client/core/pipeline`, 不要挪到这里.
