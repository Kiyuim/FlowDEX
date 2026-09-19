# Market服务文档

## 一、Market服务整体架构

### 1.1 服务定位

Market服务是一个**行情数据中心服务**，作为gRPC服务为其他服务（Trade、前端API）提供市场数据查询能力。

**核心职责：**
- 提供交易对(Pair)信息查询
- 提供Token基础信息查询
- 提供K线(Kline)数据查询
- 提供原生代币(SOL)价格查询
- 提供Pump.fun Token列表查询
- 管理K线数据的多级缓存（内存+Redis+MySQL）
- 通过Redis Pub/Sub推送新Token信息到WebSocket

**技术特点：**
- **多级缓存架构**：内存缓存 → Redis缓存 → MySQL持久化
- **高性能查询**：使用SingleFlight防止缓存击穿
- **定时任务**：定期更新Pump Token热门列表缓存
- **数据压缩**：K线数据以CSV格式压缩存储在Redis中

---

## 二、Market服务核心业务流程

### 2.1 服务初始化流程

```go
func main() {
    // 1. 加载配置
    var c config.Config
    conf.MustLoad(*configFile, &c)
    
    // 2. 初始化服务上下文（MySQL + Redis）
    svcCtx := svc.NewServiceContext(c)
    
    // 3. 初始化Redis客户端
    rds.Init(&redis.RedisKeyConf{...})
    
    // 4. 初始化缓存系统（内存缓存 + Redis缓存）
    cache.Init(svcCtx, nil)
    
    // 5. 启动gRPC服务器
    s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
        market.RegisterMarketServer(grpcServer, server.NewMarketServer(svcCtx))
        reflection.Register(grpcServer)
    })
    
    // 6. 启动定时任务服务组
    serviceGroup := service.NewServiceGroup()
    pumpTicker := ticker.NewPumpTicker(svcCtx)  // Pump Token列表更新定时器
    serviceGroup.Add(pumpTicker)
    go serviceGroup.Start()
    
    // 7. 启动服务
    s.Start()
}
```

**初始化关键点：**
1. **ServiceContext包含**：MySQL连接池、Redis客户端
2. **缓存系统包含**：KlineCache（内存）、KlineRedisCache（Redis）
3. **定时任务包含**：
   - `PumpTicker.UpdateCache()`：每1分钟更新Completing/Completed状态的Pump Token列表
   - `PumpTicker.UpdateNewCreationCache()`：每3秒更新NewCreation状态的Pump Token列表

---

## 三、核心RPC接口详解

### 3.1 GetPairInfoByToken - 查询交易对信息

**功能说明：**  
根据Token地址查询该Token的主要交易对信息（价格、流动性、市值等）。

**接口定义：**
```protobuf
message GetPairInfoByTokenRequest {
  int64 chain_id = 1;
  string token_address = 2;
}

message GetPairInfoByTokenResponse {
  int64 chain_id = 1;                     // 链ID
  string address = 2;                     // 交易对地址
  string name = 3;                        // DEX名称（如PumpFun、Raydium V4）
  string factory_address = 4;             // 工厂合约地址
  string base_token_address = 5;          // Base Token地址（通常是SOL/WSOL）
  string token_address = 6;               // Token地址
  string base_token_symbol = 7;           // Base Token符号（SOL）
  string token_symbol = 8;                // Token符号
  int64 base_token_decimal = 9;           // Base Token精度
  int64 token_decimal = 10;               // Token精度
  bool base_token_is_native_token = 11;   // Base Token是否为原生币
  bool base_token_is_token0 = 12;         // Base Token是否为token0
  double init_base_token_amount = 13;     // 初始Base Token流动性
  double init_token_amount = 14;          // 初始Token流动性
  double current_base_token_amount = 15;  // 当前Base Token流动性
  double current_token_amount = 16;       // 当前Token流动性
  double fdv = 17;                        // 完全稀释市值
  double mkt_cap = 18;                    // 市值
  double token_price = 19;                // Token价格（USD）
  double base_token_price = 20;           // Base Token价格（USD）
  int64 block_num = 21;                   // 创建区块高度
  int64 block_time = 22;                  // 创建时间戳
  double highest_token_price = 23;        // 历史最高价
  int64 latest_trade_time = 24;           // 最后交易时间戳
}
```

**业务逻辑：**
```go
func (l *GetPairInfoByTokenLogic) GetPairInfoByToken(in *market.GetPairInfoByTokenRequest) (*market.GetPairInfoByTokenResponse, error) {
    // 1. 根据链ID和Token地址查询交易对
    pairModel := solmodel.NewPairModel(l.svcCtx.DB)
    pair, err := pairModel.FindOneByChainIdTokenAddress(l.ctx, in.ChainId, in.TokenAddress)
    if err != nil {
        return nil, err
    }
    
    // 2. 校验数据有效性
    if pair == nil {
        return nil, fmt.Errorf("pairInfo is nil for token: %s", in.TokenAddress)
    }
    
    // 3. 组装返回数据（字段映射）
    return &market.GetPairInfoByTokenResponse{
        ChainId:                in.ChainId,
        Address:                pair.Address,
        Name:                   pair.Name,
        BaseTokenAddress:       pair.BaseTokenAddress,
        TokenAddress:           pair.TokenAddress,
        CurrentBaseTokenAmount: pair.CurrentBaseTokenAmount,
        CurrentTokenAmount:     pair.CurrentTokenAmount,
        Fdv:                    pair.Fdv,
        MktCap:                 pair.MktCap,
        TokenPrice:             pair.TokenPrice,
        BaseTokenPrice:         pair.BaseTokenPrice,
        // ... 其他字段
    }, nil
}
```

**调用方：**
- **Trade服务**：创建市价单/限价单时，需要获取交易对信息（精度、DEX类型、价格等）
- **前端API**：Token详情页展示交易对信息

---

### 3.2 GetKline - 查询K线数据（不是重点，了解即可）

**功能说明：**  
查询指定交易对的K线数据，支持多种时间周期（1m/5m/15m/1h/4h/12h/1d）。

**接口定义：**
```protobuf
message GetKlineRequest {
  int64 chain_id = 1;
  string pair_address = 2;
  string interval = 3;          // 时间周期：1m/5m/15m/1h/4h/12h/1d
  int64 from_timestamp = 4;     // 起始时间戳（秒）
  int64 to_timestamp = 5;       // 结束时间戳（秒）
  int64 limit = 6;              // 最多返回条数
  int64 must_count_back = 7;    // 是否必须回填缺失数据
}

message GetKlineResponse {
  repeated NewKline list = 1;
}

message NewKline {
  double open = 4;              // 开盘价
  double high = 5;              // 最高价
  double low = 6;               // 最低价
  double close = 7;             // 收盘价
  double volume_token = 13;     // 成交量（Token数量）
  int64 candle_time = 14;       // K线时间戳（秒）
}
```

