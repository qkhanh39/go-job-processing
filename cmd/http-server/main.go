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

	"github.com/jackc/pgx/v5/pgxpool"
	httpHandler "github.com/qkhanh39/go-job-processing/internal/adapter/http"
	"github.com/qkhanh39/go-job-processing/internal/adapter/repository"
	"github.com/qkhanh39/go-job-processing/internal/config"
	"github.com/qkhanh39/go-job-processing/internal/connector"
	"github.com/qkhanh39/go-job-processing/internal/entities"
	"github.com/qkhanh39/go-job-processing/internal/notification"
	"github.com/qkhanh39/go-job-processing/internal/service/handler"
	"github.com/qkhanh39/go-job-processing/internal/service/job"
	"github.com/qkhanh39/go-job-processing/internal/worker"
)

type App struct {
	router httpHandler.Router
	DB     *pgxpool.Pool
	worker *worker.Pool
}

func NewApp(cfg *config.Config) (*App, error) {
	pool, err := connector.NewDBConnector(
		context.Background(),
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Database,
		cfg.Database.Host,
		cfg.Database.SSLMode,
		cfg.Database.Port,
	)

	if err != nil {
		return nil, err
	}

	jobRepo := repository.New(pool)

	jobTypeHandler := constructHandlerMap()

	notifier := notification.NewChannelNotifier()

	jobService := job.New(
		jobRepo,
		cfg.MaxAttempts,
		jobTypeHandler,
		notifier,
	)

	router := httpHandler.New(jobService)

	jobWorker := worker.New(
		jobService,
		notifier.Channel(),
		cfg.Worker.PollInterval,
		cfg.Worker.Workers,
		cfg.Worker.QueueSize,
		cfg.Worker.ShutdownTimeout,
	)

	return &App{
		router: *router,
		DB:     pool,
		worker: jobWorker,
	}, nil
}

func constructHandlerMap() map[entities.JobType]handler.JobHandler {
	return map[entities.JobType]handler.JobHandler{
		entities.JobTypeSendEmail: handler.NewSendEmailJobHandler(),
	}
}

func (a *App) Run(ctx context.Context, port int) error {
	serverAddr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: a.router.SetupRoutes(),
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	workerErr := make(chan error, 1)
	serverErr := make(chan error, 1)

	go func() {
		workerErr <- a.worker.Run(runCtx)
	}()

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	var (
		runErr     error
		workerDone bool
	)

	select {
	case <-ctx.Done():
	case err := <-workerErr:
		workerDone = true

		if err != nil && !errors.Is(err, context.Canceled) {
			runErr = fmt.Errorf("worker stopped unexpectedly: %w", err)
		} else if ctx.Err() == nil {
			runErr = errors.New("worker stopped unexpectedly")
		}

	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("HTTP server failed: %w", err)
		}

		return nil
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		if runErr == nil {
			runErr = fmt.Errorf("shutdown HTTP server: %w", err)
		} else {
			runErr = errors.Join(
				runErr,
				fmt.Errorf("shutdown HTTP server: %w", err),
			)
		}
	}

	if !workerDone {
		select {
		case err := <-workerErr:
			if err != nil && !errors.Is(err, context.Canceled) {
				workerErrWrapped := fmt.Errorf("stop worker: %w", err)
				if runErr == nil {
					runErr = workerErrWrapped
				} else {
					runErr = errors.Join(runErr, workerErrWrapped)
				}
			}
		case <-shutdownCtx.Done():
			if runErr == nil {
				runErr = fmt.Errorf("worker shutdown timed out: %w", shutdownCtx.Err())
			} else {
				runErr = errors.Join(
					runErr,
					fmt.Errorf("worker shutdown timed out: %w", shutdownCtx.Err()),
				)
			}
		}
	}

	return runErr
}

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	defer app.DB.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	log.Println("Application initialized")

	if err := app.Run(ctx, cfg.Server.HTTPPort); err != nil {
		panic(fmt.Sprintf("Server failed: %v", err))
	}
}
