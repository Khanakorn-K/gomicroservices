package main

import (
	"context"
	"gomicroservice/cmd/initializer"
	catalogpb "gomicroservice/proto"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	initializer.LoadEnv()
	pool := initializer.ConnectDB()
	defer pool.Close()
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	catalogClient := catalogpb.NewCatalogServiceClient(conn)
	app := fiber.New()
	app.Use(cors.New())

	app.Get("/bookings", func(c fiber.Ctx) error {
		rows, err := pool.Query(c.Context(),
			`SELECT id, service_id, customer_name, status, create_at
		 FROM bookings
		 ORDER BY id`,
		)
		if err != nil {
			log.Printf("query bookings: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "database error",
			})
		}
		defer rows.Close()

		list := []booking{}
		for rows.Next() {
			var b booking
			if err := rows.Scan(&b.ID, &b.ServiceID, &b.CustomerName, &b.Status, &b.CreateAt); err != nil {
				log.Printf("scan booking: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "database error",
				})
			}
			list = append(list, b)
		}

		if err := rows.Err(); err != nil {
			log.Printf("iterate bookings: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "database error",
			})
		}

		return c.JSON(list)
	})

	app.Post("/booking", func(c fiber.Ctx) error {
		var body bookingRequest
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		if body.ServiceID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "service_id must be greater than 0",
			})
		}

		customerName := strings.TrimSpace(body.CustomerName)
		if customerName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "customer_name is required",
			})
		}

		customerName = body.CustomerName
		if customerName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "customer_name is required",
			})
		}

		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()

		resp, err := catalogClient.CheckServiceExists(ctx, &catalogpb.CheckServiceRequest{
			ServiceId: body.ServiceID,
		})
		if err != nil {
			log.Printf("CheckServiceExists: %v", err)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "catalog service unavailable",
			})
		}
		if !resp.GetExists() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "service not found",
			})
		}

		var b booking
		err = pool.QueryRow(ctx,
			`INSERT INTO bookings (service_id, customer_name)
		 VALUES ($1, $2)
		 RETURNING id, service_id, customer_name, status, create_at`,
			body.ServiceID, customerName,
		).Scan(&b.ID, &b.ServiceID, &b.CustomerName, &b.Status, &b.CreateAt)
		if err != nil {
			log.Printf("insert booking: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "database error",
			})
		}

		return c.Status(fiber.StatusCreated).JSON(b)
	})
	log.Fatal(app.Listen(":8081"))
}

type booking struct {
	ID           int64     `json:"id"`
	ServiceID    int64     `json:"service_id"`
	CustomerName string    `json:"customer_name"`
	Status       string    `json:"status"`
	CreateAt     time.Time `json:"create_at"`
}

type bookingRequest struct {
	ServiceID    int64  `json:"service_id"`
	CustomerName string `json:"customer_name"`
}
