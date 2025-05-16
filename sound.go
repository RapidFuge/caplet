package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed sounds/*.wav
var soundFiles embed.FS

// Sound types
const (
	Captured = "sounds/CaptureSound.wav"       // Success/completion sound
	Uploaded = "sounds/TaskCompletedSound.wav" // Secondary success sound
	Error    = "sounds/ErrorSound.wav"         // Error notification sound
)

var (
	playMutex     sync.Mutex
	tempDir       string
	extractedOnce sync.Once
	extractErr    error
)

// extractSoundFiles extracts embedded WAV files to a temporary directory
// This only needs to be done once per program execution
func extractSoundFiles() error {
	var err error
	extractedOnce.Do(func() {
		// Create temporary directory
		tempDir, err = os.MkdirTemp("", "sounds")
		if err != nil {
			extractErr = fmt.Errorf("failed to create temp directory: %w", err)
			return
		}

		// Extract sound files
		files := []string{Captured, Uploaded, Error}
		for _, file := range files {
			// Read embedded file
			data, err := fs.ReadFile(soundFiles, file)
			if err != nil {
				extractErr = fmt.Errorf("embedded sound file not found: %w", err)
				return
			}

			// Create destination path
			destPath := filepath.Join(tempDir, filepath.Base(file))

			// Write to temp file
			err = os.WriteFile(destPath, data, 0644)
			if err != nil {
				extractErr = fmt.Errorf("failed to extract sound file: %w", err)
				return
			}
		}
	})
	return extractErr
}

// findAudioPlayer attempts to locate an available audio player command
func findAudioPlayer() (string, []string, error) {
	// Ordered list of possible audio players, with their arguments
	players := []struct {
		cmd  string
		args []string
	}{
		{"aplay", []string{"-q"}},                                          // ALSA audio player
		{"paplay", []string{}},                                             // PulseAudio player
		{"ogg123", []string{"-q"}},                                         // Ogg Vorbis player (can play WAV too)
		{"mplayer", []string{"-really-quiet"}},                             // MPlayer
		{"mpg123", []string{"-q"}},                                         // MPG123 (can play WAV too)
		{"cvlc", []string{"--play-and-exit", "--quiet"}},                   // VLC command line
		{"ffplay", []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}}, // FFmpeg player
	}

	for _, player := range players {
		path, err := exec.LookPath(player.cmd)
		if err == nil {
			return path, player.args, nil
		}
	}

	return "", nil, fmt.Errorf("no suitable audio player found on this system")
}

// PlaySound plays a specified sound using available system audio tools
func PlaySound(soundFile string) error {
	playMutex.Lock()
	defer playMutex.Unlock()

	// Extract sound files if needed
	if err := extractSoundFiles(); err != nil {
		return err
	}

	// Get the path to the extracted sound file
	soundPath := filepath.Join(tempDir, filepath.Base(soundFile))

	// Find an available audio player
	playerCmd, playerArgs, err := findAudioPlayer()
	if err != nil {
		return fmt.Errorf("no audio player available: %w", err)
	}

	// Create the command with all arguments
	args := append(playerArgs, soundPath)
	cmd := exec.Command(playerCmd, args...)

	// Execute the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error playing sound %s with %s: %w", soundFile, playerCmd, err)
	}

	return nil
}

// PlayCaptured plays the CaptureSound sound.
// Use this when a screenshotting the screen is successful
func PlayCaptured() error {
	return PlaySound(Captured)
}

// PlayUploaded plays the TaskCompletedSound sound
// Use this when you successfully uploaded the file/image.
func PlayUploaded() error {
	return PlaySound(Uploaded)
}

// PlayError plays the ErrorSound sound.
// Use this when a task has failed or an error has occurred.
func PlayError() error {
	return PlaySound(Error)
}

// Cleanup removes the temporary directory and extracted files
// Call this when your application shuts down
func Cleanup() {
	if tempDir != "" {
		os.RemoveAll(tempDir)
	}
}
