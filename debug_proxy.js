const http = require('http');
const https = require('https');
const url = require('url');

const PORT = 9000;
const TARGET_HOST = 'localhost';
const TARGET_PORT = 8081;

const server = http.createServer((req, res) => {
  console.log(`Received request: ${req.method} ${req.url}`);
  
  // Parse the request URL
  const parsedUrl = url.parse(req.url);
  
  // Create options for the proxy request
  const options = {
    hostname: TARGET_HOST,
    port: TARGET_PORT,
    path: parsedUrl.path,
    method: req.method,
    headers: req.headers
  };
  
  console.log(`Proxying to: ${TARGET_HOST}:${TARGET_PORT}${parsedUrl.path}`);
  
  // Create a proxy request
  const proxyReq = http.request(options, (proxyRes) => {
    console.log(`Received response: ${proxyRes.statusCode}`);
    
    // Set the status code and headers from the proxied response
    res.writeHead(proxyRes.statusCode, proxyRes.headers);
    
    // Collect the response data
    let responseData = '';
    proxyRes.on('data', (chunk) => {
      responseData += chunk;
    });
    
    // When the response is complete, log it and send it back to the client
    proxyRes.on('end', () => {
      console.log('Response data:', responseData);
      res.end(responseData);
    });
  });
  
  // Handle errors in the proxy request
  proxyReq.on('error', (error) => {
    console.error('Proxy request error:', error);
    res.writeHead(500);
    res.end(`Proxy error: ${error.message}`);
  });
  
  // Collect the request data
  let requestData = '';
  req.on('data', (chunk) => {
    requestData += chunk;
  });
  
  // When the request is complete, log it and send it to the target
  req.on('end', () => {
    console.log('Request data:', requestData);
    proxyReq.end(requestData);
  });
});

server.listen(PORT, () => {
  console.log(`Debug proxy server listening on port ${PORT}`);
  console.log(`Proxying requests to ${TARGET_HOST}:${TARGET_PORT}`);
}); 