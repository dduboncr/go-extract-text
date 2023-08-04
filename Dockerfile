FROM golang:1.19.7-alpine3.17 AS builder

WORKDIR /app
COPY . .
RUN go build -o go-video-extractor

FROM golang:1.20-alpine AS final

RUN apk add --no-cache bash

# Install ffmpeg
RUN apk update && \
    apk add -q ffmpeg && \
    rm -rf /var/lib/apt/lists/*

# Set environment variables for Tesseract installation
ENV TESSDATA_PREFIX=/usr/share/tessdata

# Install Tesseract and its dependencies
RUN apk --no-cache add \
    tesseract-ocr \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-deu \
    # Add other language data packages as needed
    && mkdir -p $TESSDATA_PREFIX \
    && chmod 755 $TESSDATA_PREFIX

WORKDIR /app

COPY --from=builder /app/go-video-extractor .
COPY --from=builder /app/credentials.json .
COPY --from=builder /app/bin ./bin

EXPOSE 3000

CMD [ "./go-video-extractor" ]

