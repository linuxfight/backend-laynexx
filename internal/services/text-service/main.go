package main

import (
	"api-repository/internal/adapters/interceptors"
	"api-repository/internal/config"
	"api-repository/internal/services"
	"api-repository/internal/services/text-service/service"
	textService "api-repository/pkg/api/text-service"
	"api-repository/pkg/db/postgres"
	"api-repository/pkg/db/redis"
	"api-repository/pkg/utils"
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"time"

	"google.golang.org/grpc"
)

func main() {
	utils.CreateNewSugaredLogger()

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	cfg, err := config.NewMainConfig()
	if err != nil {
		log.Fatalf("can't get env files | err: %v", err)
	}

	pgConn, err := postgres.NewPostgres(cfg.POSTGRES)
	if err != nil {
		log.Fatalf("can't connect to postgres | err: %v", err)
	}

	redisConn := redis.NewRedisConn(cfg.REDIS)
	_, err = redisConn.Ping(ctx).Result()
	if err != nil {
		log.Printf("can't connect to redis | err: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.TextServicePort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	jwtManager := utils.NewJWTManager(cfg.SecretToken, 7*24*time.Hour)
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			interceptors.AuthInterceptor(jwtManager),
			interceptors.LoggingInterceptor(utils.GetSugaredLogger()),
		),
	}
	server := grpc.NewServer(opts...)

	svc := service.NewTextService(pgConn, redisConn)
	textService.RegisterTextServer(server, svc)

	log.Printf("Configuration:\n%s", services.GetBeautifulConfigurationString(cfg))
	log.Println(services.GetServerStartedLogString(time.Now(), cfg.TextServicePort, "text-service"))
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	<-ctx.Done()
	server.GracefulStop()
	log.Print("text service stopped")
}
