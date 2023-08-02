#!/bin/bash

function get_timestamps() {
  local input_file=$1
  local time_list=()
  local seconds=1

  local duration=$(ffmpeg -i "$input_file" 2>&1 | grep "Duration" | awk '{print $2}' | tr -d ,)
  local total_seconds=$(date -u -d "1970-01-01 $duration" +"%s")

  while [[ "$seconds" -le "$total_seconds" ]]; do
    local hours=$((seconds / 3600))
    local minutes=$(((seconds % 3600) / 60))
    local remaining_seconds=$((seconds % 60))

    # Pad single-digit hours, minutes, and seconds with leading zeros
    local formatted_time=$(printf "%02d:%02d:%02d" "$hours" "$minutes" "$remaining_seconds")

    time_list+=("$formatted_time")
    ((seconds++))
  done

  echo "${time_list[@]}"
}

function extract_text() {
    local input_file=$1
    local timestamp=$2
    local language=$3
    local output_text=$(ffmpeg -ss "$timestamp" -i "$input_file" -f image2pipe -frames:v 1 -q:v 2 - | tesseract stdin stdout -l "eng")
    # local output_text=$(ffmpeg -ss "$timestamp" -i "$input_file" -f image2pipe -frames:v 1 -q:v 2 - | tesseract stdin stdout -l $language)
    echo "$timestamp $output_text"
}