**多级缓存架构：**

```
查询K线数据流程：
1. 查询Redis缓存（ZSet结构）
   ├─ 命中：直接返回
   └─ 未命中：执行步骤2
   
2. 查询内存缓存（KlineCache）
   ├─ 有数据：合并MySQL历史数据后写入Redis
   └─ 无数据：执行步骤3
   
3. 查询MySQL数据库
   ├─ 分月表查询：trade_kline_{interval}_{month}
   ├─ 跨月查询：自动合并多个月份的数据
   └─ 返回结果并写入Redis
```

**核心实现：**

#### 3.2.1 查询入口
```go
func (l *GetKlineLogic) GetKline(in *market.GetKlineRequest) (*marketclient.GetKlineResponse, error) {
    // 使用SingleFlight防止缓存击穿
    klines, err := l.doChanGetKlineData(l.ctx, klineSingleLight, in)
    if err != nil {
        return nil, err
    }
    
    // 组装返回结果
    res := &marketclient.GetKlineResponse{}
    for _, kline := range klines.List {
        res.List = append(res.List, &marketclient.NewKline{
            Open:        kline.Open,
            High:        kline.High,
            Low:         kline.Low,
            Close:       kline.Close,
            CandleTime:  kline.CandleTime,
            VolumeToken: kline.VolumeToken,
        })
    }
    return res, nil
}
```

#### 3.2.2 SingleFlight防止缓存击穿
```go
func (l *GetKlineLogic) doChanGetKlineData(ctx context.Context, g *singleflight.Group, in *marketclient.GetKlineRequest) (*marketclient.Klines, error) {
    // 使用pair_address + interval + to_timestamp + limit作为Key
    ch := g.DoChan(fmt.Sprintf("%s_%s_%d_%d", in.PairAddress, in.Interval, in.ToTimestamp, in.Limit), func() (interface{}, error) {
        return l.getKline(in)
    })
    
    select {
    case <-ctx.Done():
        return &marketclient.Klines{}, ctx.Err()
    case ret := <-ch:
        return ret.Val.(*marketclient.Klines), ret.Err
    }
}
```

**SingleFlight作用：**  
当多个请求同时查询相同的K线数据时（如前端页面被多个用户同时打开），只有第一个请求真正执行查询，其他请求等待并共享结果。

#### 3.2.3 Redis缓存查询
```go
func (ks *klineRedisCache) FetchKline(ctx context.Context, option KlineQueryOption) ([]*datakline.Kline, bool) {
    // Redis Key格式：kline:{chainId}:{pairAddress}:{interval}
    key := ks.key(option.ChainId, option.PairAddr, option.Interval)
    
    // 使用ZSet按时间范围查询（倒序）
    list, err := rds.Client.ZrevrangebyscoreWithScoresCtx(ctx, key, option.From, option.To)
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return nil, false  // 未命中
        }
        return nil, false
    }
    
    // 解压缩K线数据（CSV格式 → Kline结构体）
    res := make([]*datakline.Kline, 0, option.Limit)
    for _, str := range list {
        kline, err := ks.decompress(str.Key)
        if err != nil {
            continue
        }
        kline.ChainId = int64(option.ChainId)
        kline.Interval = string(option.Interval)
        if kline.CandleTime < option.To {
            res = append(res, kline)
        }
        if len(res) >= int(option.Limit) {
            break
        }
    }
    return res, true
}
```

**Redis数据结构：**
- **Key格式**：`kline:900:xxxxx:1m`（链ID:交易对地址:时间周期）
- **数据结构**：ZSet（Sorted Set）
- **Score**：K线时间戳（秒）
- **Value**：压缩的CSV字符串（15个字段用逗号分隔）

**CSV压缩格式：**
```
pairAddr,candleTime,open,close,high,low,mcapOpen,mcapHigh,mcapLow,mcapClose,amountUsd,volumeToken,buyCount,sellCount,totalCount
```

**示例：**
```
"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v,1704067200,0.9998,1.0002,1.0005,0.9995,0,0,0,0,125000.5,125500,45,38,83"
```

#### 3.2.4 数据压缩与解压
```go
// 压缩：将Kline结构体转为CSV字符串
func (ks *klineRedisCache) compress(pb *datakline.Kline) string {
    return strings.Join(
        []string{
            pb.PairAddr,
            util.Int64ToString(pb.CandleTime),
            util.Float64ToString(pb.Open),
            util.Float64ToString(pb.Close),
            util.Float64ToString(pb.High),
            util.Float64ToString(pb.Low),
            util.Float64ToString(pb.McapOpen),
            util.Float64ToString(pb.McapHigh),
            util.Float64ToString(pb.McapLow),
            util.Float64ToString(pb.McapClose),
            util.Float64ToString(pb.AmountUsd),
            util.Float64ToString(pb.VolumeToken),
            strconv.Itoa(int(pb.BuyCount)),
            strconv.Itoa(int(pb.SellCount)),
            strconv.Itoa(int(pb.TotalCount)),
        },
        ",")
}

// 解压：将CSV字符串解析为Kline结构体
func (ks *klineRedisCache) decompress(str string) (*datakline.Kline, error) {
    parts := strings.Split(str, ",")
    if len(parts) != 15 {
        return nil, fmt.Errorf("invalid string len, need:15 get:%d", len(parts))
    }
    
    return &datakline.Kline{
        PairAddr:    parts[0],
        CandleTime:  util.StringToInt64(parts[1]),
        Open:        util.StringToFloat64(parts[2]),
        Close:       util.StringToFloat64(parts[3]),
        High:        util.StringToFloat64(parts[4]),
        Low:         util.StringToFloat64(parts[5]),
        McapOpen:    util.StringToFloat64(parts[6]),
        McapHigh:    util.StringToFloat64(parts[7]),
        McapLow:     util.StringToFloat64(parts[8]),
        McapClose:   util.StringToFloat64(parts[9]),
        AmountUsd:   util.StringToFloat64(parts[10]),
        VolumeToken: util.StringToFloat64(parts[11]),
        BuyCount:    util.StringToInt64(parts[12]),
        SellCount:   util.StringToInt64(parts[13]),
        TotalCount:  util.StringToInt64(parts[14]),
    }, nil
}
```

**压缩优势：**
- 原始Protobuf格式：约200字节/根K线
- CSV压缩格式：约100字节/根K线
- **节省50%存储空间**

