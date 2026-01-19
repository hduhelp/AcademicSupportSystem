package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"HelpStudent/config"
	"HelpStudent/core/healthz"
	"HelpStudent/core/kernel"
	"HelpStudent/core/logx"
	"HelpStudent/core/store/pg"
	"HelpStudent/core/stringx"
	"HelpStudent/core/tracex"
	"HelpStudent/internal/app/appInitialize"

	"github.com/flamego/cors"
	"github.com/flamego/flamego"
	"github.com/hduhelp/api_open_sdk/transfer"
	"github.com/soheilhy/cmux"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	configYml string
	engine    *kernel.Engine
	StartCmd  = &cobra.Command{
		Use:     "server",
		Short:   "Set Application config info",
		Example: "main server -c config/settings.yml",
		PreRun: func(cmd *cobra.Command, args []string) {
			setUp()
			loadStore()
			loadApp()
		},
		Run: func(cmd *cobra.Command, args []string) {
			run()
		},
	}
)

func init() {
	StartCmd.PersistentFlags().StringVarP(&configYml, "config", "c", "config/config.yaml", "Start server with provided configuration file")
}

// 初始化配置和日志
func setUp() {
	// 初始化全局 ctx
	ctx, cancel := context.WithCancel(context.Background())

	// 初始化资源管理器
	engine = &kernel.Engine{Ctx: ctx, Cancel: cancel}
	kernel.Kernel = engine

	// 加载配置
	config.LoadConfig(configYml, func(globalConfig *config.GlobalConfig) {
		for _, listener := range engine.ConfigListener {
			listener(globalConfig)
		}
	})

	if logx.SystemLogger == nil {
		logx.SystemLogger = logx.Setup()
	}
	if logx.ServiceLogger == nil {
		logx.ServiceLogger = logx.Setup()
	}
	if config.GetConfig().MODE == "debug" {
		logx.SystemLogger.SetLevel(zap.DebugLevel)
		logx.ServiceLogger.SetLevel(zap.DebugLevel)
	}

	// 初始化 flamego
	flamego.SetEnv(flamego.EnvType(config.GetConfig().MODE))
	engine.Fg = flamego.New()
	engine.Fg.Use(flamego.Recovery(), flamego.Renderer(), cors.CORS(cors.Options{
		AllowCredentials: true,
		Methods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
	}))

}

// 存储介质连接
func loadStore() {
	engine.MainPG = pg.MustNewPGOrm(config.GetConfig().MainPostgres)
}

// 加载应用，包含多个生命周期
func loadApp() {
	apps := appInitialize.GetApps()
	for _, app := range apps {
		_err := app.PreInit(engine)
		if _err != nil {
			logx.SystemLogger.Errorw("failed to pre init app", zap.Field{Key: "error", Type: zapcore.StringType, String: _err.Error()})
			os.Exit(1)
		}
	}
	for _, app := range apps {
		_err := app.Init(engine)
		if _err != nil {
			logx.SystemLogger.Errorw("failed to init app", zap.Field{Key: "error", Type: zapcore.StringType, String: _err.Error()})
			os.Exit(1)
		}
	}
	for _, app := range apps {
		_err := app.PostInit(engine)
		if _err != nil {
			logx.SystemLogger.Errorw("failed to post init app", zap.Field{Key: "error", Type: zapcore.StringType, String: _err.Error()})
			os.Exit(1)
		}
	}
	for _, app := range apps {
		_err := app.Load(engine)
		if _err != nil {
			logx.SystemLogger.Errorw("failed to load app", zap.Field{Key: "error", Type: zapcore.StringType, String: _err.Error()})
			os.Exit(1)
		}
	}
	for _, app := range apps {
		_err := app.Start(engine)
		if _err != nil {
			logx.SystemLogger.Errorw("failed to start app", zap.Field{Key: "error", Type: zapcore.StringType, String: _err.Error()})
			os.Exit(1)
		}
	}

	// 设置/grpc路由 将gw嵌入到flamego中，flamego 为入口网关，含 /grpc 前缀的请求转发到 grpc-gateway 处理
	engine.Fg.Any("/grpc/{**}", func(w http.ResponseWriter, r *http.Request) {
		r.RequestURI = strings.Replace(r.RequestURI, "/grpc", "", 1)
		r.URL.Path = strings.Replace(r.URL.Path, "/grpc", "", 1)
		engine.Mux.ServeHTTP(w, r)
	})

}

// 启动服务
func run() {
	transfer.Init(config.GetConfig().OAuth[0].HDUHelp.ClientID, config.GetConfig().OAuth[0].HDUHelp.ClientSecret)
	port := config.GetConfig().Port
	// 开启 tcp 监听
	conn, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		logx.SystemLogger.Errorw("failed to listen", zap.Field{Key: "error", Type: zapcore.StringType, String: err.Error()})
	}

	// 分流
	tcpMux := cmux.New(conn)
	httpL := tcpMux.Match(cmux.HTTP1Fast())
	go func() {
		// 在 flamego 外再包一层 otelhttp 用于链路追踪注入
		engine.HttpServer = &http.Server{
			Handler: otelhttp.NewHandler(engine.Fg, "gateway", otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			})),
		}
		if _err := engine.HttpServer.Serve(httpL); _err != nil && _err != http.ErrServerClosed {
			logx.SystemLogger.Errorw("failed to start to listen and serve http", zap.Field{Key: "error", Type: zapcore.StringType, String: _err.Error()})
		}
	}()

	go func() {
		logx.SystemLogger.Info("mux listen starting...")
		if _err := tcpMux.Serve(); _err != nil {
			logx.SystemLogger.Errorw("failed to serve mux", zap.Field{Key: "error", Type: zapcore.StringType, String: _err.Error()})
		}
	}()

	println(stringx.Green("Server run at:"))
	println(fmt.Sprintf("-  Local:   http://localhost:%s", port))
	// 健康检查设置为可接受服务
	healthz.Health.Set(true)

	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 健康检查设置为不可接受服务
	healthz.Health.Set(false)

	println(stringx.Blue("Shutting down server..."))
	tracex.StopAgent()

	if engine.SlsClient != nil {
		if err = engine.SlsClient.Close(); err != nil {
			println(stringx.Yellow("Sls client close failed: " + err.Error()))
		}
	}
	logx.SystemLogger.Stop()
	logx.ServiceLogger.Stop()

	ctx, cancel := context.WithTimeout(engine.Ctx, 5*time.Second)
	defer engine.Cancel()
	defer cancel()

	if err := engine.HttpServer.Shutdown(ctx); err != nil {
		println(stringx.Yellow("Server forced to shutdown: " + err.Error()))
	}

	println(stringx.Green("Server exiting Correctly"))
}
