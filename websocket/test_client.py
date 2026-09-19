#!/usr/bin/env python3
import asyncio
import websockets
import json
import logging

# 设置日志级别
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

async def test_websocket():
    uri = "ws://118.194.235.63:8086/ws/tokens?chain_id=100000"
    logger.info(f"🔌 Connecting to: {uri}")
    
    try:
        async with websockets.connect(uri) as websocket:
            logger.info("✅ Connected successfully!")
            
            # 发送订阅消息
            subscribe_msg = {
                "type": "subscribe",
                "data": {
                    "chain_id": 100000,
                    "categories": ["new_creation", "completing", "completed"]
                }
            }
            
            await websocket.send(json.dumps(subscribe_msg))
            logger.info(f"📨 Sent subscription: {subscribe_msg}")
            
            # 监听消息
            async for message in websocket:
                try:
                    data = json.loads(message)
                    logger.info(f"📨 Received message: {data.get('type', 'unknown')}")
                    if data.get('type') == 'new_token':
                        token_data = data.get('data', {})
                        logger.info(f"🆕 New token: {token_data.get('token_name', 'Unknown')}")
                    elif data.get('type') == 'connection_established':
                        logger.info(f"🎯 Connection established: {data.get('data', {})}")
                    else:
                        logger.info(f"📄 Full message: {data}")
                except json.JSONDecodeError as e:
                    logger.error(f"❌ JSON decode error: {e}")
                    logger.error(f"📄 Raw message: {message}")
                    
    except websockets.exceptions.ConnectionClosed as e:
        logger.error(f"❌ Connection closed: {e}")
    except Exception as e:
        logger.error(f"❌ Connection failed: {e}")

if __name__ == "__main__":
    logger.info("🚀 Starting WebSocket test client...")
    asyncio.run(test_websocket()) 