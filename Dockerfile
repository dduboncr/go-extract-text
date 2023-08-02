# Use the official Node.js image as the base image
FROM golang:1.20-alpine

# Install Bash
RUN apk add --no-cache bash

# Install ffmpeg
RUN apk update && \
    apk add -q ffmpeg && \
    rm -rf /var/lib/apt/lists/*

# Set environment variables for Tesseract installation
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr

# Install Tesseract and its dependencies
RUN apk --no-cache add \
    tesseract-ocr \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-deu \
    # Add other language data packages as needed
    && mkdir -p $TESSDATA_PREFIX \
    && chmod 755 $TESSDATA_PREFIX

# Your Go application setup here (copy your Go code and build it, if needed)
# For example:

WORKDIR /app
COPY . .
RUN go build -o go-video-extractor

# Start your Go application (replace "my_app" with your actual binary name)
# For example:
# CMD ["./my_app"]

EXPOSE 3000

CMD [ "tail", "-f", "/dev/null" ]

