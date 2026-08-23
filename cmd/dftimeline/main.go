// Command dftimeline 是数字取证时间线冲突解析服务入口。
//
// 用法：
//
//	go run ./cmd/dftimeline --addr :8090 --db dftimeline.db
//	go run ./cmd/dftimeline --smoke-test
//
// --smoke-test 不启动长驻服务，而是真实创建数据、执行核心闭环、
// 关闭并重开数据库验证持久化恢复后以 0 退出；这是 Docker 双架构验证的判据。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task182-dftimeline/internal/httpapi"
	"task182-dftimeline/internal/service"
	"task182-dftimeline/internal/store"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	dbPath := flag.String("db", "dftimeline.db", "sqlite database path")
	smoke := flag.Bool("smoke-test", false, "run self-check and exit without serving")
	flag.Parse()

	if *smoke {
		// 自检使用独立临时数据库，避免污染在线数据且可重复运行。
		out, err := service.RunSelfCheck("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "SELFCHECK FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(out)
		os.Exit(0)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	app := service.New(st)
	if err := app.Recover(); err != nil {
		log.Fatalf("recover: %v", err)
	}
	log.Printf("recovered state from %s", *dbPath)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(app).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("dftimeline listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
