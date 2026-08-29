package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	usecases "gitlab.com/URL-shortener4224128/analytics-repository/src/application/use_cases"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/ports/out"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/infrastructure/adapters"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/infrastructure/gotel"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/infrastructure/persistence/impl"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/infrastructure/presentation/controller"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	amqp "github.com/rabbitmq/amqp091-go"

	otelmetric "go.opentelemetry.io/otel/metric"
	
	"database/sql"
	"log"
	"time"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

type container struct {
	db *sql.DB
	rabbitChan *amqp.Channel
	rabbitCon *amqp.Connection
	telemetry gotel.TelemetryProvider
	analyticsController *controller.AnalyticsController
	analyticsConsumer *adapters.AnalyticsConsumer
	analyticsRepo out.AnalyticRepo
	latencyHistogram otelmetric.Int64Histogram
}

func NewContainer(ctx context.Context) (*container, error) {

	container := &container{}

	telemetry, err := gotel.NewTelemetry(ctx, gotel.NewConfigFromEnv())

	if err != nil {
		return nil, err
	}

	container.telemetry = telemetry

	latencyHistogram, err := container.telemetry.MeterInt64Histogram(gotel.Metric{
			Name: "analytics_http_request_duration_ms",
			Description: "Request latency",
			Unit: "ms",
	})

	if err != nil {
		return nil, err
	}

	container.latencyHistogram = latencyHistogram


	conn, ch, err := newRabbitMQChannel()

	if err != nil {
		return nil, err
	}

	container.rabbitChan = ch
	container.rabbitCon = conn

	db, err := newDB(
		os.Getenv("PSQL_HOST"),
		os.Getenv("PSQL_PORT"),
		os.Getenv("PSQL_USER"),
		os.Getenv("PSQL_PASS"),
		os.Getenv("DBNAME"),
	)

	if err != nil {
		return nil, err
	}

	container.db = db	

	container.analyticsRepo = impl.NewAnalyticRepoPostgres(container.db, container.telemetry)

	consumer, err := adapters.NewAnalyticsConsumer(
		usecases.NewSaveAnalyticsUseCase(
			container.analyticsRepo,
		),
		container.rabbitChan,
		container.telemetry,
	)

	if err != nil {
		return nil, err
	}

	container.analyticsConsumer = consumer

	container.analyticsController = controller.NewAnalyticsController(
		usecases.NewGetAnalyticsByDeviceUseCase(
			container.analyticsRepo,
		),
		container.telemetry,
		container.latencyHistogram,
	)

	return container, nil
}

func newRabbitMQChannel() (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(os.Getenv("RABBITMQ"))

	ch, err := conn.Channel()
	
	return conn, ch, err
}


func newDB(host string, port string, user string, password string, dbname string) (*sql.DB, error) {
	db, err := sql.Open("postgres", fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", 
			host,
			user,
			password,
			dbname,
			port,
		),
	)

	if err != nil {
			return nil, err
	}

	db.SetMaxOpenConns(25) 
	
	db.SetMaxIdleConns(25) 
	
	db.SetConnMaxLifetime(5 * time.Minute) 
	
	db.SetConnMaxIdleTime(1 * time.Minute) 

	if err := db.Ping(); err != nil {
			return nil, err
	}

	return db, nil
}

func migrate(db *sql.DB) error {
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS urls (
            id SERIAL PRIMARY KEY,
						domain VARCHAR(255) NOT NULL,
            code VARCHAR(255) UNIQUE NOT NULL,
            url TEXT UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
    `)

    if err != nil {
        return err
    }

    _, err = db.Exec(`
        CREATE UNIQUE INDEX IF NOT EXISTS idx_urls_code
        ON urls (code);
    `)

		if err != nil {
        return err
    }

		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS url_visits (
					id SERIAL PRIMARY KEY,
					url_id INT NOT NULL,
					device_name VARCHAR(255) NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

					CONSTRAINT fk_url_id
						FOREIGN KEY (url_id) 
						REFERENCES urls(id) 
						ON DELETE CASCADE 
						ON UPDATE CASCADE

			);
		`)

    if err != nil {
        return err
    }

    return nil
}

func main() {
	err := godotenv.Load()

	failOnError(err, "Load .env file failed")

	ctx := context.Background()

	container, err := NewContainer(ctx)

	if err != nil {
		container.telemetry.LogFatalln("In main.go: ", err.Error())
		container.telemetry.Shutdown(ctx)
		panic(err)
	}

	defer container.telemetry.Shutdown(ctx)
	defer container.db.Close()
	defer container.rabbitCon.Close()
	defer container.rabbitChan.Close()	

	container.analyticsConsumer.Consume(ctx)

	migrate(container.db)

	mux := http.NewServeMux()
	
	handlerWithOtel := otelhttp.NewHandler(mux, "analytics service")
	
	server := http.Server{
		Addr:    ":3002",
		Handler: handlerWithOtel,
	}
	
	mux.HandleFunc("/analytics/devices", container.analyticsController.GetAnalyticsByDevice)

	fmt.Println("Server listen :3002")

	if err := server.ListenAndServe(); err != nil {
		container.telemetry.LogFatalln("In main.go: ", err.Error())
		panic(err)
	}
}
