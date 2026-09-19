package middleware

import (
	"bufio"
	"bytes"
	"dex/internal/respx"
	"dex/pkg/xcode"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/core/logx"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

var _ http.Hijacker = (*responseWriter)(nil)

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	// rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	return rw.body.Write(p)
}

func (rw *responseWriter) Body() []byte {
	return rw.body.Bytes()
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("not support")
	}
	return hijacker.Hijack()
}

func WrapResponse(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logCtx := logx.ContextWithFields(r.Context(), logx.Field("path", r.URL.Path))

		// Log incoming request details
		logc.Infof(logCtx, "[GATEWAY-REQ] %s %s", r.Method, r.URL.Path)
		for k, v := range r.Header {
			logc.Infof(logCtx, "[GATEWAY-REQ-HEADER] %s: %v", k, v)
		}
		if r.Body != nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(r.Body)
			bodyStr := buf.String()
			logc.Infof(logCtx, "[GATEWAY-REQ-BODY] %s", bodyStr)
			// Restore the body for downstream handlers
			r.Body = io.NopCloser(bytes.NewBufferString(bodyStr))
		}

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)
		if rw.statusCode == http.StatusGatewayTimeout {
			respx.JsonResp(w, r, http.StatusGatewayTimeout, rw.body.String(), nil)
			return
		}
		if rw.statusCode != http.StatusOK {
			code, msg := unwrapHttpStatusCode(rw.body.String())
			if code == xcode.InvalidSignatureError.Code() {
				http.Error(w, "", http.StatusUnauthorized)
				return
			}
			if code == 0 {
				code = xcode.InternalError.Code()
			}
			respx.JsonResp(w, r, code, msg, nil)
			return
		}

		// Log outgoing response details
		logc.Infof(logCtx, "[GATEWAY-RESP-STATUS] %d", rw.statusCode)
		for k, v := range w.Header() {
			logc.Infof(logCtx, "[GATEWAY-RESP-HEADER] %s: %v", k, v)
		}
		logc.Infof(logCtx, "[GATEWAY-RESP-BODY] %s", rw.body.String())

		// Wrap succeeded respx
		logc.Infof(logCtx, "Response body: %s", rw.body.String())

		// Special handling for add_liquidity_v1 endpoint
		if strings.Contains(r.URL.Path, "add_liquidity_v1") {
			logc.Infof(logCtx, "[DEBUG] Processing add_liquidity_v1 endpoint")
			logc.Infof(logCtx, "[DEBUG] Raw response body: %s", rw.body.String())

			// This is a special case for AddLiquidityV1 which returns a txHash
			// Extract the txHash directly from the response body
			bodyStr := rw.body.String()

			// Check if it contains a txHash field
			if strings.Contains(bodyStr, "txHash") {
				logc.Infof(logCtx, "[DEBUG] Found txHash in response body")

				// Try to parse as JSON first
				var addLiqResp struct {
					TxHash string `json:"txHash"`
				}

				err := json.Unmarshal(rw.Body(), &addLiqResp)
				if err != nil {
					logc.Errorf(logCtx, "[DEBUG] Failed to parse response as JSON: %v", err)
				} else {
					logc.Infof(logCtx, "[DEBUG] JSON parsed successfully: %+v", addLiqResp)
				}

				if err == nil && addLiqResp.TxHash != "" {
					// Successfully parsed as JSON
					logc.Infof(logCtx, "[DEBUG] Successfully extracted txHash from JSON: %s", addLiqResp.TxHash)
					respData := map[string]interface{}{
						"txHash": addLiqResp.TxHash,
					}
					respx.JsonResp(w, r, xcode.Ok, "", respData)
					return
				}

				// If JSON parsing failed, try to extract the txHash directly
				logc.Infof(logCtx, "[DEBUG] Attempting direct extraction of txHash")
				start := strings.Index(bodyStr, "txHash")
				logc.Infof(logCtx, "[DEBUG] txHash position: %d", start)

				if start > 0 {
					// Find the value after txHash
					valueStart := strings.Index(bodyStr[start:], ":")
					logc.Infof(logCtx, "[DEBUG] Value start position (relative): %d", valueStart)

					if valueStart > 0 {
						valueStart = start + valueStart + 1
						logc.Infof(logCtx, "[DEBUG] Value start position (absolute): %d", valueStart)

						// Find the end of the value (either comma, closing brace, or quote)
						valueEnd := -1
						for i := valueStart; i < len(bodyStr); i++ {
							if bodyStr[i] == ',' || bodyStr[i] == '}' || bodyStr[i] == '"' {
								valueEnd = i
								break
							}
						}
						logc.Infof(logCtx, "[DEBUG] Value end position: %d", valueEnd)

						if valueEnd > valueStart {
							txHash := strings.TrimSpace(bodyStr[valueStart:valueEnd])
							// Remove any quotes
							txHash = strings.Trim(txHash, "\"'")
							logc.Infof(logCtx, "[DEBUG] Extracted txHash: '%s'", txHash)

							if txHash != "" {
								logc.Infof(logCtx, "[DEBUG] Successfully extracted txHash directly: %s", txHash)
								respData := map[string]interface{}{
									"txHash": txHash,
								}
								respx.JsonResp(w, r, xcode.Ok, "", respData)
								return
							} else {
								logc.Errorf(logCtx, "[DEBUG] Extracted txHash is empty")
							}
						} else {
							logc.Errorf(logCtx, "[DEBUG] Invalid value positions: start=%d, end=%d", valueStart, valueEnd)
						}
					} else {
						logc.Errorf(logCtx, "[DEBUG] Could not find ':' after txHash")
					}
				} else {
					logc.Errorf(logCtx, "[DEBUG] Could not find txHash in response body")
				}
			} else {
				logc.Errorf(logCtx, "[DEBUG] Response does not contain txHash field")
			}
		}

		// Regular JSON response handling
		var resp map[string]interface{}
		err := json.Unmarshal(rw.Body(), &resp)
		if err != nil {
			logc.Errorf(logCtx, "[DEBUG] Regular JSON unmarshal failed: %s\n body:%s", err.Error(), rw.body.String())
			http.Error(w, err.Error(), http.StatusOK)
			return
		}

		logc.Infof(logCtx, "[DEBUG] Regular response processing successful: %+v", resp)
		respx.JsonResp(w, r, xcode.Ok, "", resp)
	}
}

func unwrapHttpStatusCode(s string) (int, string) {
	connectIndex := strings.Index(s, "connect")
	if connectIndex >= 0 {
		return xcode.InternalError.Code(), ""
	}

	descIndex := strings.Index(s, "desc = ")
	if descIndex == -1 {
		return 0, s
	}
	desc := strings.TrimSpace(s[descIndex+6:])
	segs := strings.Split(desc, " ")
	code, _ := strconv.Atoi(segs[0])
	message := strings.Join(segs[1:], " ")
	if code == 0 {
		message = desc
	}

	return code, message
}
