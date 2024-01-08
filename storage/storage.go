package storage

import (
	"context"
	"encoding/base64"
	"extract/utils"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func DownloadFile(bucketName, bucketFilepath, bucketFilename, localFilePath string) (string, error) {

	localFilename := localFilePath + "/" + bucketFilename
	if utils.FileExists(localFilename) {
		fmt.Printf("File already exists in local: %s\n", localFilePath)
		return localFilePath, nil
	}

	// Create a context and a storage client using your Google Cloud credentials.
	ctx := context.Background()

	// decode base64 credentials
	decodedCredentials, err := base64.StdEncoding.DecodeString(os.Getenv("GCP_CREDS_JSON_BASE64"))

	if err != nil {
		fmt.Printf("Error decoding credentials: %v\n", err)
		return "", fmt.Errorf("error decoding credentials: %v", err)
	}

	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(decodedCredentials))

	if err != nil {
		fmt.Printf("Error creating GCS client: %v\n", err)
		return "", fmt.Errorf("error creating GCS client: %v", err)
	}
	defer client.Close()

	// Open the GCS bucket and the object (file) you want to download.
	bucket := client.Bucket(bucketName)
	object := bucket.Object(bucketFilepath)

	// Create a new local file to save the downloaded data.
	localFile, err := os.Create(localFilename)
	if err != nil {
		fmt.Printf("Error creating local file: %v\n", err)
		return "", fmt.Errorf("error creating local file: %v", err)
	}
	defer localFile.Close()

	fmt.Printf("Downloading file: %s\n", bucketFilename)
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
