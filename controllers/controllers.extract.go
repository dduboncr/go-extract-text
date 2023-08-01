package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"
)

func extractFrames(filePath string) error {
	// clean up frames folder
	os.RemoveAll("frames")
	fmt.Print("Removed frames folder\n")

	// using ffmpeg to extract frames
	cmd := exec.Command("ffmpeg", "-i", filePath, "-vf", "fps=1", "frames/frame%d.jpg")

	// Set up the standard output and error output to be the same as the current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	err := cmd.Run()

	if err != nil {
		fmt.Printf("Error executing bash script %s: %s\n", filePath, err)
		return err
	}

	return nil
}

func fileExists(filename string) bool {
	// Use os.Stat to get file information
	_, err := os.Stat(filename)

	// Check if there was no error and the file exists
	if err == nil {
		return true
	}

	// If os.Stat returns an error, check if it's a "not found" error
	if os.IsNotExist(err) {
		return false
	}

	// If there was some other error (permissions, etc.), return false as well
	return false
}

func downloadFile(bucketName, objectName, localFilePath string) (string, error) {
	if fileExists(localFilePath) {
		fmt.Printf("File already exists in local: %s\n", localFilePath)
		return localFilePath, nil
	}

	// Create a context and a storage client using your Google Cloud credentials.
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithCredentialsFile("./credentials.json"))

	if err != nil {
		return "", fmt.Errorf("error creating GCS client: %v", err)
	}
	defer client.Close()

	// Open the GCS bucket and the object (file) you want to download.
	bucket := client.Bucket(bucketName)
	object := bucket.Object(objectName)

	// Create a new local file to save the downloaded data.
	localFile, err := os.Create(localFilePath)
	if err != nil {
		return "", fmt.Errorf("error creating local file: %v", err)
	}
	defer localFile.Close()

	// Download the object data and write it to the local file.
	reader, err := object.NewReader(ctx)
	if err != nil {
		return "", fmt.Errorf("error reading object: %v", err)
	}
	defer reader.Close()

	if _, err := io.Copy(localFile, reader); err != nil {
		return "", fmt.Errorf("error copying object data to local file: %v", err)
	}

	return localFilePath, nil
}

func getTimestamps(inputFile string) ([]string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("source ./bin/extract-video-visual-texts.bash && get_timestamps \"%s\"", inputFile))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error executing bash script: %s", stderr.String())
	}
	output := strings.TrimSpace(stdout.String())
	timestamps := strings.Split(output, " ")
	return timestamps, nil
}

func extractText(inputFile, timestamp string) (string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("source ./bin/extract-video-visual-texts.bash && extract_text \"%s\" \"%s\"", inputFile, timestamp))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error executing bash script: %s", stderr.String())
	}
	output := strings.TrimSpace(stdout.String())
	return output, nil
}

func Process(context *fiber.Ctx) error {

	var requestBody struct {
		fileUrl string `json:"fileUrl"`
	}

	if err := context.BodyParser(&requestBody); err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "fileUrl is required",
		})
	}

	// download file from fileUrl
	filePath, err := downloadFile(
		"flowpi-test-bucket", // fileUrl
		"extract_text.mp4",   // objectName
		"extract_text.mp4",   // localFilePath
	)

	fmt.Printf("Filepath: %s\n", filePath)

	if err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "error downloading file",
		})
	}

	timestamps, err := getTimestamps(filePath)
	if err != nil {
		fmt.Printf("Error getting timestamps: %s\n", err)
		return context.Status(400).JSON(fiber.Map{
			"message": "error getting video duration",
		})
	}

	// var extractedTexts []string
	// for _, timestamp := range timestamps {
	// 	text, err := extractText(filePath, timestamp)
	// 	if err != nil {
	// 		return context.Status(400).JSON(fiber.Map{
	// 			"message": "error extracting text",
	// 		})
	// 	}
	// 	extractedTexts = append(extractedTexts, text)
	// }

	numCPU := runtime.NumCPU()
	maxWorkers := runtime.GOMAXPROCS(numCPU / 2)
	var extractedTexts []string
	var wg sync.WaitGroup
	ch := make(chan string, len(timestamps))
	workerCh := make(chan int, maxWorkers)

	// print maxWorkers
	fmt.Printf("Max workers: %d\n", maxWorkers)

	for _, timestamp := range timestamps {
		fmt.Printf("timestamp:", timestamp)
		activeWorkers := runtime.NumGoroutine()
		if activeWorkers < maxWorkers {
			wg.Add(1)
			workerCh <- 1 // Send a signal to the worker to indicate a new task
			go func(ts string) {
				defer wg.Done()
				text, err := extractText(filePath, ts)
				if err != nil {
					// You may handle the error as needed
					// For simplicity, let's just log the error here
					fmt.Println("Error extracting text:", err)
					return
				}
				ch <- text
			}(timestamp)
			wg.Wait()
		} else {
			// Wait for a worker to finish before sending a new signal
			fmt.Printf("waiting for a new worker:", timestamp)
			wg.Wait()
		}
	}

	wg.Wait()
	close(ch)
	close(workerCh)

	for text := range ch {
		extractedTexts = append(extractedTexts, text)
	}

	// print content of extractedTexts
	fmt.Printf("Extracted texts: %s\n", extractedTexts)

	context.Status(200).JSON(fiber.Map{
		"filePath":   filePath,
		"timestamps": timestamps,
	})

	return nil
}
