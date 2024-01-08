# Go Extract Text

Extracts visual texts from videos

## Requirements

- Docker

## Setup

- Create a .env by copying the .env-example and replace the variables with your own keys

## How to run

- store gcp credentials in base64

```bash
cat credentials.json | base64
```

Go to the project root directory and run:

1. `docker build -t go-extract .`

2. `docker run --name go-extract --rm -it -v .:/go/src -p 3000:3000 go-extract`

3. ```bash
   curl http://127.0.0.1:3000/process \
       -H "Content-Type: application/json" \
       -H "x-api-key": "<API_KEY>" \
       -d '{ "fileUrl": "gs://<BUCKET_NAME>/<FILE_PATH>", "language":"eng" }'
   ```
