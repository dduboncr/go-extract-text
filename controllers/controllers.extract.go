package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
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

func extractText(inputFile, timestamp, language string, results chan string) (string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("source ./bin/extract-video-visual-texts.bash && extract_text \"%s\" \"%s\" \"%s\"", inputFile, timestamp, language))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error executing bash script: %s", stderr.String())
	}
	output := strings.TrimSpace(stdout.String())

	results <- output

	return output, nil
}

func Process(context *fiber.Ctx) error {

	var requestBody struct {
		FileUrl  string `json:"fileUrl"`
		Language string `json:"language"`
	}

	if err := context.BodyParser(&requestBody); err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "fileUrl and language are required",
		})
	}

	fmt.Printf("FileUrl: %s\n", requestBody.FileUrl)

	// download file from fileUrl
	filePath, err := downloadFile(
		// requestBody.FileUrl, // "flowpi-test-bucket", // fileUrl
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
	timestamps = timestamps[:10]
	fmt.Printf("Timestamps: %s\n", timestamps)

	if err != nil {
		fmt.Printf("Error getting timestamps: %s\n", err)
		return context.Status(400).JSON(fiber.Map{
			"message": "error getting video duration",
		})
	}

	g := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	wg.Add(g)

	ch := make(chan string, g)

	resultsCh := make(chan string, len(timestamps))

	for c := 0; c < g; c++ {
		go func(child int) {
			fmt.Printf("child %d : waiting for work\n", child)
			defer wg.Done()
			for timestamp := range ch {
				extractText(filePath, timestamp, "eng", resultsCh)
			}
			fmt.Printf("child %d : recv'd shutdown signal\n", child)
		}(c)
	}

	for _, timestamp := range timestamps {
		fmt.Printf("Sending Timestamp to Channel: %s\n", timestamp)
		ch <- timestamp
	}

	wg.Wait()
	// Close the results channel after all Go routines have finished
	close(ch)
	close(resultsCh)
	fmt.Print("All goroutines finished...\n")

	var textResults []string

	for text := range resultsCh {
		fmt.Printf("Text: %s\n", text)
		textResults = append(textResults, text)
	}

	sortedResults := sortByTimestampAndRemoveDuplicates(textResults)

	fmt.Printf("Text results length: %d\n", len(textResults))
	fmt.Printf("Sorted results length: %d\n", len(sortedResults))

	fmt.Printf("Text results: %s\n", textResults)
	fmt.Printf("Sorted results: %s\n", sortedResults)

	return context.Status(200).JSON(fiber.Map{
		"filePath":    filePath,
		"textResults": sortedResults,
	})
}

// func Process(context *fiber.Ctx) error {

// 	var requestBody struct {
// 		fileUrl  string `json:"fileUrl"`
// 		language string `json:"language"`
// 	}

// 	if err := context.BodyParser(&requestBody); err != nil {
// 		return context.Status(400).JSON(fiber.Map{
// 			"message": "fileUrl and language are required",
// 		})
// 	}

// 	// download file from fileUrl
// 	filePath, err := downloadFile(
// 		"flowpi-test-bucket", // fileUrl
// 		"extract_text.mp4",   // objectName
// 		"extract_text.mp4",   // localFilePath
// 	)

// 	fmt.Printf("Filepath: %s\n", filePath)

// 	if err != nil {
// 		return context.Status(400).JSON(fiber.Map{
// 			"message": "error downloading file",
// 		})
// 	}

// 	timestamps, err := getTimestamps(filePath)

// 	if err != nil {
// 		fmt.Printf("Error getting timestamps: %s\n", err)
// 		return context.Status(400).JSON(fiber.Map{
// 			"message": "error getting video duration",
// 		})
// 	}

// 	resultsChan := make(chan string, len(timestamps))
// 	go processFile(10, filePath, timestamps, "eng", resultsChan)
// 	result := <-resultsChan

// 	fmt.Printf("Result: %s\n", result)

// 	fmt.Printf("Results: %v", len(resultsChan))
// 	fmt.Print("All goroutines finished...\n")

// 	var textResults []string

// 	sortedResults := sortByTimestampAndRemoveDuplicates(textResults)

// 	fmt.Printf("Text results length: %d\n", len(textResults))
// 	fmt.Printf("Sorted results length: %d\n", len(sortedResults))

// 	fmt.Printf("Text results: %s\n", textResults)
// 	fmt.Printf("Sorted results: %s\n", sortedResults)

// 	context.Status(200).JSON(fiber.Map{
// 		"filePath":    filePath,
// 		"textResults": sortedResults,
// 	})

// 	return nil
// }

// func processFile(concurrency int, inputFilePath string, timestamps []string, language string, results chan string) error {
// 	in := make(chan int, runtime.NumCPU()) // which frame (timestamp) to extract
// 	// returnChannel := make(chan string)     // capture extracted texts to this channel

// 	for process := 0; process < concurrency; process++ {
// 		go extractTextWorker(in, results, inputFilePath, timestamps, language)
// 	}

// 	go func() {
// 		for x := 0; x < len(timestamps); x++ {
// 			in <- x
// 		}
// 		close(in)
// 	}()

// 	for r := range results {
// 		results <- r
// 		if r == "__EXTRACTION_COMPLETED__" {
// 			break
// 		}
// 	}

// 	return nil
// }

// func extractTextWorker(in chan int, returnChan chan string, inputFilePath string, timestamps []string, language string) {
// 	for x := range in {
// 		output, err := _extractText(inputFilePath, timestamps[x], language)

// 		if err != nil {
// 			fmt.Printf("Error extracting text from timestamp \"%s\":\n%s\n", timestamps[x], err)
// 			returnChan <- ""
// 		} else {
// 			returnChan <- output
// 		}
// 	}

// 	returnChan <- "__EXTRACTION_COMPLETED__"
// }

// func _extractText(inputFile, timestamp, language string) (string, error) {
// 	fmt.Printf("Extracting text from timestamp \"%s\"...\n", timestamp)

// 	cmd := exec.Command("bash", "-c", fmt.Sprintf("source ./bin/extract-video-visual-texts.bash && extract_text \"%s\" \"%s\" \"%s\"", inputFile, timestamp, language))
// 	var stdout, stderr bytes.Buffer
// 	cmd.Stdout = &stdout
// 	cmd.Stderr = &stderr
// 	err := cmd.Run()
// 	if err != nil {
// 		return "", fmt.Errorf("error executing bash script: %s", stderr.String())
// 	}
// 	output := strings.TrimSpace(stdout.String())

// 	return output, nil
// }

func normalizeText(inputText string) string {
	// Replace line breaks and tabs with empty strings
	normalizedText := strings.ReplaceAll(inputText, "\n", " ")
	normalizedText = strings.ReplaceAll(normalizedText, "\t", "")
	// Trim leading and trailing whitespaces
	normalizedText = strings.TrimSpace(normalizedText)

	return normalizedText
}

func sortByTimestampAndRemoveDuplicates(arr []string) []string {
	sortedArr := sortByTimestamp(arr)

	fmt.Printf("Sorted array: %s\n", sortedArr)

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
			// fmt.Printf("normalized: %s\n", normalized)
			// fmt.Printf("nextText: %s\n", nextNormalizedText)
			// fmt.Printf("HasPrefix: %t\n", strings.HasPrefix(normalized, nextNormalizedText))

			if !strings.HasPrefix(normalized, nextNormalizedText) || !stringExistsInArray(normalized, filteredArr) {
				filteredArr = append(filteredArr, normalized)
			}
		}
	}

	return filteredArr
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
