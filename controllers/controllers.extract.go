package controllers

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/otiai10/gosseract"
)

func run(scriptPath string, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Goroutine started\n")

	cmd := exec.Command("bash", scriptPath)


	// Set up the standard output and error output to be the same as the current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	err := cmd.Run()

	

	if err != nil {
		fmt.Printf("Error executing bash script %s: %s\n", scriptPath, err)
	}

	fmt.Printf("Goroutine finished\n")
}

func Process(context *fiber.Ctx) error {

	// get fileUrl from body
	var requestBody struct {
		fileUrl string `json:"fileUrl"`
	}

	if err := context.BodyParser(&requestBody); err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "fileUrl is required",
		})
	}


	// fileURL := requestBody.fileUrl


	var wg sync.WaitGroup
	wg.Add(1)

	go run("./bin/hello-world.bash", &wg)
	wg.Wait()
	fmt.Println("All goroutines completed.")

	context.Status(200).JSON(fiber.Map{
		"message": "Hello, World!",
	})
	return nil
}


func processImageStream(imageStream []byte) (string, error) {
	// Create a new gosseract client
	client := gosseract.NewClient()

	// Set the image stream as the image to be processed
	err := client.SetImageFromBytes(imageStream)
	if err != nil {
		return "", err
	}

	// Perform OCR on the image
	text, err := client.Text()
	if err != nil {
		return "", err
	}

	// Close the client
	client.Close()

	return text, nil
}






