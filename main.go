package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func downloadStream(m3u8URL, outputDir, name string) error {
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s.mp4", name))

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
		err := downloadStream(video.Location, outputDir, video.Name)
		if err != nil {
			fmt.Printf("Error downloading %s: %v\n", video.Name, err)
		}
	}
}