#### 3.2.5 MySQL数据查询
```go
func (repo *KlineMysqlRepo) QueryKline(ctx context.Context, interval constants.KlineInterval, chainId int64, pairAddress string, fromTime, toTime int64, limit int) (result []Kline, err error) {
    // 跨月查询处理
    if repo.TableName(interval, fromTime) != repo.TableName(interval, toTime) {
        // 查询起始月份数据
        var resultFirst []Kline
        err = repo.db.WithContext(ctx).Table(repo.TableName(interval, fromTime)).
            Where("chain_id = ? and pair_address = ? and candle_time between ? and ?", chainId, pairAddress, fromTime, toTime).
            Order("id desc").Limit(limit).Scan(&resultFirst).Error
        
        // 查询结束月份数据
        var resultLast []Kline
        err = repo.db.WithContext(ctx).Table(repo.TableName(interval, toTime)).
            Where("chain_id = ? and pair_address = ? and candle_time between ? and ?", chainId, pairAddress, fromTime, toTime).
            Order("id desc").Limit(limit).Scan(&resultLast).Error
        
        // 合并结果
        result = append(resultLast, resultFirst...)
    } else {
        // 同月查询
        err = repo.db.WithContext(ctx).Table(repo.TableName(interval, toTime)).
            Where("chain_id = ? and pair_address = ? and candle_time between ? and ?", chainId, pairAddress, fromTime, toTime).
            Order("id desc").Limit(limit).Scan(&result).Error
    }
    return result, err
}

// 表名生成规则
func (repo *KlineMysqlRepo) TableName(interval constants.KlineInterval, candleTime int64) string {
    if candleTime == 0 {
        return fmt.Sprintf("trade_kline_%s", interval)
    }
    // 按月分表：trade_kline_1m_01, trade_kline_1m_02, ...
    return fmt.Sprintf("trade_kline_%s_%02d", interval, time.Unix(candleTime, 0).UTC().Month())
}
```

**分月表设计：**
- **表命名规则**：`trade_kline_{interval}_{month}`
- **示例表名**：`trade_kline_1m_01`（1分钟K线，1月份）
- **优势**：
  - 单表数据量可控（最多31天数据）
  - 历史数据可按月归档
  - 查询性能更好（索引更小）

#### 3.2.6 数据补全逻辑
```go
func (l *GetKlineLogic) supplementResult(list []*marketclient.Kline, fromTime, toTime int64, interval string, count int64) []*marketclient.Kline {
    if len(list) <= 0 {
        return nil
    }
    
    intervalSecond := int64(constants2.KlineIntervalSecondsMap[interval])
    
    // 1. 填补中间缺失的K线
    inner := make([]*marketclient.Kline, 0)
    for i := 0; i < len(list)-1; i++ {
        currentKline := list[i]
        nextKline := list[i+1]
        timeDiff := currentKline.CandleTime - nextKline.CandleTime
        
        // 如果时间间隔 > 标准间隔，说明中间有缺失
        if timeDiff > intervalSecond {
            // 补充缺失的K线（价格使用前一根K线的收盘价）
            for j := nextKline.CandleTime + intervalSecond; j < currentKline.CandleTime; j += intervalSecond {
                missingKline := &marketclient.Kline{
                    ChainId:    currentKline.ChainId,
                    Interval:   interval,
                    PairAddr:   currentKline.PairAddr,
                    Open:       currentKline.Open,
                    High:       currentKline.Open,
                    Low:        currentKline.Open,
                    Close:      currentKline.Open,
                    CandleTime: j,
                }
                inner = append(inner, missingKline)
            }
        }
    }
    
    // 2. 向前补全（如果最旧的K线时间 > fromTime）
    oldestKline := list[len(list)-1]
    if oldestKline.CandleTime > fromTime {
        num := (oldestKline.CandleTime - fromTime) / intervalSecond
        num = min(num, count)
        for i := 1; i < int(num)+1; i++ {
            list = append(list, &marketclient.Kline{
                ChainId:    oldestKline.ChainId,
                Interval:   oldestKline.Interval,
                PairAddr:   oldestKline.PairAddr,
                Open:       oldestKline.Open,
                High:       oldestKline.Open,
                Low:        oldestKline.Open,
                Close:      oldestKline.Open,
                CandleTime: oldestKline.CandleTime - int64(i)*intervalSecond,
            })
        }
    }
    
    // 3. 向后补全（如果最新的K线时间 < toTime）
    if list[0].CandleTime < toTime {
        num := (toTime - list[0].CandleTime) / intervalSecond
        for i := 1; i < int(num)+1; i++ {
            list = append(list, &marketclient.Kline{
                ChainId:    list[0].ChainId,
                Interval:   list[0].Interval,
                PairAddr:   list[0].PairAddr,
                Open:       list[0].Close,
                High:       list[0].Close,
                Low:        list[0].Close,
                Close:      list[0].Close,
                CandleTime: list[0].CandleTime + int64(i)*intervalSecond,
            })
        }
    }
    
    list = append(list, inner...)
    return list
}
```

**数据补全策略：**
1. **中间缺失**：使用前一根K线的收盘价（Open=High=Low=Close）
2. **头部缺失**：使用最旧K线的开盘价
3. **尾部缺失**：使用最新K线的收盘价

**为什么需要补全？**  
- 交易对在某些时间段可能没有交易，导致K线数据缺失
- 前端图表需要连续的数据点，否则会出现断层
- 补全后保证返回的K线数量 = limit

#### 3.2.7 数据美化处理
```go
func (l *GetKlineLogic) prettyResult(list []*marketclient.Kline, count int64) {
    if len(list) <= 0 {
        return
    }
    
    // 按时间升序排序
    sort.Slice(list, func(i, j int) bool {
        return list[i].CandleTime < list[j].CandleTime
    })
    
    // 如果数据量超过limit，移除第一根（最旧的）
    frontInfo := list[0]
    if len(list) > int(count) {
        list = list[1:]
        list[0].Open = frontInfo.Close
        list[0].McapOpen = frontInfo.McapClose
        frontInfo = list[0]
    }
    
    // 保证K线连续性：每根K线的Open = 上一根的Close
    for i := 1; i < len(list); i++ {
        list[i].Open = frontInfo.Close
        list[i].McapOpen = frontInfo.McapClose
        frontInfo = list[i]
    }
    
    // 保证High >= Open, Low <= Open
    for i := 1; i < len(list); i++ {
        list[i].High = max(list[i].High, list[i].Open)
        list[i].Low = min(list[i].Low, list[i].Open)
        list[i].McapHigh = max(list[i].McapHigh, list[i].McapOpen)
        list[i].McapLow = min(list[i].McapLow, list[i].McapOpen)
    }
}
```

**美化目的：**
1. **连续性**：保证每根K线的开盘价 = 上一根的收盘价
2. **合理性**：保证High ≥ Open/Close, Low ≤ Open/Close
3. **一致性**：市值字段也做相同处理

---

### 3.3 GetNativeTokenPrice - 查询原生代币价格

**功能说明：**  
查询指定时间点的原生代币（SOL）价格。

**接口定义：**
```protobuf
message GetNativeTokenPriceRequest {
  int64 chain_id = 1;
  string search_time = 2;  // 格式：2006-01-02 15:04:05
}

message GetNativeTokenPriceResponse {
  double base_token_price_usd = 1;  // SOL的USD价格
}
```

