package controllers

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"
)

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

	// download file from fileUrl
	filePath, err := downloadFile(
		"flowpi-test-bucket",
		"extract_text.mp4",
		"extract_text.mp4",
	)

	// log filepath
	fmt.Printf("Filepath: %s\n", filePath)

	if err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "error downloading file",
		})
	}

	// extract frames from video
	err = extractFrames(filePath)

	if err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "error extracting frames",
		})
	}

	context.Status(200).JSON(fiber.Map{
		"filePath": filePath,
	})
	return nil
}
