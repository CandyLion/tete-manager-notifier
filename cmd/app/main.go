package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CandyLion/tmbark-notifier/internal/config"
	"github.com/CandyLion/tmbark-notifier/internal/db"
	"github.com/CandyLion/tmbark-notifier/internal/mqtt"
	appweb "github.com/CandyLion/tmbark-notifier/internal/web"
)

func main() {
	cfg := config.Load()

	// 初始化日志
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("🚀 特特管家通知器启动 | CarID: %d", cfg.CarID)

	// 初始化数据库
	if err := db.Init(cfg); err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}
	log.Println("✅ 数据库连接成功")

	webServer := appweb.NewServer(cfg)
	if cfg.WebPort > 0 {
		go func() {
			if err := webServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("❌ 详情页服务启动失败: %v", err)
			}
		}()
		log.Printf("✅ 行程详情页服务已启动: %s", webServer.Addr())
	}
	if cfg.BarkKey != "" && cfg.PublicBaseURL == "" {
		log.Println("⚠️ 未配置 PUBLIC_BASE_URL，Bark 行程通知将不会附带轨迹图和详情页链接")
	}

	// 初始化 MQTT 客户端并启动订阅
	mqttClient := mqtt.NewClient(cfg)
	if err := mqttClient.Connect(); err != nil {
		log.Fatalf("❌ MQTT 连接失败: %v", err)
	}

	log.Println("✅ MQTT 订阅已启动，等待车辆状态变化...")

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("🛑 收到停止信号，正在优雅退出...")
	mqttClient.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := webServer.Shutdown(ctx); err != nil {
		log.Printf("⚠️ 详情页服务关闭异常: %v", err)
	}
	fmt.Println("👋 程序已退出")
}
