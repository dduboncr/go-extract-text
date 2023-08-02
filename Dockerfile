# Use the official Node.js image as the base image
FROM node:18

# Install ffmpeg
RUN apt-get update && \
    apt-get install -y ffmpeg && \
    rm -rf /var/lib/apt/lists/*

# Install gsutil
RUN apt-get update && \
    apt-get install -y gnupg2 ca-certificates lsb-release && \
    export CLOUD_SDK_REPO="cloud-sdk-$(lsb_release -c -s)" && \
    echo "deb https://packages.cloud.google.com/apt $CLOUD_SDK_REPO main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
    apt-get update && \
    apt-get install -y google-cloud-sdk && \
    rm -rf /var/lib/apt/lists/*

# Install Tesseract and required dependencies
RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    tesseract-ocr-eng \
    # Add other language packs as needed (e.g., tesseract-ocr-fra, tesseract-ocr-spa, etc.)
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# # Authenticate with Google Cloud
# ARG GOOGLE_STORAGE_CLIENT_EMAIL
# ARG GOOGLE_PRIVATE_KEY

# ENV GOOGLE_STORAGE_CLIENT_EMAIL_ENV=$GOOGLE_STORAGE_CLIENT_EMAIL
# ENV GOOGLE_PRIVATE_KEY_ENV=$GOOGLE_PRIVATE_KEY

# RUN mkdir /app/.keys && \
#     echo "$GOOGLE_PRIVATE_KEY_ENV" > /app/.keys/gcp-bucket.pem && \
#     gcloud iam service-accounts keys create /app/.keys/gcp-bucket-keys.json \
#       --iam-account="$GOOGLE_STORAGE_CLIENT_EMAIL_ENV" \
#       --key-file-type=json \
#       --private-key-file=/app/.keys/gcp-bucket.pem && \
#     # Authenticate gsutil with the service account key
#     gcloud auth activate-service-account --key-file=/app/.keys/gcp-bucket-keys.json && \
#     rm -rf /app/.keys

CMD [ "tail", "-f", "/dev/null" ]

# # Copy the package.json and package-lock.json (if available) to the container
# COPY package*.json ./

# RUN npm i

# COPY . .

# EXPOSE 3434

# CMD ["npm", "start"]
