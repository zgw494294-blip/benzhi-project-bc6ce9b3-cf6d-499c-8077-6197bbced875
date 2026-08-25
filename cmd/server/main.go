package main

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/httpapi"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		slog.Error("服务退出", "error", e)
		os.Exit(1)
	}
}
func run() error {
	addrFlag := flag.String("addr", defaultAddr, "回环监听地址")
	dbFlag := flag.String("db", "frequency-review.db", "SQLite 数据库路径")
	selfcheck := flag.Bool("selfcheck", false, "运行有界 HTTP 业务自检后退出")
	flag.Parse()
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			set = true
		}
	})
	addr, e := address(*addrFlag, set)
	if e != nil {
		return e
	}
	dsn := *dbFlag
	if *selfcheck {
		dsn = "file:selfcheck?mode=memory&cache=shared"
	}
	ctx := context.Background()
	store, e := storage.Open(ctx, dsn)
	if e != nil {
		return e
	}
	defer store.Close()
	service := application.New(store)
	api := httpapi.New(service)
	listener, e := net.Listen("tcp", addr)
	if e != nil {
		return fmt.Errorf("监听 %s: %w", addr, e)
	}
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	serveErrors := make(chan error, 1)
	go func() {
		e := server.Serve(listener)
		if e != nil && e != http.ErrServerClosed {
			serveErrors <- e
		} else {
			serveErrors <- nil
		}
	}()
	if *selfcheck {
		e = runSelfcheck(addr)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		serveErr := <-serveErrors
		if e != nil {
			return e
		}
		if serveErr != nil {
			return serveErr
		}
		fmt.Println("selfcheck: 频率变更审查与许可签发链路通过")
		return nil
	}
	slog.Info("频率变更启用审查台已启动", "addr", addr)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
	case e = <-serveErrors:
		return e
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