**业务逻辑：**
```go
func (l *GetNativeTokenPriceLogic) GetNativeTokenPrice(in *market.GetNativeTokenPriceRequest) (*market.GetNativeTokenPriceResponse, error) {
    resp := &market.GetNativeTokenPriceResponse{
        BaseTokenPriceUsd: 0,
    }
    
    // 1. 解析查询时间
    searchTime, err := time.Parse(time.DateTime, in.SearchTime)
    if err != nil {
        return nil, nil
    }
    
    // 2. 从trade表查询最近的交易记录，获取base_token_price_usd字段
    tradeModel := solmodel.NewTradeModel(l.svcCtx.DB)
    price, err := tradeModel.GetNativeTokenPrice(l.ctx, in.ChainId, searchTime)
    if err != nil {
        return resp, err
    }
    
    resp.BaseTokenPriceUsd = price
    return resp, nil
}
```

**数据来源：**  
从`trade`表中查询指定时间最近的交易记录，获取`base_token_price_usd`字段（该字段由Consumer服务在解析交易时写入）。

**调用方：**
- **Trade服务**：计算限价单目标价格时，如果pair表中的base_token_price为0，则调用此接口获取实时SOL价格

---

### 3.4 PushTokenInfo - 推送Token信息到WebSocket

**功能说明：**  
接收Consumer服务推送的新Token信息（PumpFun新创建的Token），通过Redis Pub/Sub推送给WebSocket服务，实时通知前端。

**接口定义：**
```protobuf
message PushTokenInfoRequest {
  int64 chain_id = 1;
  string token_address = 2;
  string pair_address = 3;
  double token_price = 4;
  double mkt_cap = 5;
  string token_name = 6;
  string token_symbol = 7;
  string token_icon = 8;
  int64 launch_time = 9;
  int64 hold_count = 10;
  double change_24 = 11;
  int64 txs_24h = 12;
  int32 pump_status = 13;  // 0:新创建 1:进行中 2:已完成
}

message PushTokenInfoResponse {
  int64 chain_id = 1;
  string token_address = 2;
  uint32 txs_24h = 3;
  string vol_24h = 4;
  string change_24 = 5;
  string token_price = 6;
  string mkt_cap = 7;
}
```

**业务流程：**
```go
func (l *PushTokenInfoLogic) PushTokenInfo(in *market.PushTokenInfoRequest) (*market.PushTokenInfoResponse, error) {
    // 1. 使用Consumer传入的Token元数据
    tokenName := in.TokenName
    tokenSymbol := in.TokenSymbol
    tokenIcon := in.TokenIcon
    launchTime := in.LaunchTime
    var twitterUsername, telegram string
    var holdCount int64
    
    // 如果没有提供启动时间，使用当前时间
    if launchTime == 0 {
        launchTime = time.Now().Unix()
    }
    
    // 2. 从数据库增强Token信息（社交媒体、持仓人数）
    tokenModel := solmodel.NewTokenModel(l.svcCtx.DB)
    tokenInfo, err := tokenModel.FindOneByChainIdAddress(l.ctx, in.ChainId, in.TokenAddress)
    if err == nil && tokenInfo != nil {
        // 使用数据库值（如果Consumer没提供）
        if tokenName == "" {
            tokenName = tokenInfo.Name
        }
        if tokenIcon == "" {
            tokenIcon = tokenInfo.Icon
        }
        twitterUsername = tokenInfo.TwitterUsername
        telegram = tokenInfo.Telegram
        
        // 计算持仓人数（从sol_token_account表统计）
        solTokenAccountModel := solmodel.NewSolTokenAccountModel(l.svcCtx.DB)
        holders, err := solTokenAccountModel.CountByTokenAddressWithTime(l.ctx, in.ChainId, in.TokenAddress, tokenInfo.CreatedAt)
        if err != nil {
            l.Errorf("Failed to count token holders: %v", err)
            holdCount = 0
        } else {
            holdCount = holders
        }
    } else {
        l.Infof("Token info not found in database for %s: %v", in.TokenAddress, err)
        holdCount = 0
    }
    
    // 3. 组装完整的Token数据（Consumer数据 + 数据库增强数据）
    tokenData := map[string]interface{}{
        "chain_id":         in.ChainId,
        "token_address":    in.TokenAddress,
        "pair_address":     in.PairAddress,
        "token_price":      in.TokenPrice,
        "mkt_cap":          in.MktCap,
        "token_name":       tokenName,
        "token_symbol":     tokenSymbol,
        "token_icon":       tokenIcon,
        "launch_time":      launchTime,
        "hold_count":       holdCount,              // 真实持仓人数
        "change_24":        in.Change_24,
        "txs_24h":          in.Txs_24H,
        "pump_status":      in.PumpStatus,
        "twitter_username": twitterUsername,        // 社交媒体
        "telegram":         telegram,               // 社交媒体
    }
    
    // 4. 转为JSON
    jsonData, err := json.Marshal(tokenData)
    if err != nil {
        l.Errorf("Failed to marshal token data: %v", err)
        return &market.PushTokenInfoResponse{...}, nil
    }
    
    // 5. 发布到Redis频道（WebSocket服务订阅此频道）
    channel := "pump_token_new"
    _, err = l.svcCtx.RDS.Publish(channel, string(jsonData))
    if err != nil {
        l.Errorf("Failed to publish token info to Redis: %v", err)
        return &market.PushTokenInfoResponse{...}, nil
    }
    
    l.Infof("Successfully published token info to channel %s", channel)
    
    return &market.PushTokenInfoResponse{
        ChainId:      in.ChainId,
        TokenAddress: in.TokenAddress,
        Txs_24H:      0,
        Vol_24H:      "0",
        Change_24:    "0",
        TokenPrice:   "0",
        MktCap:       "0",
    }, nil
}
```

**数据流转：**
```
Consumer服务（解析PumpFun创建Token交易）
    ↓
调用 Market.PushTokenInfo() gRPC接口
    ↓
Market服务从数据库增强Token信息
    ↓
发布到Redis频道：pump_token_new
    ↓
WebSocket服务订阅此频道
    ↓
实时推送给前端用户（SSE/WebSocket）
```

**为什么需要数据库增强？**
- Consumer服务解析交易时，只能获取链上数据（Token地址、价格、市值）
- Token的社交媒体信息（Twitter、Telegram）存储在数据库的`token`表中
- 持仓人数需要从`sol_token_account`表统计

---

### 3.5 GetPumpTokenList - 查询Pump Token列表

**功能说明：**  
查询PumpFun协议的Token列表，支持按状态筛选（新创建/进行中/已完成）。

