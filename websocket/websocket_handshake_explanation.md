# WebSocket握手过程详解

## 1. 客户端发起HTTP请求

当你调用 `new WebSocket(url)` 时，浏览器会发送一个特殊的HTTP请求：

```http
GET /ws/tokens?chain_id=100000 HTTP/1.1
Host: 118.194.235.63:8086
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13
Origin: http://localhost:3000
```

**关键字段解释：**
- `Upgrade: websocket` - 告诉服务器要升级到WebSocket协议
- `Connection: Upgrade` - 表示连接升级
- `Sec-WebSocket-Key` - 客户端生成的随机key，用于安全验证
- `Sec-WebSocket-Version: 13` - WebSocket协议版本

## 2. 服务器响应握手

服务器收到请求后，如果同意升级，会返回：

```http
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

**关键字段解释：**
- `101 Switching Protocols` - 状态码表示协议切换成功
- `Sec-WebSocket-Accept` - 服务器根据客户端的key生成的确认值

## 3. 连接建立成功

握手成功后：
- HTTP连接升级为WebSocket连接
- 可以双向发送数据帧
- 触发客户端的 `onopen` 事件

## 4. 数据帧结构

WebSocket数据以帧的形式传输：

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-------+-+-------------+-------------------------------+
|F|R|R|R| opcode|M| Payload len |    Extended payload length    |
|I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
|N|V|V|V|       |S|             |   (if payload len==126/127)   |
| |1|2|3|       |K|             |                               |
+-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
|     Extended payload length continued, if payload len == 127  |
+ - - - - - - - - - - - - - - - +-------------------------------+
|                               |Masking-key, if MASK set to 1  |
+-------------------------------+-------------------------------+
| Masking-key (continued)       |          Payload Data         |
+-------------------------------- - - - - - - - - - - - - - - - +
:                     Payload Data continued ...                :
+ - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
|                     Payload Data continued ...                |
+---------------------------------------------------------------+
```

## 5. 关闭连接

关闭连接时发送关闭帧：
- 状态码：1000 (正常关闭), 1001 (端点离开), 1002 (协议错误) 等
- 可选的关闭原因 