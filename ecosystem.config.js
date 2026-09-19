module.exports = {
  apps: [
    // Market Service
    {
      name: 'market',
      cwd: './market',
      script: '/usr/local/go/bin/go',
      args: 'run market.go',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '500M',
      env: {
        PATH: '/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
      },
      error_file: '../logs/market-error.log',
      out_file: '../logs/market-out.log',
      log_file: '../logs/market.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true
    },
    // Trade Service
    {
      name: 'trade',
      cwd: './trade',
      script: '/usr/local/go/bin/go',
      args: 'run trade.go',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '500M',
      env: {
        PATH: '/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
      },
      error_file: '../logs/trade-error.log',
      out_file: '../logs/trade-out.log',
      log_file: '../logs/trade.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true
    },
    // Consumer Service
    {
      name: 'consumer',
      cwd: './consumer',
      script: '/usr/local/go/bin/go',
      args: 'run consumer.go',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '500M',
      env: {
        PATH: '/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
      },
      error_file: '../logs/consumer-error.log',
      out_file: '../logs/consumer-out.log',
      log_file: '../logs/consumer.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true
    },
    // Gateway Service
    {
      name: 'gateway',
      cwd: './gateway',
      script: '/usr/local/go/bin/go',
      args: 'run gateway.go',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '500M',
      env: {
        PATH: '/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
      },
      error_file: '../logs/gateway-error.log',
      out_file: '../logs/gateway-out.log',
      log_file: '../logs/gateway.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true
    },
    // WebSocket Service
    {
      name: 'websocket',
      cwd: './websocket',
      script: '/usr/local/go/bin/go',
      args: 'run token_websocket_server.go',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '500M',
      env: {
        PATH: '/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
      },
      error_file: '../logs/websocket-error.log',
      out_file: '../logs/websocket-out.log',
      log_file: '../logs/websocket.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true
    },
    // Frontend Service (pump-tokens-ui)
    {
      name: 'pump-tokens-ui',
      cwd: './pump-tokens-ui',
      script: 'npx',
      args: 'serve -s build -l 3001',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '1G',
      env: {
        NODE_ENV: 'production',
        PORT: 3001
      },
      env_production: {
        NODE_ENV: 'production',
        PORT: 3001
      },
      error_file: '../logs/pump-tokens-ui-error.log',
      out_file: '../logs/pump-tokens-ui-out.log',
      log_file: '../logs/pump-tokens-ui.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true
    }
  ]
};
