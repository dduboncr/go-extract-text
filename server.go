package main

import (
	"extract/controllers"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func SetupRoutes(app *fiber.App) {
	app.Post("/process", controllers.Process)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Server is running",
		})
	})

}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file:", err)
	}

	app := fiber.New()

	SetupRoutes(app)

	app.Use(recover.New())
	app.Use(cors.New())
	app.Use(logger.New(logger.Config{
		Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
	}))

	port := os.Getenv("PORT")

	if port == "" {
		port = "3000"
	}

	fmt.Println("Server is running on port", port)
	app.Listen(":" + port)

}