**接口定义：**
```protobuf
message GetPumpTokenListRequest {
  int64 chain_id = 1;
  int32 pump_status = 2;     // 0:新创建 1:进行中 2:已完成
  string sorted_type = 3;    // 排序类型（预留字段）
  string honeypot_filter = 4; // 蜜罐过滤（预留字段）
  int32 page_no = 5;
  int32 page_size = 6;
}

message GetPumpTokenListResponse {
  repeated PumpTokenItem list = 1;
  int32 total = 2;
}

message PumpTokenItem {
  int64 chain_id = 1;
  string chain_icon = 2;
  string token_address = 3;
  string token_icon = 4;
  string token_name = 5;
  int64 launch_time = 6;
  double mkt_cap = 7;
  int64 hold_count = 8;
  uint32 txs_24h = 9;
  double vol_24h = 10;
  double domestic_progress = 11;  // Bonding Curve进度（0-1）
  string twitter_username = 12;
  string telegram = 13;
  double change24 = 14;
  string pair_address = 15;
}
```

**业务逻辑：**
```go
func (l *GetPumpTokenListLogic) GetPumpTokenList(in *market.GetPumpTokenListRequest) (*market.GetPumpTokenListResponse, error) {
    var resultList []*market.PumpTokenItem
    var pairList []solmodel.Pair
    pairModel := solmodel.NewPairModel(l.svcCtx.DB)
    redisClient := l.svcCtx.RDS
    
    // Redis缓存Key：pump-token-list-{pumpStatus}
    pairCacheKey := fmt.Sprint("pump-token-list-", in.PumpStatus)
    
    // 1. 尝试从Redis缓存读取
    cachedData, err := redisClient.Get(pairCacheKey)
    if err == nil && cachedData != "" {
        // 命中缓存，直接返回
        err = json.Unmarshal([]byte(cachedData), &resultList)
        if err != nil {
            logx.Errorf("Failed to unmarshal cached data: %v", err)
            return nil, err
        }
        
        return &market.GetPumpTokenListResponse{
            List:  resultList,
            Total: int32(len(resultList)),
        }, nil
    }
    
    // 2. 未命中缓存，从数据库查询
    in.PageNo = 1
    in.PageSize = 10  // 固定返回Top 10
    
    // 根据pump_status查询不同的数据
    switch in.PumpStatus {
    case constants.PumpStatusNewCreation:  // 0: 新创建
        pairList, err = pairModel.FindLatestPumpLimit(l.ctx, in.PageNo, in.PageSize)
    case constants.PumpStatusCompleting:   // 1: 进行中
        pairList, err = pairModel.FindLatestCompletingPumpLimit(l.ctx, in.PageNo, in.PageSize)
    case constants.PumpStatusCompleted:    // 2: 已完成
        pairList, err = pairModel.FindLatestCompletePumpLimit(l.ctx, in.PageNo, in.PageSize)
    }
    
    if err != nil {
        return nil, err
    }
    if len(pairList) == 0 {
        return &market.GetPumpTokenListResponse{
            List:  []*market.PumpTokenItem{},
            Total: 0,
        }, nil
    }
    
    // 3. 批量查询Token详细信息
    tokenAddresses := make([]string, 0)
    for _, pair := range pairList {
        if pair.TokenAddress != "" {
            tokenAddresses = append(tokenAddresses, pair.TokenAddress)
        }
    }
    
    tokenModel := solmodel.NewTokenModel(l.svcCtx.DB)
    tokenList, err := tokenModel.FindAllByAddresses(l.ctx, in.ChainId, tokenAddresses)
    if err != nil {
        return nil, err
    }
    
    tokenMap := make(map[string]*solmodel.Token)
    for _, token := range tokenList {
        tokenMap[token.Address] = &token
    }
    
    // 4. 批量统计持仓人数
    tokenHolderMap := make(map[string]int64)
    solTokenAccountModel := solmodel.NewSolTokenAccountModel(l.svcCtx.DB)
    for _, tokenAddress := range tokenAddresses {
        token := tokenMap[tokenAddress]
        if token != nil {
            // 统计该Token创建后的持仓账户数量
            holders, err := solTokenAccountModel.CountByTokenAddressWithTime(l.ctx, in.ChainId, tokenAddress, token.CreatedAt)
            if err != nil {
                l.Errorf("GetPumpTokenList: countByTokenAddressWithTime failed: %v", err)
                tokenHolderMap[tokenAddress] = 0
                continue
            }
            tokenHolderMap[tokenAddress] = holders
        }
    }
    
    // 5. 组装返回列表
    list := make([]*market.PumpTokenItem, 0)
    for _, pair := range pairList {
        token := tokenMap[pair.TokenAddress]
        var tokenIcon, twitterUsername, telegram string
        if token != nil {
            tokenIcon = token.Icon
            twitterUsername = token.TwitterUsername
            telegram = token.Telegram
        }
        
        list = append(list, &market.PumpTokenItem{
            ChainId:          pair.ChainId,
            ChainIcon:        chain.ChainId2ChainIcon(in.ChainId),
            TokenAddress:     pair.TokenAddress,
            TokenIcon:        tokenIcon,
            TokenName:        pair.TokenSymbol,
            LaunchTime:       pair.BlockTime.Unix(),
            MktCap:           pair.Fdv,
            HoldCount:        tokenHolderMap[pair.TokenAddress],
            DomesticProgress: pair.PumpPoint,  // Bonding Curve进度
            TwitterUsername:  twitterUsername,
            Telegram:         telegram,
        })
    }
    
    // 6. 写入Redis缓存（7天过期）
    listData, err := json.Marshal(list)
    if err != nil {
        logx.Errorf("Failed to marshal list: %v", err)
        return nil, err
    }
    
    err = redisClient.Set(pairCacheKey, string(listData))
    if err != nil {
        logx.Errorf("Failed to set list in Redis: %v", err)
        return nil, err
    }
    
    err = redisClient.Expire(pairCacheKey, 60*60*24*7) // 7天
    if err != nil {
        logx.Errorf("Failed to set expiration for list in Redis: %v", err)
        return nil, err
    }
    
    return &market.GetPumpTokenListResponse{
        List:  list,
        Total: 50,  // 固定返回50（前端分页用）
    }, nil
}
```

**数据库查询逻辑（以FindLatestCompletingPumpLimit为例）：**
```sql
SELECT * FROM pair
WHERE name = 'PumpFun'
  AND pump_point > 0
  AND pump_point < 1
ORDER BY fdv DESC
LIMIT 10
```

**PumpFun状态判断：**
- **新创建**（PumpStatusNewCreation=0）：按创建时间倒序
- **进行中**（PumpStatusCompleting=1）：`0 < pump_point < 1`，按市值倒序
- **已完成**（PumpStatusCompleted=2）：`pump_point >= 1`，按市值倒序

