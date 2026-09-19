const webpack = require('webpack');

module.exports = {
  webpack: {
    configure: (webpackConfig) => {
      // Add fallbacks for Node.js modules
      webpackConfig.resolve.fallback = {
        ...webpackConfig.resolve.fallback,
        "http": require.resolve("stream-http"),
        "https": require.resolve("https-browserify"),
        "url": require.resolve("url/"),
        "buffer": require.resolve("buffer"),
        "crypto": require.resolve("crypto-browserify"),
        "stream": require.resolve("stream-browserify"),
        "assert": require.resolve("assert/"),
        "os": require.resolve("os-browserify/browser"),
        "path": require.resolve("path-browserify"),
        "zlib": require.resolve("browserify-zlib"),
        "util": require.resolve("util/")
      };

      // Add plugins
      webpackConfig.plugins = [
        ...webpackConfig.plugins,
        new webpack.ProvidePlugin({
          process: 'process/browser',
          Buffer: ['buffer', 'Buffer'],
        }),
      ];

      return webpackConfig;
    },
  },
  devServer: {
    allowedHosts: 'all',
    host: '0.0.0.0',
    port: 3001,
    proxy: {
      '/v1': {
        target: 'http://localhost:8083',
        changeOrigin: true,
        secure: false,
      },
      '/direct-api': {
        target: 'http://localhost:8083',
        changeOrigin: true,
        secure: false,
      },
    },
  },
}; 