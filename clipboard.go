package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ClipboardContent represents data held in clipboard
type ClipboardContent struct {
	Type        string // "text", "image", or "file"
	Data        []byte // Text content or file path
	ContentType string // MIME type for files
}

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CopyToClipboard copies data to the system clipboard
func CopyToClipboard(data string, ext string) error {
	if runtime.GOOS == "linux" {
		exists := FileExists(data)
		isWayland := os.Getenv("WAYLAND_DISPLAY") != ""

		if exists && ext != "" {
			// Copy image file to clipboard
			imageType := strings.Replace(ext, ".", "", 1)

			var cmd *exec.Cmd
			if isWayland {
				cmd = exec.Command("wl-copy", "--type", "image/"+imageType)
			} else {
				cmd = exec.Command("xclip", "-selection", "clipboard", "-t", "image/"+imageType, "-i")
			}

			fileContent, err := os.ReadFile(data)
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			stdin, err := cmd.StdinPipe()
			if err != nil {
				return fmt.Errorf("error getting stdin pipe: %w", err)
			}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("error starting command: %w", err)
			}

			if _, err := stdin.Write(fileContent); err != nil {
				return fmt.Errorf("error writing to stdin: %w", err)
			}

			stdin.Close()

			if err := cmd.Wait(); err != nil {
				return fmt.Errorf("command failed: %w", err)
			}

			fmt.Println("Image file copied to clipboard.")
		} else {
			// Handle text copying
			var cmd *exec.Cmd
			if isWayland {
				cmd = exec.Command("wl-copy")
			} else {
				cmd = exec.Command("xclip", "-selection", "clipboard")
			}

			stdin, err := cmd.StdinPipe()
			if err != nil {
				return fmt.Errorf("error getting stdin pipe: %w", err)
			}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("error starting command: %w", err)
			}

			if _, err := io.WriteString(stdin, data); err != nil {
				return fmt.Errorf("error writing to stdin: %w", err)
			}

			stdin.Close()

			if err := cmd.Wait(); err != nil {
				return fmt.Errorf("command failed: %w", err)
			}

			fmt.Println("Text copied to clipboard.")
		}
	} else {
		fmt.Println("Clipboard text copying not supported on this OS.")
	}

	return nil
}

// ImageTypes contains supported image MIME types
var ImageTypes = []string{
	"image/png",
	"image/jpeg",
	"image/jpg",
	"image/gif",
	"image/bmp",
	"image/tiff",
	"image/webp",
	"image/svg+xml",
}

// GetWaylandClipboardContent retrieves content from Wayland clipboard
func GetWaylandClipboardContent() (*ClipboardContent, error) {
	// Try to get available MIME types from clipboard
	var availableMimeTypes []string

	mimeCmd := exec.Command("wl-paste", "--list-types")
	mimeOutput, err := mimeCmd.Output()
	if err == nil {
		types := strings.Split(strings.TrimSpace(string(mimeOutput)), "\n")
		for _, t := range types {
			if t != "" {
				availableMimeTypes = append(availableMimeTypes, t)
			}
		}
	}

	// Filter for image types if available, otherwise use predefined list
	var imageTypes []string
	if len(availableMimeTypes) > 0 {
		for _, t := range availableMimeTypes {
			if strings.HasPrefix(t, "image/") {
				imageTypes = append(imageTypes, t)
			}
		}
	} else {
		imageTypes = ImageTypes
	}

	// Try to get image from clipboard in any of the available formats
	for _, imageType := range imageTypes {
		extension := strings.Replace(strings.Split(imageType, "/")[1], "+xml", "", 1)
		if extension == "" {
			extension = "png"
		}

		cmd := exec.Command("wl-paste", "--type", imageType)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			return &ClipboardContent{
				Type:        "image",
				Data:        output,
				ContentType: extension,
			}, nil
		}
	}

	// Try to get text from clipboard
	textCmd := exec.Command("wl-paste", "--type", "text/plain")
	textOutput, err := textCmd.Output()
	if err == nil && len(textOutput) > 0 {
		text := strings.TrimSpace(string(textOutput))

		// Check if text is a file path that exists
		if text != "" && FileExists(text) {
			return &ClipboardContent{
				Type:        "file",
				Data:        []byte(text),
				ContentType: "txt",
			}, nil
		} else {
			return &ClipboardContent{
				Type:        "text",
				Data:        []byte(text),
				ContentType: "txt",
			}, nil
		}
	}

	return nil, nil
}

// GetX11ClipboardContent retrieves content from X11 clipboard
func GetX11ClipboardContent() (*ClipboardContent, error) {
	// Try to get available MIME types from clipboard
	var availableMimeTypes []string

	mimeCmd := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o")
	mimeOutput, err := mimeCmd.Output()
	if err == nil {
		lines := strings.Split(string(mimeOutput), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "image/") || line == "text/plain" {
				availableMimeTypes = append(availableMimeTypes, line)
			}
		}
	}

	// Filter for image types if available, otherwise use predefined list
	var imageTypes []string
	if len(availableMimeTypes) > 0 {
		for _, t := range availableMimeTypes {
			if strings.HasPrefix(t, "image/") {
				imageTypes = append(imageTypes, t)
			}
		}
	} else {
		imageTypes = ImageTypes
	}

	// Try to get image from clipboard in any of the available formats
	for _, imageType := range imageTypes {
		extension := strings.Replace(strings.Split(imageType, "/")[1], "+xml", "", 1)
		if extension == "" {
			extension = "png"
		}

		imageCmd := exec.Command("xclip", "-selection", "clipboard", "-t", imageType, "-o")
		output, err := imageCmd.Output()
		if err == nil && len(output) > 0 {
			return &ClipboardContent{
				Type:        "image",
				Data:        output,
				ContentType: extension,
			}, nil
		}
	}

	// Try to get text from clipboard
	textCmd := exec.Command("xclip", "-selection", "clipboard", "-t", "text/plain", "-o")
	textOutput, err := textCmd.Output()
	if err == nil && len(textOutput) > 0 {
		text := strings.TrimSpace(string(textOutput))

		// Check if text is a file path that exists
		if text != "" && FileExists(text) {
			return &ClipboardContent{
				Type:        "file",
				Data:        []byte(text),
				ContentType: "txt",
			}, nil
		} else {
			return &ClipboardContent{
				Type:        "text",
				Data:        []byte(text),
				ContentType: "txt",
			}, nil
		}
	}

	return nil, nil
}

// GetClipboardContent retrieves content from clipboard regardless of display server
func GetClipboardContent() (*ClipboardContent, error) {
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")

	if waylandDisplay != "" {
		return GetWaylandClipboardContent()
	} else {
		return GetX11ClipboardContent()
	}
}
