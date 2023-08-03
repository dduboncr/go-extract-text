# Go Extract Text

Extracts visual texts from videos

## Requirements

* Docker

## How to run

* You need to have a credentials.json file with your gcloud bucket service account credentials in the root directory of this project
* You need to setup a .env file with API_KEY property

Go to the project root directory and run:

1. `docker build -t go-extract .`

2. `docker run --name go-extract --rm -it -v .:/go/src -p 3000:3000 go-extract`

3. ```
    curl http://127.0.0.1:3000/process \
        -H "Content-Type: application/json" \
        -H "x-api-key": "<API_KEY>" \
        -d '{ "fileUrl": "<BUCKET_FILE_URL>" }'

