package controllers

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"
)

func secondsToTimeList(totalSeconds int) []string {
	var timeList []string

	for seconds := 1; seconds <= totalSeconds; seconds++ {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		remainingSeconds := seconds % 60

		// Pad single-digit hours, minutes, and seconds with leading zeros
		formattedTime := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, remainingSeconds)

		timeList = append(timeList, formattedTime)
	}

	return timeList
}

func getVideoDurationSeconds(inputFile string) (int64, error) {
	// Get file info
	fileInfo, err := os.Stat(inputFile)
	if err != nil {
		return 0, err
	}

	// Get the file's modification time
	modTime := fileInfo.ModTime()

	// Calculate the duration in seconds from the modification time
	durationSeconds := time.Since(modTime).Seconds()

	return int64(durationSeconds), nil
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

func getVideoDuration(inputVideoPath string) (string, error) {
	// get video duration using ffmpeg
	cmd := exec.Command("ffmpeg", "-i", inputVideoPath, "2>&1", "|", "grep", "Duration", "|", "awk", "'{print $2}'", "|", "tr", "-d", ",")
	// run the command and get the output
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error executing bash script %s: %s\n", inputVideoPath, err)
		return "", err
	}

	return string(out), nil
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

	duration, durationErr := getVideoDurationSeconds(filePath)
	secondList := secondsToTimeList(int(duration))

	if durationErr != nil {
		// print error
		fmt.Printf("Error getting video duration: %s\n", durationErr)
		return context.Status(400).JSON(fiber.Map{
			"message": "error getting video duration",
		})
	}
	// // extract frames from video
	// err = extractFrames(filePath)

	// if err != nil {
	// 	return context.Status(400).JSON(fiber.Map{
	// 		"message": "error extracting frames",
	// 	})
	// }

	context.Status(200).JSON(fiber.Map{
		"filePath":   filePath,
		"secondList": secondList,
	})

	return nil
}
