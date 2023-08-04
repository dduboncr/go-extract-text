package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"
)

func extractFrames(filePath string) ([]string, error) {
	// clean up frames folder
	os.RemoveAll("frames")
	fmt.Print("Removed frames folder\n")

	err := os.Mkdir("frames", 0755)
	if err != nil {
		fmt.Printf("Error creating frames folder: %s\n", err)
		return nil, err
	}

	// using ffmpeg to extract frames
	cmd := exec.Command("ffmpeg", "-i", filePath, "-vf", "fps=1", "frames/frame%d.jpg")

	// Set up the standard output and error output to be the same as the current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	err = cmd.Run()

	if err != nil {
		fmt.Printf("Error executing bash script %s: %s\n", filePath, err)
		return nil, err
	}

	files, err := ioutil.ReadDir("frames")
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

func tessExtractText(inputFile, language string, results chan string) {
	start := time.Now()

	fmt.Printf("Running tesseract on %s\n", inputFile)

	// echo $(tesseract frames/frame5.jpg stdout -l "eng")
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo $(tesseract frames/\"%s\" stdout -l \"%s\")", inputFile, language))
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
		results <- output
	}
}

func extractText(inputFile, timestamp, language string, results chan string) {
	start := time.Now()

	cmd := exec.Command("bash", "-c", fmt.Sprintf("source ./bin/extract-video-visual-texts.bash && extract_text \"%s\" \"%s\" \"%s\"", inputFile, timestamp, language))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		results <- ""
	}

	output := strings.TrimSpace(stdout.String())
	elapsed := time.Since(start)
	fmt.Printf("The function took %v to run.\n", elapsed.Seconds())
	results <- output
}

// func Process(context *fiber.Ctx) error {
// 	start := time.Now()

// 	var requestBody struct {
// 		FileUrl  string `json:"fileUrl"`
// 		Language string `json:"language"`
// 	}

// 	if err := context.BodyParser(&requestBody); err != nil {
// 		return context.Status(400).JSON(fiber.Map{
// 			"message": "fileUrl and language are required",
// 		})
// 	}

// 	fmt.Printf("FileUrl: %s\n", requestBody.FileUrl)

// 	// download file from fileUrl
// 	filePath, err := downloadFile(
// 		// requestBody.FileUrl, // "flowpi-test-bucket", // fileUrl
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
// 	timestamps = timestamps[:40]
// 	fmt.Printf("Timestamps: %s\n", timestamps)

// 	if err != nil {
// 		fmt.Printf("Error getting timestamps: %s\n", err)
// 		return context.Status(400).JSON(fiber.Map{
// 			"message": "error getting video duration",
// 		})
// 	}

// 	// pooling example
// 	// resultCh := make(chan string)
// 	// g := runtime.GOMAXPROCS(0)
// 	// fmt.Printf("GOMAXPROCS: %d\n", g)
// 	// for c := 0; c < g; c++ {
// 	// 	go func(child int) {
// 	// 		// for d := range resultCh {
// 	// 		// 	fmt.Printf("child %d : recv'd signal : %s\n", child, d)
// 	// 		// }
// 	// 		// fmt.Printf("child %d : recv'd shutdown signal\n", child)
// 	// 		for _, timestamp := range timestamps {
// 	// 			extractText(filePath, timestamp, requestBody.Language, resultCh)
// 	// 		}
// 	// 	}(c)
// 	// }

// 	// defer close(resultCh)

// 	maxWorkers := runtime.NumCPU()
// 	fmt.Printf("Max workers: %d\n", maxWorkers)
// 	// resultCh := make(chan string, maxWorkers)

// 	// go func() {
// 	// 	for _, timestamp := range timestamps {
// 	// 		extractText(filePath, timestamp, requestBody.Language, resultCh)
// 	// 	}
// 	// 	close(resultCh)
// 	// }()

// 	timestampsCh := make(chan string, maxWorkers)
// 	resultCh := make(chan string, len(timestamps))
// 	defer close(resultCh)
// 	go func() {
// 		for _, timestamp := range timestamps {
// 			timestampsCh <- timestamp
// 		}
// 		close(timestampsCh)
// 	}()
// 	for timestamp := range timestampsCh {
// 		extractText(filePath, timestamp, requestBody.Language, resultCh)
// 	}

