# Mock WebSocket Data Testing Guide

这个指南将帮你使用模拟数据测试WebSocket实时K线更新功能。

## 🎯 可用的Mock方法

### 方法1: 前端Mock模式 (推荐)
**优点**: 完全在前端控制，不需要后端配置
**适用场景**: 测试前端图表逻辑、UI交互、时间戳处理

### 方法2: 后端Mock数据发送器
**优点**: 测试完整的数据流水线
**适用场景**: 测试Redis→WebSocket→前端的完整链路

### 方法3: 测试页面
**优点**: 独立的测试环境，方便演示
**适用场景**: 功能演示、用户验收测试

---

## 🚀 方法1: 前端Mock模式

### 1. 启用Mock模式
```jsx
<TradingViewChart 
  token={yourToken}
  visible={true}
  mockMode={true}  // 启用mock模式
/>
```

### 2. Mock功能特性
- ✅ **智能时间戳**: 根据选择的间隔自动生成正确的时间戳
- ✅ **价格波动**: 模拟真实的价格变动（基于上一个价格）
- ✅ **间隔支持**: 支持所有间隔（1m, 5m, 15m, 1h, 4h, 1d）
- ✅ **自动更新**: 每5秒自动发送新数据
- ✅ **手动控制**: 可手动发送单条数据

### 3. Mock控制按钮
当`mockMode={true}`时，会显示以下控制按钮：
- **📡 Send Mock Data**: 立即发送一条mock数据
- **▶️ Start Auto Mock**: 开始自动发送数据（每5秒）
- **⏹️ Stop Auto Mock**: 停止自动发送

### 4. 查看调试信息
打开浏览器控制台（F12）查看详细日志：
```
🧪 Starting mock WebSocket data...
📡 Mock WebSocket data: {type: 'kline_update', data: {...}}
📊 Updating chart with mock data: {time: 1751119200, open: 0.000005...}
✅ Processing kline data for matching interval: 1h
```

---

## 🔄 方法2: 后端Mock数据发送器

### 1. 运行Mock数据发送器
```bash
cd websocket
go run mock_data_sender.go
```

### 2. 输出示例
```
🔗 Connected to Redis successfully!
📡 Starting mock kline data generator...
Press Ctrl+C to stop

📤 Published mock data: pair=CT6gBT14micHX9NUuLfdobjbCGSWPZb6M8FqBvdkudp9, interval=1h, price=0.00000532, subscribers=1
📤 Published mock data: pair=7dKYHW1Lr4UJXgumK56FJ3zmLvkwEknLJzL3iPjGaHpZ, interval=1h, price=0.00000498, subscribers=1
```

### 3. 配置参数
在`mock_data_sender.go`中可以修改：
- `pairs`: 要生成数据的交易对地址
- `intervals`: 时间间隔
- `basePrice`: 基础价格
- `volatility`: 波动率
- `ticker`: 发送频率（默认3秒）

---

## 🧪 方法3: 独立测试页面

### 1. 使用测试组件
```jsx
import MockTradingViewTest from './components/MockTradingViewTest';

function App() {
  return <MockTradingViewTest />;
}
```

### 2. 测试页面功能
- **模式切换**: 在Mock模式和Real模式之间切换
- **功能说明**: 显示Mock模式的所有功能
- **实时日志**: 指导用户查看控制台输出
- **视觉反馈**: 清晰的UI指示当前模式

---

## 🔍 测试场景

### 1. 时间戳顺序测试
```javascript
// 测试不同间隔的时间戳是否正确
// Mock模式会为每个间隔生成正确的candleTime
console.log('Testing 1h interval timestamp:', candleTime);
```

### 2. 间隔过滤测试
```javascript
// 测试前端是否正确过滤不匹配的间隔
// 应该看到 "Filtering out kline data" 日志
setInterval('5m'); // 切换到5分钟间隔
```

### 3. 价格连续性测试
```javascript
// 测试价格是否平滑变化（不会跳跃太大）
// Mock模式基于上一个价格生成新价格
```

### 4. 错误处理测试
```javascript
// 测试图表被销毁后的错误处理
// Mock模式会捕获并处理图表销毁错误
```

---

## 📊 监控和调试

### 1. WebSocket服务器日志
```bash
cd websocket && tail -f websocket.log | grep -E "(Broadcasting|Mock)"
```

### 2. Redis监控
```bash
redis-cli monitor | grep "kline:updates"
```

### 3. 前端控制台过滤
```javascript
// 只显示mock相关的日志
console.log('%c Mock Data', 'background: #ff6b35; color: white; padding: 2px 5px; border-radius: 3px;');
```

---

## ⚠️ 注意事项

### 1. Mock模式 vs 真实模式
- Mock模式不会连接真实WebSocket
- Mock模式使用本地定时器，不依赖后端
- 真实模式需要后端服务正常运行

### 2. 时间戳精度
- Mock模式生成的时间戳精确到秒
- 确保candleTime按间隔对齐（如1h间隔对齐到整点）

### 3. 内存管理
- Mock模式会清理定时器避免内存泄漏
- 组件卸载时自动停止mock数据生成

### 4. 开发环境
- Mock模式最适合开发和测试环境
- 生产环境建议使用真实WebSocket连接

---

## 🔧 故障排除

### 问题1: Mock数据没有显示
**解决**: 
1. 检查`mockMode={true}`是否设置
2. 查看控制台是否有错误信息
3. 确认图表已正确初始化

### 问题2: 时间戳顺序错误
**解决**:
1. 确认使用了正确的间隔映射
2. 检查candleTime计算逻辑
3. 验证前端间隔过滤是否生效

### 问题3: 后端Mock发送器连接失败
**解决**:
1. 确认Redis服务正在运行
2. 检查Redis连接配置
3. 验证WebSocket服务器是否在监听

---

## 🎉 最佳实践

1. **开发阶段**: 使用前端Mock模式快速测试UI逻辑
2. **集成测试**: 使用后端Mock发送器测试完整流水线
3. **演示阶段**: 使用测试页面展示功能
4. **生产前**: 使用真实数据进行最终验证

祝你测试愉快！🚀 