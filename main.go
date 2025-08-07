package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
)

type VideoDef struct {
	Name           string `json:"name"`
	Duration       int    `json:"duration"`
	Location       string `json:"location"`
	SubtitleBase   string `json:"subtitle_base"`
	HasTranslation bool   `json:"has_translations"`
}

type VideoData struct {
	VideoDefs []VideoDef `json:"video_defs"`
}

func getVideoDuration(filePath string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	
	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}
	
	return duration, nil
}

func fileExistsWithValidDuration(filePath string, expectedDuration int) bool {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	
	// Get actual duration
	actualDuration, err := getVideoDuration(filePath)
	if err != nil {
		fmt.Printf("Warning: Could not get duration for %s, will re-download: %v\n", filePath, err)
		return false
	}
	
	// Compare with tolerance of 5 seconds
	tolerance := 5.0
	expectedFloat := float64(expectedDuration)
	
	if math.Abs(actualDuration-expectedFloat) <= tolerance {
		return true
	}
	
	fmt.Printf("Duration mismatch for %s: expected %.1fs, got %.1fs (tolerance: %.1fs)\n",
		filePath, expectedFloat, actualDuration, tolerance)
	return false
}

func downloadStream(m3u8URL, outputDir, name string, expectedDuration int) error {
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s.mp4", name))
	
	// Check if file already exists with valid duration
	if fileExistsWithValidDuration(outputFile, expectedDuration) {
		fmt.Printf("File %s already exists with correct duration, skipping...\n", name)
		return nil
	}

	// Initialize a cyclic progress bar
	bar := pb.New(100) // The total here doesn't matter; it will cycle
	bar.SetTemplateString(`{{cycle . "▌" "▀" "▐" "▄"}} {{string . "suffix"}}`)
	bar.Set("suffix", fmt.Sprintf(" Downloading %s", name))
	bar.Start()

	cmd := exec.Command(
		"ffmpeg",
		"-y",          // Overwrite output files without asking
		"-i", m3u8URL, // Input file URL
		"-c", "copy", // Copy streams without reencoding
		"-bsf:a", "aac_adtstoasc", // Bitstream filter for audio; necessary for MP4 files
		"-progress", "pipe:1", // Output progress to stdout
		"-nostats",                // Do not print encoding progress to stderr
		"-movflags", "+faststart", // Start playback before file is completely downloaded
		outputFile, // Output file path
	)

	if err := cmd.Start(); err != nil {
		return err
	}

	// Start a goroutine to keep the bar cycling
	go func() {
		for {
			bar.Increment()
			time.Sleep(100 * time.Millisecond) // Update every 100ms
		}
	}()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		return err
	}

	bar.Finish()
	fmt.Printf("Saved to %s\n", outputFile)
	return nil
}

func main() {
	// Load JSON data from the provided file
	file, err := os.Open("video_defs.json")
	if err != nil {
		fmt.Println("Error opening JSON file:", err)
		return
	}
	defer file.Close()

	var data VideoData
	err = json.NewDecoder(file).Decode(&data)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// Create output directory if it doesn't exist
	outputDir := "downloaded_videos"
	err = os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating output directory:", err)
		return
	}

	for _, video := range data.VideoDefs {
		err := downloadStream(video.Location, outputDir, video.Name, video.Duration)
		if err != nil {
			fmt.Printf("Error downloading %s: %v\n", video.Name, err)
		}
	}
}
