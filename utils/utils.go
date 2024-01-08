package utils

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

func FileExists(filename string) bool {
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

func ExtractTimestamp(s string) string {
	// Regular expression to match the timestamp inside << >>
	re := regexp.MustCompile(`(\d{2}:\d{2}:\d{2})`)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func SecondsToHHMMSS(seconds int) string {
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	seconds %= 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func SortByTimestamp(arr []string) []string {
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
			timestamp:      ExtractTimestamp(s),
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

func ExtractSeconds(filename string) (int, error) {
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

func StringExistsInArray(target string, arr []string) bool {
	for _, str := range arr {
		if str == target {
			return true
		}
	}
	return false
}

func NormalizeText(inputText string) string {
	// Replace line breaks and tabs with empty strings
	normalizedText := strings.ReplaceAll(inputText, "\n", " ")
	normalizedText = strings.ReplaceAll(normalizedText, "\t", "")
	// Trim leading and trailing whitespaces
	normalizedText = strings.TrimSpace(normalizedText)

	return normalizedText
}

func SortByTimestampAndRemoveDuplicates(arr []string) map[string]string {
	sortedArr := SortByTimestamp(arr)

	// Filter out empty strings with or without timestamps and those with timestamps but no text
	filteredArr := make([]string, 0)
	for idx, s := range sortedArr {
		timestamp := ExtractTimestamp(s)

		if len(s) < 9 {
			fmt.Printf("String \"%s\" is too short\n", s)
			continue
		}

		normalized := NormalizeText(s[8:])

		// Check if the string is empty or has a valid timestamp but no accompanying text
		if normalized != "" && timestamp != "" {
			nextText := sortedArr[idx+1]

			// Check if the next string is the last in the array
			// In that case compare the current string with the previous string
			if idx == len(sortedArr)-1 {
				nextText = sortedArr[idx-2]
			}

			nextNormalizedText := NormalizeText(nextText[8:])

			textIsSimilar := areStringsSimilar(normalized, nextNormalizedText, 10)
			if !strings.HasPrefix(normalized, nextNormalizedText) || !StringExistsInArray(normalized, filteredArr) || !textIsSimilar {
				filteredArr = append(filteredArr, normalized+" "+timestamp)
			}
		}
	}

	return MapTextsByTimestamp(filteredArr)
}

func MapTextsByTimestamp(texts []string) map[string]string {
	textsMap := make(map[string]string)
	for _, text := range texts {
		timestamp := text[len(text)-8:]
		if timestamp != "" {
			textsMap[timestamp] = strings.TrimSpace(text[:len(text)-8])
		}
	}
	return textsMap
}

func areStringsSimilar(string1, string2 string, threshold int) bool {
	distance := levenshtein.DistanceForStrings([]rune(string1), []rune(string2), levenshtein.DefaultOptions)
	return distance <= threshold
}
