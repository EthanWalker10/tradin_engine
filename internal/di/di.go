package di

import (
	"context"

	app "github.com/yzimhao/trading_engine/v2/app"
	"github.com/yzimhao/trading_engine/v2/app/example"
	"github.com/yzimhao/trading_engine/v2/internal/modules"
	"github.com/yzimhao/trading_engine/v2/internal/persistence/gorm"
	"github.com/yzimhao/trading_engine/v2/internal/services"
	"go.uber.org/fx"  // 依赖注入
	"go.uber.org/zap" // 日志记录
)

// 管理服务器的生命周期（启动、停止等）。
type Server interface {
	Start() error
	Stop() error
	Scheme() string
	Addr() string
}

// 用于设置服务器的生命周期钩子，并在应用启动时启动 HTTP 服务器。
// 本身并不被 app.Run() 直接调用，而是通过 fx.Invoke 注册到 fx.App 中，app.Run() 启动时会触发该回调函数。
func RunServer(lc fx.Lifecycle, server Server, logger *zap.Logger) {
	lc.Append(fx.Hook{ // 注册了两个生命周期钩子
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting server", zap.String("scheme", server.Scheme()), zap.String("addr", server.Addr()))
			go server.Start() // 协程异步执行
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Stopping server")
			return server.Stop()
		},
	})
}

func App() *fx.App {
	return fx.New(
		// 注册所有需要注入的依赖（包括各种服务、模块等的实例）
		fx.Provide(
			zap.NewDevelopment,
			NewViper,
			NewRedis,
			NewGinEngine,
			NewHttpServer,
			NewCache,
			NewGorm,
			NewBroker,
		),

		// 也是由 fx.Option 包裹的 fx.provide
		gorm.Module,
		app.Module,
		example.Module,
		services.Module,
		modules.Load,
		fx.Invoke(RunServer),
	)
}
