package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/api/middleware"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/generated"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaoshub"
	handler2 "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chaoshub/handler"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agenthub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/apphub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb"
	dbSchemaChaosHub "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/chaos_hub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/config"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/handlers"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/observability"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/projects"
	pb "github.com/litmuschaos/litmus/chaoscenter/graphql/server/protos"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetReportCaller(true)
	log.Printf("go version: %s", runtime.Version())
	log.Printf("go os/arch: %s/%s", runtime.GOOS, runtime.GOARCH)

	// Load .env from CWD if present (local dev convenience only).
	// In production, start-agentcert.sh sets all env vars before launching
	// this binary — no path probing needed or wanted.
	_ = godotenv.Load()

	err := envconfig.Process("", &utils.Config)
	if err != nil {
		log.Fatal(err)
	}
}

func validateVersion() error {
	currentVersion := utils.Config.Version
	dbVersion, err := config.GetConfig(context.Background(), "version")
	if err != nil {
		return fmt.Errorf("failed to get version from db, error = %w", err)
	}
	if dbVersion == nil {
		err := config.CreateConfig(
			context.Background(),
			&config.ServerConfig{Key: "version", Value: currentVersion},
		)
		if err != nil {
			return fmt.Errorf("failed to insert current version in db, error = %w", err)
		}
		return nil
	}
	// This check will be added back once DB upgrader job becomes functional
	// if dbVersion.Value.(string) != currentVersion {
	// 	return fmt.Errorf("control plane needs to be upgraded from version %v to %v", dbVersion.Value.(string), currentVersion)
	// }
	return nil
}

func setupGin() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(middleware.DefaultStructuredLogger())
	router.Use(gin.Recovery())
	router.Use(middleware.ValidateCors())
	return router
}

