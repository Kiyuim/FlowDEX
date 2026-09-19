# Windows 启动脚本使用说明

此目录包含用于在 Windows 系统上启动各个服务的批处理脚本。

## 可用脚本

- `start_all.cmd` - 启动所有服务（如有管理员权限会自动设置防火墙规则）
- `start_consumer.cmd` - 仅启动 Consumer 服务
- `start_market.cmd` - 仅启动 Market 服务
- `start_trade.cmd` - 仅启动 Trade 服务
- `start_gateway.cmd` - 仅启动 Gateway 服务
- `start_websocket.cmd` - 仅启动 WebSocket 服务

## 使用方法

### 基本用法

1. 双击需要运行的脚本文件（推荐以管理员身份运行 `start_all.cmd`）
2. 每个脚本都会在单独的命令行窗口中运行对应的服务
3. 服务启动后命令行窗口会保持打开状态，关闭窗口即可停止服务

### 指定配置文件后缀

所有启动脚本都支持通过参数指定配置文件后缀，格式如下：

```
start_服务名.cmd [环境]
```

例如：

- 不带参数时，使用默认配置文件（如 `etc/consumer.yaml`）

  ```
  start_consumer.cmd
  ```

- 指定使用 local 环境的配置文件（如 `etc/consumer-local.yaml`）

  ```
  start_consumer.cmd local
  ```

- 指定使用 dev 环境的配置文件（如 `etc/consumer-dev.yaml`）
  ```
  start_consumer.cmd dev
  ```

### 同时启动所有服务

使用 `start_all.cmd` 可以同时启动所有服务，并可选择指定环境：

```
start_all.cmd [环境]
```

例如：

- 使用默认环境启动所有服务：

  ```
  start_all.cmd
  ```

- 使用 local 环境启动所有服务：
  ```
  start_all.cmd local
  ```

## 配置文件

每个服务都应该有以下配置文件：

1. 默认配置文件：`etc/服务名.yaml`
2. 本地开发配置文件：`etc/服务名-local.yaml`
3. 其他环境配置文件：`etc/服务名-环境名.yaml`

## 防火墙设置

`start_all.cmd` 脚本在执行时会检查是否有管理员权限：

- 如果有管理员权限，会自动为 Go 程序和服务端口添加防火墙规则
- 如果没有管理员权限，会跳过防火墙设置，直接启动服务

**建议以管理员身份运行 `start_all.cmd`** 以避免在启动服务时出现 Windows 防火墙弹窗提示。

### 如何以管理员身份运行脚本：

1. 右键点击 `start_all.cmd`
2. 选择"以管理员身份运行"

## 乱码问题解决

所有脚本都使用 `chcp 65001` 命令设置 UTF-8 编码，以解决中文显示乱码问题。

## 自定义配置

如需自定义配置，请直接编辑对应服务的启动脚本，修改 `set` 命令后的值。
