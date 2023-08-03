#!/bin/bash

function get_timestamps() {
  local input_file=$1
  local timestamps_list=()
  local duration=$(ffmpeg -i "$input_file" 2>&1 | grep "Duration" | awk '{print $2}' | tr -d ,)
  
  # Parse the input video length string to extract hours, minutes, and seconds
  local hours=$(echo "$duration" | cut -d: -f1)
  local minutes=$(echo "$duration" | cut -d: -f2)
  local seconds=$(echo "$duration" | cut -d: -f3 | cut -d. -f1)

  # Convert the hours, minutes, and seconds to total seconds
  local total_seconds=$((hours * 3600 + minutes * 60 + seconds))

  # Generate the list of strings representing each second
  for ((i = 1; i <= total_seconds; i++)); do
    local hours=$(printf "%02d" $((i / 3600)))
    local minutes=$(printf "%02d" $(((i / 60) % 60)))
    local seconds=$(printf "%02d" $((i % 60)))
    timestamps_list+=("$hours:$minutes:$seconds")
  done

  # Return the seconds list as a string array
  echo "${timestamps_list[@]}"
}

function extract_text() {
  local input_file=$1
  local timestamp=$2
  local language=$3
  local output_text=$(ffmpeg -ss "$timestamp" -i "$input_file" -f image2pipe -frames:v 1 -q:v 2 - | tesseract stdin stdout -l "eng")
  # local output_text=$(ffmpeg -ss "$timestamp" -i "$input_file" -f image2pipe -frames:v 1 -q:v 2 - | tesseract stdin stdout -l $language)
  echo "$timestamp $output_text"
}