func main() {
	router := setupGin()
	var err error
	mongodb.MgoClient, err = mongodb.MongoConnection()
	if err != nil {
		log.Fatal(err)
	}

	mongoClient := mongodb.Client.Initialize(mongodb.MgoClient)

	var mongodbOperator mongodb.MongoOperator = mongodb.NewMongoOperations(mongoClient)
	mongodb.Operator = mongodbOperator

	if err := validateVersion(); err != nil {
		log.Fatal(err)
	}

	// Initialize Langfuse tracer for backend observability
	if err := observability.InitializeLangfuseTracer(); err != nil {
		log.Printf("Failed to initialize Langfuse tracer: %v", err)
		// Don't fail startup if Langfuse is not configured
	}

	// Initialize OTEL tracer for OTEL-compliant tracing to Langfuse
	if err := observability.InitOTELTracer(context.Background()); err != nil {
		log.Printf("Failed to initialize OTEL tracer: %v", err)
		// Don't fail startup if OTEL is not configured
	}

	// Emit an explicit startup mode so trace path is unambiguous in runtime logs.
	if observability.OTELTracerEnabled() {
		log.Println("[Observability] Mode: OTEL enabled (trace events via OTEL exporter; Langfuse REST remains active for direct Langfuse flows)")
	} else if observability.GetLangfuseTracer().IsEnabled() {
		log.Println("[Observability] Mode: Langfuse-only (OTEL disabled; traces/observations/scores via Langfuse REST)")
	} else {
		log.Println("[Observability] Mode: tracing disabled (no OTEL endpoint and no Langfuse credentials)")
	}

	enableHTTPSConnection, err := strconv.ParseBool(utils.Config.EnableInternalTls)
	if err != nil {
		log.Errorf("unable to parse boolean value %v", err)
	}

	if enableHTTPSConnection {
		if utils.Config.TlsCertPath == "" || utils.Config.TlsKeyPath == "" {
			log.Fatalf("Failure to start chaoscenter authentication REST server due to empty TLS cert file path and TLS key path")
		}
		go startGRPCServerWithTLS(mongodbOperator) // start GRPC serve
	} else {
		go startGRPCServer(utils.Config.GrpcPort, mongodbOperator) // start GRPC serve
	}

	srv := handler.New(generated.NewExecutableSchema(graph.NewConfig(mongodbOperator)))

	// Pass through actual error messages instead of generic "internal system error"
	srv.SetErrorPresenter(func(ctx context.Context, e error) *gqlerror.Error {
		err := graphql.DefaultErrorPresenter(ctx, e)
		return err
	})
	srv.SetRecoverFunc(func(ctx context.Context, err interface{}) error {
		log.Errorf("PANIC in GraphQL resolver: %v", err)
		return fmt.Errorf("panic: %v", err)
	})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					origin = r.Host
				}
				for _, allowedOrigin := range utils.Config.AllowedOrigins {
					match, err := regexp.MatchString(allowedOrigin, origin)
					if err == nil && match {
						return true
					}
				}
				return false
			},
		},
	})

	enableIntrospection, err := strconv.ParseBool(utils.Config.EnableGQLIntrospection)
	if err != nil {
		log.Errorf("unable to parse boolean value %v", err)
	}
	if enableIntrospection {
		srv.Use(extension.Introspection{})
	}

	// go routine for syncing chaos hubs
	go chaoshub.NewService(dbSchemaChaosHub.NewChaosHubOperator(mongodbOperator)).RecurringHubSync()
	go chaoshub.NewService(dbSchemaChaosHub.NewChaosHubOperator(mongodbOperator)).SyncDefaultChaosHubs()

	// go routine for syncing agent hub and app hub
	go agenthub.NewService(nil).SyncDefaultAgentHub()
	go apphub.NewService().SyncDefaultAppHub()

	// routers
	router.GET("/", handlers.PlaygroundHandler())
	router.Any("/query", authorization.Middleware(srv, mongodb.MgoClient))

	router.Any("/file/:key", handlers.FileHandler(mongodbOperator))

	//chaos hub routers
	router.GET("/icon/:projectId/:hubName/:chartName/:iconName", handler2.ChaosHubIconHandler())
	router.GET("/icon/default/:hubName/:chartName/:iconName", handler2.DefaultChaosHubIconHandler())

	// Helm chart routers
	router.POST("/api/helm/install", handlers.HelmChartUploadHandler(mongodb.MgoClient))
	router.DELETE("/api/helm/uninstall", handlers.HelmChartUninstallHandler(mongodb.MgoClient))
	router.GET("/api/helm/releases", handlers.HelmChartListHandler(mongodb.MgoClient))

	// Port-forward routers
	router.POST("/api/port-forward/start", handlers.PortForwardHandler(mongodb.MgoClient))
	router.POST("/api/port-forward/stop", handlers.StopPortForwardHandler(mongodb.MgoClient))

	//general routers
	router.GET("/status", handlers.StatusHandler())
	router.GET("/readiness", handlers.ReadinessHandler())

	projectEventChannel := make(chan string)
	go projects.ProjectEvents(projectEventChannel, mongodb.MgoClient, mongodbOperator)

	// Graceful shutdown handler for OTEL and Langfuse flush
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Infof("Received signal %v, shutting down tracers...", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := observability.GetLangfuseTracer().Close(shutdownCtx); err != nil {
			log.Errorf("Langfuse tracer shutdown error: %v", err)
		}
		if err := observability.ShutdownOTELTracer(shutdownCtx); err != nil {
			log.Errorf("OTEL tracer shutdown error: %v", err)
		}
		os.Exit(0)
	}()

	if enableHTTPSConnection {
		if utils.Config.TlsCertPath == "" || utils.Config.TlsKeyPath == "" {
			log.Fatalf("Failure to start chaoscenter authentication GRPC server due to empty TLS cert file path and TLS key path")
		}

		log.Infof("graphql server running at https://localhost:%s", utils.Config.RestPort)
		// configuring TLS config based on provided certificates & keys
		conf := utils.GetTlsConfig(utils.Config.TlsCertPath, utils.Config.TlsKeyPath, true)

		server := http.Server{
			Addr:      ":" + utils.Config.RestPort,
			Handler:   router,
			TLSConfig: conf,
		}
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Fatalf("Failure to start litmus-portal graphql REST server due to %v", err)
		}
	} else {
		log.Infof("graphql server running at http://localhost:%s", utils.Config.RestPort)
		log.Fatal(http.ListenAndServe(":"+utils.Config.RestPort, router))
	}
}

// startGRPCServer initializes, registers services to and starts the gRPC server for RPC calls
func startGRPCServer(port string, mongodbOperator mongodb.MongoOperator) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()

	// Register services

	pb.RegisterProjectServer(grpcServer, &projects.ProjectServer{Operator: mongodbOperator})

	log.Infof("GRPC server listening on %v", lis.Addr())
	log.Fatal(grpcServer.Serve(lis))
}

// startGRPCServerWithTLS initializes, registers services to and starts the gRPC server for RPC calls
func startGRPCServerWithTLS(mongodbOperator mongodb.MongoOperator) {

	lis, err := net.Listen("tcp", ":"+utils.Config.GrpcPort)
	if err != nil {
		log.Fatal("failed to listen: %w", err)
	}

	// configuring TLS config based on provided certificates & keys
	conf := utils.GetTlsConfig(utils.Config.TlsCertPath, utils.Config.TlsKeyPath, true)

	// create tls credentials
	tlsCredentials := credentials.NewTLS(conf)

	// create grpc server with tls credential
	grpcServer := grpc.NewServer(grpc.Creds(tlsCredentials))

	// Register services

	pb.RegisterProjectServer(grpcServer, &projects.ProjectServer{Operator: mongodbOperator})

	log.Infof("GRPC server listening on %v", lis.Addr())
	log.Fatal(grpcServer.Serve(lis))
}
