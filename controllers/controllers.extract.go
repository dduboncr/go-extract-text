package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

func extractFrames(localFilePath, framesFolder, filename string) ([]string, error) {
	// clean up frames folder
	os.RemoveAll(framesFolder)
	fmt.Print("Removed frames folder\n")

	err := os.Mkdir(framesFolder, 0755)
	if err != nil {
		fmt.Printf("Error creating frames folder: %s\n", err)
		return nil, err
	}

	filepath := localFilePath + "/" + filename
	// using ffmpeg to extract frames
	cmd := exec.Command("ffmpeg", "-i", filepath, "-vf", "fps=1", framesFolder+"/frame_sec_%d.jpg")

	// Set up the standard output and error output to be the same as the current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	err = cmd.Run()

	if err != nil {
		fmt.Printf("Error executing bash script %s: %s\n", filepath, err)
		return nil, err
	}

	files, err := os.ReadDir(framesFolder)
	if err != nil {
		return nil, err
	}

	var fileNames []string
	for _, file := range files {
		if !file.IsDir() {
			fileNames = append(fileNames, file.Name())
		}
	}

	return fileNames, nil
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

func downloadFile(bucketName, bucketFilepath, bucketFilename, localFilePath string) (string, error) {
	localFilename := localFilePath + "/" + bucketFilename
	if fileExists(localFilename) {
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
	object := bucket.Object(bucketFilepath)

	// Create a new local file to save the downloaded data.
	localFile, err := os.Create(localFilename)
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

func extractText(inputFilename, tmpFramerFolder, language string, results chan<- string) {
	fmt.Printf("Running tesseract on %s\n", inputFilename)

	start := time.Now()
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo $(tesseract %s/\"%s\" stdout -l \"%s\")", tmpFramerFolder, inputFilename, language))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		results <- ""
	} else {
		output := strings.TrimSpace(stdout.String())
		elapsed := time.Since(start)
		fmt.Printf("The function took %v to run.\n", elapsed.Seconds())
		secs, err := extractSeconds(inputFilename)
		if err != nil {
			results <- ""
		} else {
			hhmmss := secondsToHHMMSS(secs)
			results <- fmt.Sprintf("%s %s", hhmmss, output)
		}
	}
}

func extractBucketNameAndObjectPath(fileUrl string) (string, string, string, error) {
	regex := regexp.MustCompile(`gs://([^/]+)/(.+)`)
	matches := regex.FindStringSubmatch(fileUrl)
	if len(matches) < 3 {
		return "", "", "", fmt.Errorf("invalid fileUrl")
	}
	bucketName := matches[1]
	bucketFilepath := matches[2]

	bucketFilepathParts := strings.Split(bucketFilepath, "/")
	bucketFilename := bucketFilepathParts[len(bucketFilepathParts)-1]

	return bucketName, bucketFilepath, bucketFilename, nil
}

func Process(context *fiber.Ctx) error {
	start := time.Now()

	var requestBody struct {
		FileUrl  string `json:"fileUrl"`
		Language string `json:"language"`
	}

	if err := context.BodyParser(&requestBody); err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "fileUrl and language are required",
		})
	}

	uuid := uuid.New().String()
	fmt.Printf("UUID: %s\n", uuid)

	err := os.Mkdir("/tmp/"+uuid, 0755)
	if err != nil {
		return context.Status(500).JSON(fiber.Map{
			"message": "Error creating tmp file folder:" + err.Error(),
		})
	}

	localFilePath := "/tmp/" + uuid
	bucketName, bucketFilepath, bucketFilename, err := extractBucketNameAndObjectPath(requestBody.FileUrl)
	if err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	filePath, err := downloadFile(
		bucketName,
		bucketFilepath,
		bucketFilename,
		localFilePath,
	)

	fmt.Printf("Filepath: %s\n", filePath)
	if err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "error downloading file",
		})
	}

	framesFolder := localFilePath + "/frames"
	frames, err := extractFrames(localFilePath, framesFolder, bucketFilename)
	if err != nil {
		fmt.Printf("Error extracting frames: %s\n", err)
		return context.Status(500).JSON(fiber.Map{
			"message": "error extracting frames",
		})
	}

	maxWorkers := runtime.NumCPU() / 2
	framesCh := make(chan string, len(frames))
	resultsCh := make(chan string, len(frames))

	for i := 0; i < maxWorkers; i++ {
		go worker(framesCh, framesFolder, requestBody.Language, resultsCh)
	}

	for _, frame := range frames {
		framesCh <- frame
	}

	close(framesCh)

	var textResults []string

	for i := 0; i < len(frames); i++ {
		result := <-resultsCh
		textResults = append(textResults, result)
	}

	close(resultsCh)

	sortedResults := sortByTimestampAndRemoveDuplicates(textResults)
	os.RemoveAll(localFilePath)

	fmt.Printf("Text results length: %d\n", len(textResults))
	fmt.Printf("Sorted results length: %d\n", len(sortedResults))

	elapsed := time.Since(start)
	fmt.Printf("The function took %v to run.\n", elapsed.Seconds())

	return context.Status(200).JSON(fiber.Map{
		"filePath":        filePath,
		"textResults":     sortedResults,
		"secondsToFinish": elapsed.Seconds(),
	})
}

