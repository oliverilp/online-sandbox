package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/template/html/v2"
)

type PageData struct {
	Output string
}

func main() {
	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Rate limiting middleware
	app.Use(limiter.New(limiter.Config{
		Max:        5,
		Expiration: 10 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).SendString("Too many requests. Please try again later.")
		},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			"Code": "",
		})
	})

	app.Post("/", func(c *fiber.Ctx) error {
		code := c.FormValue("code")
		output, err := runPHPCode(code)
		if err != nil {
			if _, ok := err.(*TimeoutError); ok {
				output = "Execution timed out after 10 seconds."
			} else {
				output = "Something went wrong while execution your code."
			}
		}

		return c.Render("index", fiber.Map{
			"Output": output,
			"Code":   code,
		})
	})

	log.Println("Starting server at :8080")
	log.Fatal(app.Listen("localhost:8080"))
}
