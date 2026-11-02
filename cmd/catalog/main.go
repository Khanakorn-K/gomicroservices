package main

import (
	"context"
	"gomicroservice/cmd/initializer"
	"log"
	"net"
	"strings"

	catalogpb "gomicroservice/proto"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

type catalogServer struct {
	catalogpb.UnimplementedCatalogServiceServer
	pool *pgxpool.Pool
}

func (s *catalogServer) CheckServiceExists(ctx context.Context, req *catalogpb.CheckServiceRequest) (*catalogpb.CheckServiceResponse, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM services WHERE  id = $1)",
		req.GetServiceId(),
	).Scan(&exists)
	if err != nil {
		return nil, err
	}
	return &catalogpb.CheckServiceResponse{Exists: exists}, nil
}

type service struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type serviceRequest struct {
	Name string `json:"name"`
}

func main() {
	initializer.LoadEnv()
	pool := initializer.ConnectDB()
	defer pool.Close()

	app := fiber.New()
	app.Use(cors.New())

	app.Get("/services", func(c fiber.Ctx) error {
		rows, err := pool.Query(c.Context(), "SELECT id, name FROM services ORDER BY id")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "database error",
			})
		}
		defer rows.Close()

		list := []service{}
		for rows.Next() {
			var s service
			if err := rows.Scan(&s.ID, &s.Name); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "database error",
				})
			}
			list = append(list, s)
		}

		if err := rows.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "database error",
			})
		}

		return c.JSON(list)
	})
	app.Post("/services", func(c fiber.Ctx) error {
		var body serviceRequest
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		name := strings.TrimSpace(body.Name)
		if name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "name is required",
			})
		}

		var s service
		err := pool.QueryRow(
			c.Context(),
			"INSERT INTO services (name) VALUES ($1) RETURNING id, name",
			name,
		).Scan(&s.ID, &s.Name)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "database error",
			})
		}

		return c.Status(fiber.StatusCreated).JSON(s)
	})

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	catalogpb.RegisterCatalogServiceServer(grpcServer, &catalogServer{pool: pool})
	go func() {
		log.Fatal(grpcServer.Serve(lis))
	}()
	log.Fatal(app.Listen(":8080"))
}
