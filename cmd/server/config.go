package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddr = "127.0.0.1:19081"

func address(flagValue string, flagWasSet bool) (string, error) {
	addr := flagValue
	if !flagWasSet {
		if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
			port, e := strconv.Atoi(raw)
			if e != nil || port < 1024 || port > 65535 {
				return "", fmt.Errorf("PORT 必须是 1024 到 65535 的端口号")
			}
			addr = net.JoinHostPort("127.0.0.1", raw)
		}
	}
	host, port, e := net.SplitHostPort(addr)
	if e != nil {
		return "", fmt.Errorf("监听地址无效: %w", e)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", errors.New("监听地址必须是回环地址")
	}
	n, e := strconv.Atoi(port)
	if e != nil || n < 1024 || n > 65535 {
		return "", errors.New("监听端口必须在 1024 到 65535 之间")
	}
	return addr, nil
}
