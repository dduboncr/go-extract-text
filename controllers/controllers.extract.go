package controllers

import (
	"bytes"
	"extract/storage"
	"extract/utils"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TextResult struct {
	Text      string `json:"text"`
	Timestamp struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"timestamp"`
}

func extractFrames(localFilePath, framesFolder, filename string) ([]string, error) {
	os.RemoveAll(framesFolder)
	err := os.Mkdir(framesFolder, 0755)
	if err != nil {
		fmt.Printf("Error creating frames folder: %s\n", err)
		return nil, err
	}

	filepath := localFilePath + "/" + filename

	cmd := exec.Command("ffmpeg", "-nostdin", "-i", filepath, "-loglevel", "error", "-threads", "3", "-vf", "fps=1", framesFolder+"/frame_sec_%d.jpg")

	// create just 5 frame
	// cmd := exec.Command("ffmpeg", "-i", filepath, "-vf", "fps=1", "-vframes", "5", framesFolder+"/frame_sec_%d.jpg")
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

	// return just first frame
	return fileNames, nil
}

func extractText(inputFilename, tmpFramerFolder, language string, results chan<- string) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo $(tesseract %s/\"%s\" stdout -l \"%s\")", tmpFramerFolder, inputFilename, language))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()

	if err != nil {
		results <- ""
	} else {
		output := strings.TrimSpace(stdout.String())
		secs, err := utils.ExtractSeconds(inputFilename)
		if err != nil {
			results <- ""
		} else {
			hhmmss := utils.SecondsToHHMMSS(secs)
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

var requestBody struct {
	FileUrl  string `json:"fileUrl"`
	Language string `json:"language"`
}

func Process(context *fiber.Ctx) error {
	start := time.Now()

	if err := context.BodyParser(&requestBody); err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": "fileUrl and language are required",
		})
	}

	uuid := uuid.New().String()

	err := os.Mkdir("/tmp/"+uuid, 0755)
	if err != nil {
		return context.Status(500).JSON(fiber.Map{
			"message": "Error creating tmp file folder:" + err.Error(),
		})
	}

	fmt.Printf("Start Processing video with file url %s\n", requestBody.FileUrl)
	localFilePath := "/tmp/" + uuid
	bucketName, bucketFilepath, bucketFilename, err := extractBucketNameAndObjectPath(requestBody.FileUrl)

	if err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	downloadFileStart := time.Now()
	_, err = storage.DownloadFile(
		bucketName,
		bucketFilepath,
		bucketFilename,
		localFilePath,
	)
	fmt.Printf("Download file took %v to run.\n", time.Since(downloadFileStart).Seconds())

	if err != nil {
		return context.Status(400).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	framesFolder := localFilePath + "/frames"

	fmt.Printf("Start extracting frames...\n")
	framesExtractStart := time.Now()
	frames, err := extractFrames(localFilePath, framesFolder, bucketFilename)
	fmt.Printf("Extract frames took %v to run.\n", time.Since(framesExtractStart).Seconds())

	if err != nil {
		fmt.Printf("Error extracting frames: %s\n", err)
		return context.Status(500).JSON(fiber.Map{
			"message": "error extracting frames",
		})
	}

	maxWorkers := runtime.GOMAXPROCS(0)

	fmt.Printf("Max workers: %d\n", maxWorkers)

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

	removeDuplicateStartTime := time.Now()
	sortedResults := utils.SortByTimestampAndRemoveDuplicates(textResults)
	fmt.Printf("SortByTimestampAndRemoveDuplicates took %v to run.\n", time.Since(removeDuplicateStartTime).Seconds())

	removeFolderStartTime := time.Now()
	os.RemoveAll(localFilePath)
	fmt.Printf("Removed tmp folder took %v to run.\n", time.Since(removeFolderStartTime).Seconds())

	fmt.Printf("The Entire processing took %v to run.\n", time.Since(start).Seconds())

	fmt.Printf("Finish processing video...\n")

	var textResultsArray []TextResult
	transformResultStartTime := time.Now()
	for timestamp, text := range sortedResults {
		textResultsArray = append(textResultsArray, TextResult{
			Text: text,
			Timestamp: struct {
				Start string `json:"start"`
				End   string `json:"end"`
			}{
				Start: timestamp,
				End:   timestamp,
			},
		},
		)

	}
	fmt.Printf("Transform result took %v to run.\n", time.Since(transformResultStartTime).Seconds())

	return context.Status(200).JSON(fiber.Map{
		"textResults":     textResultsArray,
		"secondsToFinish": time.Since(start).Seconds(),
	})
}

func worker(framesCh <-chan string, framesFolder, language string, resultsCh chan<- string) {
	for frame := range framesCh {
		extractText(frame, framesFolder, language, resultsCh)
	}
}