func worker(framesCh <-chan string, framesFolder, language string, resultsCh chan<- string) {
	for frame := range framesCh {
		extractText(frame, framesFolder, language, resultsCh)
	}
}

func normalizeText(inputText string) string {
	// Replace line breaks and tabs with empty strings
	normalizedText := strings.ReplaceAll(inputText, "\n", " ")
	normalizedText = strings.ReplaceAll(normalizedText, "\t", "")
	// Trim leading and trailing whitespaces
	normalizedText = strings.TrimSpace(normalizedText)

	return normalizedText
}

func sortByTimestampAndRemoveDuplicates(arr []string) map[string]string {
	sortedArr := sortByTimestamp(arr)

	// Filter out empty strings with or without timestamps and those with timestamps but no text
	filteredArr := make([]string, 0)
	for idx, s := range sortedArr {
		timestamp := extractTimestamp(s)

		if len(s) < 9 {
			fmt.Printf("String \"%s\" is too short\n", s)
			continue
		}

		normalized := normalizeText(s[8:])

		// Check if the string is empty or has a valid timestamp but no accompanying text
		if normalized != "" && timestamp != "" {
			nextText := sortedArr[idx+1]

			// Check if the next string is the last in the array
			// In that case compare the current string with the previous string
			if idx == len(sortedArr)-1 {
				nextText = sortedArr[idx-2]
			}

			nextNormalizedText := normalizeText(nextText[8:])
			if !strings.HasPrefix(normalized, nextNormalizedText) || !stringExistsInArray(normalized, filteredArr) {
				filteredArr = append(filteredArr, normalized+" "+timestamp)
			}
		}
	}

	return mapTextsByTimestamp(filteredArr)
}

func mapTextsByTimestamp(texts []string) map[string]string {
	textsMap := make(map[string]string)
	for _, text := range texts {
		timestamp := text[len(text)-8:]
		if timestamp != "" {
			textsMap[timestamp] = strings.TrimSpace(text[:len(text)-8])
		}
	}
	return textsMap
}

func stringExistsInArray(target string, arr []string) bool {
	for _, str := range arr {
		if str == target {
			return true
		}
	}
	return false
}

func extractTimestamp(s string) string {
	// Regular expression to match the timestamp inside << >>
	re := regexp.MustCompile(`(\d{2}:\d{2}:\d{2})`)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func sortByTimestamp(arr []string) []string {
	// Create a custom struct to hold the original string and its timestamp
	type tuple struct {
		originalString string
		timestamp      string
	}

	// Extract timestamps and create a slice of tuples
	tuples := make([]tuple, len(arr))
	for i, s := range arr {
		tuples[i] = tuple{
			originalString: s,
			timestamp:      extractTimestamp(s),
		}
	}

	// Sort the slice of tuples based on the timestamps in descending order
	sort.SliceStable(tuples, func(i, j int) bool {
		return tuples[i].timestamp > tuples[j].timestamp
	})

	// Extract and return the sorted strings
	sortedStrings := make([]string, len(arr))
	for i, t := range tuples {
		sortedStrings[i] = t.originalString
	}
	return sortedStrings
}

func extractSeconds(filename string) (int, error) {
	re := regexp.MustCompile(`frame_sec_(\d+)\.jpg`)
	matches := re.FindStringSubmatch(filename)

	if len(matches) == 2 {
		secondsStr := matches[1]
		seconds, err := strconv.Atoi(secondsStr)
		if err != nil {
			fmt.Println("Error converting seconds to integer:", err)
			return 0, err
		}
		return seconds, nil
	}

	return 0, errors.New("no seconds found in the input string")
}

func secondsToHHMMSS(seconds int) string {
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	seconds %= 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