**Bonding Curve进度计算：**  
`pump_point = 当前SOL流动性 / 目标SOL流动性`（PumpFun完成条件：85 SOL）

---

## 四、定时任务详解

### 4.1 PumpTicker - Pump Token列表缓存更新

**功能说明：**  
定时更新Pump Token列表缓存到Redis，减轻数据库查询压力。

**启动流程：**
```go
func (t *PumpTicker) Start() {
    // 1. 启动Completing/Completed状态更新任务（每1分钟）
    threading.GoSafe(func() {
        t.logger.Info("tradeTicker:udpatePumpCache")
        go t.StartTicker()
    })
    
    // 2. 启动NewCreation状态更新任务（每3秒）
    threading.GoSafe(func() {
        t.logger.Info("tradeTicker:udpateNewCreationCache")
        go t.StartNewCreationTicker()
    })
}
```

**为什么分两个定时器？**
- **NewCreation（新创建）**：更新频率高（3秒），因为PumpFun上Token创建速度快，需要及时展示
- **Completing/Completed（进行中/已完成）**：更新频率低（1分钟），因为这些Token状态变化较慢

#### 4.1.1 Completing/Completed更新任务
```go
func (l *PumpTicker) StartTicker() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    // 获取分布式锁（防止多实例重复执行）
    _, err := xredis.MustLock(context.Background(), l.sc.RDS, "lock:PumpTicker", 5, 5)
    if err != nil {
        logx.Errorf("xredis.MustLock pumpTicker acquiring lock fail:%v", err)
        return
    }
    
    for {
        select {
        case <-ticker.C:
            threading.RunSafe(func() {
                l.UpdateCache()
            })
        }
    }
}

func (l *PumpTicker) UpdateCache() {
    pairModel := solmodel.NewPairModel(l.sc.DB)
    redisClient := l.sc.RDS
    tokenModel := solmodel.NewTokenModel(l.sc.DB)
    
    // 定义通用的缓存更新函数
    updateCacheForStatus := func(status int, findFunc func(ctx context.Context, pageNo, pageSize int32) ([]solmodel.Pair, error)) {
        // 1. 查询Top 10交易对
        pairList, err := findFunc(l.ctx, 1, 10)
        if err != nil {
            logx.Errorf("Failed to get latest pump limit for status %d: %v", status, err)
            return
        }
        
        // 2. 批量查询Token信息（社交媒体）
        tokenAddresses := make([]string, 0)
        for _, pair := range pairList {
            if pair.TokenAddress != "" {
                tokenAddresses = append(tokenAddresses, pair.TokenAddress)
            }
        }
        
        chainId := constants.Sol
        tokenList, err := tokenModel.FindAllByAddresses(l.ctx, int64(chainId), tokenAddresses)
        if err != nil {
            return
        }
        
        tokenMap := make(map[string]*solmodel.Token)
        for _, token := range tokenList {
            tokenMap[token.Address] = &token
        }
        
        // 3. 组装列表数据
        list := make([]*market.PumpTokenItem, 0)
        for _, pair := range pairList {
            token := tokenMap[pair.TokenAddress]
            var tokenIcon, twitterUsername, telegram string
            if token != nil {
                tokenIcon = token.Icon
                twitterUsername = token.TwitterUsername
                telegram = token.Telegram
            }
            
            list = append(list, &market.PumpTokenItem{
                ChainId:          pair.ChainId,
                ChainIcon:        chain.ChainId2ChainIcon(100000),
                TokenAddress:     pair.TokenAddress,
                TokenIcon:        tokenIcon,
                TokenName:        pair.TokenSymbol,
                LaunchTime:       pair.BlockTime.Unix(),
                MktCap:           pair.Fdv,
                DomesticProgress: pair.PumpPoint,
                TwitterUsername:  twitterUsername,
                Telegram:         telegram,
                PairAddress:      pair.Address,
            })
        }
        
        // 4. 写入Redis（60秒过期）
        listData, err := json.Marshal(list)
        if err != nil {
            logx.Errorf("Failed to marshal list: %v", err)
            return
        }
        
        pairCacheKey := fmt.Sprint("pump-token-list-", status)
        err = redisClient.Set(pairCacheKey, string(listData))
        if err != nil {
            logx.Errorf("Failed to set list in Redis: %v", err)
            return
        }
        
        err = redisClient.Expire(pairCacheKey, 60)  // 60秒
        if err != nil {
            logx.Errorf("Failed to set expiration for list in Redis: %v", err)
            return
        }
    }
    
    // 更新Completing和Completed两个状态的缓存
    updateCacheForStatus(constants.PumpStatusCompleting, pairModel.FindLatestCompletingPumpLimit)
    updateCacheForStatus(constants.PumpStatusCompleted, pairModel.FindLatestCompletePumpLimit)
}
```

#### 4.1.2 NewCreation更新任务
```go
func (l *PumpTicker) StartNewCreationTicker() {
    ticker := time.NewTicker(3 * time.Second)
    defer ticker.Stop()
    
    // 获取分布式锁（防止多实例重复执行）
    _, err := xredis.MustLock(context.Background(), l.sc.RDS, "lock:PumpTickerNewCreation", 5, 1)
    if err != nil {
        logx.Errorf("xredis.MustLock pumpTicker acquiring lock fail:%v", err)
        return
    }
    
    for {
        select {
        case <-ticker.C:
            l.UpdateNewCreationCache()
        }
    }
}

func (l *PumpTicker) UpdateNewCreationCache() {
    // 逻辑与UpdateCache相同，只是查询NewCreation状态的数据
    // ...
    
    // 缓存过期时间：7天（NewCreation数据不常变化）
    err = redisClient.Expire(pairCacheKey, 60*60*24*7)
}
```

**定时任务设计思路：**
1. **缓存预热**：定时将热门数据加载到Redis，减少实时查询压力
2. **分布式锁**：防止多个Market服务实例同时执行更新任务
3. **短TTL**：Completing/Completed缓存60秒过期，保证数据新鲜度
4. **长TTL**：NewCreation缓存7天过期，因为历史数据不变

---

## 五、核心技术要点

### 5.1 K线数据的多级缓存架构（了解即可）

```
查询流程：
┌────────────┐
│  gRPC请求  │
└─────┬──────┘
      ↓
┌─────────────────┐
│ SingleFlight    │ ← 防止缓存击穿
│ (相同查询去重)   │
└─────┬───────────┘
      ↓
┌─────────────────────────────────────────┐
│          Redis ZSet缓存                  │
│  Key: kline:{chainId}:{pair}:{interval} │
│  Score: candleTime                      │
│  Value: CSV压缩字符串                    │
│  TTL: 30天                              │
└─────┬───────────────────────────────────┘
      │ 未命中
      ↓
┌─────────────────────────────────────────┐
│          内存缓存(KlineCache)            │
│  最新K线数据（Dataflow实时更新）          │
└─────┬───────────────────────────────────┘
      │ 未命中
      ↓
┌─────────────────────────────────────────┐
│          MySQL分月表                     │
│  trade_kline_{interval}_{month}         │
│  支持跨月查询自动合并                     │
└─────────────────────────────────────────┘
```