// 	var textResults []string

// 	for i := 0; i < len(timestamps); i++ {
// 		result := <-resultCh
// 		textResults = append(textResults, result)
// 	}

// 	sortedResults := sortByTimestampAndRemoveDuplicates(textResults)

// 	fmt.Printf("Text results length: %d\n", len(textResults))
// 	fmt.Printf("Sorted results length: %d\n", len(sortedResults))

// 	// fmt.Printf("Text results: %s\n", textResults)
// 	// fmt.Printf("Sorted results: %s\n", sortedResults)

// 	elapsed := time.Since(start)
// 	fmt.Printf("The function took %v to run.\n", elapsed.Seconds())

// 	return context.Status(200).JSON(fiber.Map{
// 		"filePath":        filePath,
// 		"textResults":     sortedResults,
// 		"secondsToFinish": elapsed.Seconds(),
// 	})
// }

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

	fmt.Printf("FileUrl: %s\n", requestBody.FileUrl)

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

	frames, err := extractFrames(filePath)
	if err != nil {
		fmt.Printf("Error extracting frames: %s\n", err)
		return context.Status(500).JSON(fiber.Map{
			"message": "error extracting frames",
		})
	}

	fmt.Printf("Frames: %s\n", frames)

	maxWorkers := runtime.NumCPU()
	fmt.Printf("Max workers: %d\n", maxWorkers)
	resultCh := make(chan string, maxWorkers)

	go func() {
		for _, frame := range frames {
			tessExtractText(frame, requestBody.Language, resultCh)
		}
		defer close(resultCh)
	}()

	// timestampsCh := make(chan string, maxWorkers)
	// resultCh := make(chan string, len(timestamps))
	// go func() {
	// 	for _, timestamp := range timestamps {
	// 		timestampsCh <- timestamp
	// 	}
	// 	close(timestampsCh)
	// }()
	// for timestamp := range timestampsCh {
	// 	extractText(filePath, timestamp, requestBody.Language, resultCh)
	// }

	///////////////////////
	///////////////////////
	///////////////////////
	///////////////////////
	///////////////////////

	// timestamps, err := getTimestamps(filePath)
	// timestamps = timestamps[:40]
	// fmt.Printf("Timestamps: %s\n", timestamps)

	// if err != nil {
	// 	fmt.Printf("Error getting timestamps: %s\n", err)
	// 	return context.Status(400).JSON(fiber.Map{
	// 		"message": "error getting video duration",
	// 	})
	// }

	// maxWorkers := runtime.NumCPU()
	// fmt.Printf("Max workers: %d\n", maxWorkers)
	// // resultCh := make(chan string, maxWorkers)

	// // go func() {
	// // 	for _, timestamp := range timestamps {
	// // 		extractText(filePath, timestamp, requestBody.Language, resultCh)
	// // 	}
	// // 	close(resultCh)
	// // }()

	// timestampsCh := make(chan string, maxWorkers)
	// resultCh := make(chan string, len(timestamps))
	// go func() {
	// 	for _, timestamp := range timestamps {
	// 		timestampsCh <- timestamp
	// 	}
	// 	close(timestampsCh)
	// }()
	// for timestamp := range timestampsCh {
	// 	extractText(filePath, timestamp, requestBody.Language, resultCh)
	// }

	var textResults []string

	for i := 0; i < len(frames); i++ {
		result := <-resultCh
		textResults = append(textResults, result)
	}
	sortedResults := sortByTimestampAndRemoveDuplicates(textResults)

	fmt.Printf("Text results length: %d\n", len(textResults))
	fmt.Printf("Sorted results length: %d\n", len(sortedResults))

	fmt.Printf("Text results: %s\n", textResults)
	fmt.Printf("Sorted results: %s\n", sortedResults)

	elapsed := time.Since(start)
	fmt.Printf("The function took %v to run.\n", elapsed.Seconds())

	return context.Status(200).JSON(fiber.Map{
		"filePath":        filePath,
		"textResults":     sortedResults,
		"secondsToFinish": elapsed.Seconds(),
	})
}

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

func secondsToHHMMSS(seconds int) string {
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	seconds %= 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
