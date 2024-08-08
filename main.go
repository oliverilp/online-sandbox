package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/template/django/v3"
)

func main() {
	engine := django.New("./views", ".django")
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

	app.Static("/", "./public")

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			"Language": "python",
			"Code":     "",
		})
	})

	app.Post("/", func(c *fiber.Ctx) error {
		language := c.FormValue("language")
		code := c.FormValue("code")
		output, err := runCode(language, code)
		if err != nil {
			if _, ok := err.(*TimeoutError); ok {
				output = "Execution timed out after 10 seconds."
			} else {
				output = "Something went wrong while execution your code."
			}
		}

		return c.Render("index", fiber.Map{
			"Language": language,
			"Code":     code,
			"Output":   output,
		})
	})

	log.Println("Starting server at :4000")
	log.Fatal(app.Listen(":4000"))
}