**缓存层级说明：**
1. **Redis缓存**：存储最近1500根K线，按时间范围查询高效
2. **内存缓存**：存储实时更新的最新K线数据（由Dataflow服务推送）
3. **MySQL数据库**：存储全部历史K线数据，按月分表

**缓存更新流程：**
```
Dataflow服务（生成K线）
    ↓
写入MySQL（持久化）
    ↓
通过Channel推送到KlineCache（内存缓存）
    ↓
Market服务查询时合并内存缓存+MySQL数据
    ↓
写入Redis（ZSet结构）
```

### 5.2 SingleFlight防止缓存击穿

**问题场景：**  
当Redis缓存失效时，如果有1000个用户同时查询相同的K线数据，会导致1000次MySQL查询，数据库瞬间被打垮。

**SingleFlight解决方案：**
```go
var klineSingleLight = new(singleflight.Group)

func (l *GetKlineLogic) doChanGetKlineData(ctx context.Context, g *singleflight.Group, in *marketclient.GetKlineRequest) (*marketclient.Klines, error) {
    // 使用唯一Key标识相同的查询
    key := fmt.Sprintf("%s_%s_%d_%d", in.PairAddress, in.Interval, in.ToTimestamp, in.Limit)
    
    // 相同Key的查询只执行一次，其他请求等待并共享结果
    ch := g.DoChan(key, func() (interface{}, error) {
        return l.getKline(in)
    })
    
    select {
    case <-ctx.Done():
        return &marketclient.Klines{}, ctx.Err()
    case ret := <-ch:
        return ret.Val.(*marketclient.Klines), ret.Err
    }
}
```

**工作原理：**
1. 第一个请求到达：执行`l.getKline(in)`查询数据库
2. 后续1000个相同请求：阻塞在`ch`通道上等待
3. 第一个请求查询完成：通过`ch`广播结果给所有等待的请求
4. 所有请求同时收到结果并返回

**效果对比：**
- **未使用SingleFlight**：1000次MySQL查询，数据库压力巨大
- **使用SingleFlight**：1次MySQL查询，999次等待共享结果

### 5.3 K线数据压缩存储

**为什么要压缩？**
- Redis内存宝贵，存储1500根K线 × 1000个交易对 = 150万条数据
- 原始Protobuf格式：约200字节/根K线
- CSV压缩格式：约100字节/根K线
- **节省50%内存**

**压缩格式设计：**
```
字段顺序（15个字段）：
pairAddr,candleTime,open,close,high,low,mcapOpen,mcapHigh,mcapLow,mcapClose,amountUsd,volumeToken,buyCount,sellCount,totalCount

示例：
EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v,1704067200,0.9998,1.0002,1.0005,0.9995,0,0,0,0,125000.5,125500,45,38,83
```

**压缩/解压性能：**
- 压缩：`strings.Join()`，约1微秒/次
- 解压：`strings.Split()`，约1微秒/次
- **性能损耗可忽略不计**

### 5.4 MySQL分月表设计

**为什么分月表？**
1. **单表数据量可控**：1分钟K线，1个月最多 `30天 × 1440分钟 × 1000对 = 4320万条`
2. **查询性能更好**：索引更小，查询速度更快
3. **便于数据归档**：历史数据可按月归档到冷存储

**表命名规则：**
```
trade_kline_1m_01   // 1分钟K线，1月份
trade_kline_1m_02   // 1分钟K线，2月份
trade_kline_5m_01   // 5分钟K线，1月份
trade_kline_1h_01   // 1小时K线，1月份
```

**跨月查询处理：**
```go
if repo.TableName(interval, fromTime) != repo.TableName(interval, toTime) {
    // 查询起始月份数据
    var resultFirst []Kline
    db.Table(repo.TableName(interval, fromTime)).Where(...).Scan(&resultFirst)
    
    // 查询结束月份数据
    var resultLast []Kline
    db.Table(repo.TableName(interval, toTime)).Where(...).Scan(&resultLast)
    
    // 合并结果
    result = append(resultLast, resultFirst...)
}
```

**索引设计：**
```sql
CREATE INDEX idx_chain_pair_time ON trade_kline_1m_01 (chain_id, pair_address, candle_time);
```

### 5.5 Redis Pub/Sub实时推送（重点）

**架构流程：**
```
Consumer服务（解析PumpFun创建Token交易）
    ↓
调用 Market.PushTokenInfo() gRPC
    ↓
Market服务从数据库增强Token信息
    ↓
Redis.Publish("pump_token_new", json_data)
    ↓
WebSocket服务订阅pump_token_new频道
    ↓
实时推送给前端用户（SSE/WebSocket）
```

**为什么选择Redis Pub/Sub？**
1. **解耦**：Consumer → Market → WebSocket三个服务解耦
2. **广播**：一条消息可以推送给多个WebSocket服务实例
3. **轻量**：无需引入Kafka等重量级消息队列

**缺点：**
- Redis Pub/Sub不保证消息可靠性（订阅者不在线会丢失消息）
- 但新Token推送是实时性要求高、可靠性要求低的场景，丢失几条消息不影响业务

---

## 六、配置文件详解

### 6.1 market.yaml
```yaml
Name: market.rpc
ListenOn: 0.0.0.0:8080

# MySQL配置（主库）
Mysql:
  Master:
    Username: root
    Password: web3ite.fun
    Path: sh-cdb-d1aqnkzo.sql.tencentcdb.com
    Port: 26218
    Dbname: fun_dexs
    MaxOpenConns: 500     # 最大连接数
    MaxIdleConns: 200     # 最大空闲连接数

# Redis配置
Redis:
  Host: r-uf6ka4mjfgkkgbqlhepd.redis.rds.aliyuncs.com:6379
  Pass: Web3itefun
  Type: node
  Key: bizRedis
  PingTimeout: 10s
  Tls: false
```

**配置说明：**
- **ListenOn**：gRPC服务监听端口
- **MaxOpenConns**：500，适配高并发查询场景
- **MaxIdleConns**：200，保持足够的空闲连接减少连接建立开销
- **Redis Type**：node（单节点模式），也支持cluster（集群模式）

---

## 七、数据库表结构

### 7.1 pair表（交易对信息）
```sql
CREATE TABLE `pair` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `chain_id` bigint NOT NULL COMMENT '链ID',
  `address` varchar(255) NOT NULL COMMENT '交易对地址',
  `name` varchar(50) DEFAULT NULL COMMENT 'DEX名称（PumpFun/Raydium V4/CPMM）',
  `factory_address` varchar(255) DEFAULT NULL COMMENT '工厂合约地址',
  `base_token_address` varchar(255) NOT NULL COMMENT 'Base Token地址（SOL/WSOL）',
  `token_address` varchar(255) NOT NULL COMMENT 'Token地址',
  `base_token_symbol` varchar(50) DEFAULT NULL COMMENT 'Base Token符号',
  `token_symbol` varchar(50) DEFAULT NULL COMMENT 'Token符号',
  `base_token_decimal` int DEFAULT NULL COMMENT 'Base Token精度',
  `token_decimal` int DEFAULT NULL COMMENT 'Token精度',
  `base_token_is_native_token` tinyint DEFAULT '0' COMMENT 'Base Token是否为原生币',
  `base_token_is_token0` tinyint DEFAULT '0' COMMENT 'Base Token是否为token0',
  `init_base_token_amount` double DEFAULT NULL COMMENT '初始Base Token流动性',
  `init_token_amount` double DEFAULT NULL COMMENT '初始Token流动性',
  `current_base_token_amount` double DEFAULT NULL COMMENT '当前Base Token流动性',
  `current_token_amount` double DEFAULT NULL COMMENT '当前Token流动性',
  `fdv` double DEFAULT NULL COMMENT '完全稀释市值',
  `mkt_cap` double DEFAULT NULL COMMENT '市值',
  `token_price` double DEFAULT NULL COMMENT 'Token价格（USD）',
  `base_token_price` double DEFAULT NULL COMMENT 'Base Token价格（USD）',
  `block_num` bigint DEFAULT NULL COMMENT '创建区块高度',
  `block_time` datetime DEFAULT NULL COMMENT '创建时间',
  `highest_token_price` double DEFAULT NULL COMMENT '历史最高价',
  `latest_trade_time` datetime DEFAULT NULL COMMENT '最后交易时间',
  `pump_point` double DEFAULT NULL COMMENT 'PumpFun Bonding Curve进度（0-1）',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_address` (`chain_id`, `address`),
  KEY `idx_chain_token` (`chain_id`, `token_address`),
  KEY `idx_pump_status` (`name`, `pump_point`)  -- 查询Pump Token列表
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 7.2 token表（Token基础信息）
```sql
CREATE TABLE `token` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `chain_id` bigint NOT NULL COMMENT '链ID',
  `address` varchar(255) NOT NULL COMMENT 'Token地址',
  `name` varchar(255) DEFAULT NULL COMMENT 'Token名称',
  `symbol` varchar(50) DEFAULT NULL COMMENT 'Token符号',
  `decimals` int DEFAULT NULL COMMENT 'Token精度',
  `total_supply` double DEFAULT NULL COMMENT '总供应量',
  `icon` varchar(500) DEFAULT NULL COMMENT 'Token图标URL',
  `twitter_username` varchar(255) DEFAULT NULL COMMENT 'Twitter用户名',
  `telegram` varchar(255) DEFAULT NULL COMMENT 'Telegram链接',
  `website` varchar(500) DEFAULT NULL COMMENT '官网',
  `program` varchar(255) DEFAULT NULL COMMENT 'Token Program（Token/Token2022）',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_address` (`chain_id`, `address`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 7.3 trade_kline_XX_XX表（K线数据）
```sql
CREATE TABLE `trade_kline_1m_01` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `chain_id` bigint NOT NULL COMMENT '链ID',
  `pair_address` varchar(255) NOT NULL COMMENT '交易对地址',
  `candle_time` bigint NOT NULL COMMENT 'K线时间戳（秒）',
  `open_at` bigint DEFAULT NULL COMMENT '第一笔交易时间戳',
  `close_at` bigint DEFAULT NULL COMMENT '最后一笔交易时间戳',
  `o` double DEFAULT NULL COMMENT '开盘价',
  `c` double DEFAULT NULL COMMENT '收盘价',
  `h` double DEFAULT NULL COMMENT '最高价',
  `l` double DEFAULT NULL COMMENT '最低价',
  `v` double DEFAULT NULL COMMENT '成交额（USD）',
  `t` double DEFAULT NULL COMMENT '成交量（Token）',
  `a` double DEFAULT NULL COMMENT '平均价',
  `count` bigint DEFAULT NULL COMMENT '交易笔数',
  `buy_count` bigint DEFAULT NULL COMMENT '买入笔数',
  `sell_count` bigint DEFAULT NULL COMMENT '卖出笔数',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_pair_time` (`chain_id`, `pair_address`, `candle_time`),
  KEY `idx_chain_pair_time` (`chain_id`, `pair_address`, `candle_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 7.4 sol_token_account表（持仓账户）
```sql
CREATE TABLE `sol_token_account` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `chain_id` bigint NOT NULL COMMENT '链ID',
  `owner` varchar(255) NOT NULL COMMENT '账户Owner',
  `mint` varchar(255) NOT NULL COMMENT 'Token地址',
  `address` varchar(255) NOT NULL COMMENT 'ATA地址',
  `amount` decimal(65,0) DEFAULT NULL COMMENT '持仓数量（最小单位）',
  `decimals` int DEFAULT NULL COMMENT '精度',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_address` (`address`),
  KEY `idx_mint_created` (`mint`, `created_at`)  -- 统计持仓人数
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**索引说明：**
- `idx_mint_created`：用于统计某Token创建后的持仓人数
- 查询SQL：`SELECT COUNT(*) FROM sol_token_account WHERE mint = ? AND created_at > ?`

---

## 八、与其他服务的交互

### 8.1 调用方

**Trade服务：**
- `GetPairInfoByToken`：创建订单时获取交易对信息
- `GetNativeTokenPrice`：获取SOL实时价格

**Gateway服务（API网关）：**
- `GetKline`：前端K线图数据
- `GetPumpTokenList`：前端Token列表
- `GetTokenInfo`：Token详情页

### 8.2 被调用方

**Consumer服务：**
- 调用`PushTokenInfo`推送新Token信息

**Dataflow服务：**
- 更新K线数据到MySQL
- 推送最新K线到内存缓存（KlineCache）

---

## 九、核心优化思路总结

### 9.1 性能优化
1. **多级缓存**：Redis + 内存缓存 + MySQL，减少数据库查询
2. **SingleFlight**：防止缓存击穿，避免雪崩
3. **数据压缩**：CSV格式存储K线，节省50%内存
4. **分月表**：控制单表数据量，提升查询性能

### 9.2 可用性优化
1. **定时任务预热缓存**：热门数据提前加载到Redis
2. **分布式锁**：防止多实例重复执行定时任务
3. **数据补全**：K线缺失自动补全，保证前端图表连续性

### 9.3 扩展性设计
1. **gRPC接口**：支持跨语言调用（Go/Python/Java）
2. **Redis Pub/Sub**：解耦服务，支持水平扩展
3. **分表设计**：支持按月归档历史数据

---


