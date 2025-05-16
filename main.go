package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Upload represents an entry in the upload history
type Upload struct {
	URL       string `json:"url"`
	File      string `json:"file"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
}

var NOTIFY_ID string

var ImageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".webp": true,
	".svg":  true,
}

func ShortenURL(inputURL string, service SiteConfig, showNotification bool, historyPath string) (string, error) {
	fmt.Printf("Using %s to shorten URL\n", service.Name)

	// Prepare arguments with $input$ replaced
	args := url.Values{}
	for key, value := range service.Arguments {
		args.Set(key, strings.ReplaceAll(value, "$input$", inputURL))
	}

	// Prepare headers with $input$ replaced
	reqHeaders := make(http.Header)
	for key, value := range service.Headers {
		reqHeaders.Set(key, strings.ReplaceAll(value, "$input$", inputURL))
	}

	var req *http.Request
	var err error

	// Create request depending on method
	if service.RequestType == "POST" {
		req, err = http.NewRequest("POST", service.RequestURL, strings.NewReader(args.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		fullURL := service.RequestURL + "?" + args.Encode()
		req, err = http.NewRequest("GET", fullURL, nil)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, values := range reqHeaders {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	if showNotification {
		NOTIFY_ID, err = Notify("Shortening URL...", NOTIFY_ID, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to show notification: %v\n", err)
		}
	}

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if showNotification {
			Notify(fmt.Sprintf("Shorten failed: %v", err), NOTIFY_ID, "")
		}
		fmt.Printf("request failed: %s", err)

		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if showNotification {
			Notify(fmt.Sprintf("Shorten failed: %s", resp.Status), NOTIFY_ID, "")
		}
		return "", fmt.Errorf("shorten failed with status: %s", resp.Status)
	}

	// Parse response
	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	responseText := string(responseBytes)

	re := regexp.MustCompile(service.Regexps["url"])
	matches := re.FindStringSubmatch(responseText)

	if len(matches) < 2 {
		if showNotification {
			Notify("Could not extract shortened URL", NOTIFY_ID, "")
		}
		return "", fmt.Errorf("could not extract shortened URL")
	}

	shortURL := regexp.MustCompile(`\\(.)`).ReplaceAllString(matches[1], "$1")

	// Save to history
	err = SaveToHistory(historyPath, Upload{
		URL:       shortURL,
		File:      inputURL,
		Timestamp: time.Now().Format(time.RFC3339),
		Service:   service.Name,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save to history: %v\n", err)
	}

	return shortURL, nil
}

// UploadFile uploads a file to the specified service
func UploadFile(filePath string, service SiteConfig, showNotification bool, historyPath string, savePath string, organized bool) (string, error) {
	fmt.Printf("Uploading to %s...\n", service.Name)
	// fmt.Println(filePath)

	savePath = strings.ReplaceAll(savePath, "$HOME", os.Getenv("HOME"))

	// If organized is true, append a year-month subdirectory like "2025-05"
	if organized {
		now := time.Now()
		subDir := now.Format("2006-01")
		savePath = filepath.Join(savePath, subDir)
	}

	// Ensure the savePath directory exists
	err := os.MkdirAll(savePath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create savePath directory: %w", err)
	}

	// Clone file to savePath
	srcFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Extract the file name
	fileName := filepath.Base(filePath)
	dstFilePath := filepath.Join(savePath, fileName)

	dstFile, err := os.Create(dstFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// Create multipart form data
	body, contentType, err := createMultipartForm(filePath, service)
	if err != nil {
		return "", fmt.Errorf("failed to create form data: %w", err)
	}

	// Create request with headers
	req, err := http.NewRequest(service.RequestType, service.RequestURL, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	for key, value := range service.Headers {
		req.Header.Set(key, value)
	}

	if showNotification {
		var err error
		NOTIFY_ID, err = Notify("Uploading to host...", NOTIFY_ID, dstFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to show notification: %v\n", err)
		}
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if showNotification {
			Notify(fmt.Sprintf("Upload failed: %v", err), NOTIFY_ID, "")
		}
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if showNotification {
			Notify(fmt.Sprintf("Upload failed: %s", resp.Status), NOTIFY_ID, "")
		}
		return "", fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	// Process response according to type
	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	responseText := string(responseBytes)

	// Extract URL using the regexp
	re := regexp.MustCompile(service.Regexps["url"])
	matches := re.FindStringSubmatch(responseText)

	if len(matches) < 2 {
		if showNotification {
			Notify("Could not extract URL from response", NOTIFY_ID, "")
		}
		return "", fmt.Errorf("could not extract URL from response")
	}

	url := matches[1]
	// Clean up escaped characters
	url = regexp.MustCompile(`\\(.)`).ReplaceAllString(url, "$1")

	// Save to upload history
	err = SaveToHistory(historyPath, Upload{
		URL:       url,
		File:      dstFilePath,
		Timestamp: time.Now().Format(time.RFC3339),
		Service:   service.Name,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save to history: %v\n", err)
	}

	return url, nil
}

// createMultipartForm creates a multipart form for file upload
func createMultipartForm(filePath string, service SiteConfig) (io.Reader, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file to form
	fileField := service.FileFormName
	if fileField == "" {
		fileField = "file" // Default form field name if not specified
	}

	part, err := writer.CreateFormFile(fileField, filepath.Base(filePath))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to copy file content: %w", err)
	}

	// Add any additional arguments
	for key, value := range service.Arguments {
		err = writer.WriteField(key, value)
		if err != nil {
			return nil, "", fmt.Errorf("failed to write form field: %w", err)
		}
	}

	err = writer.Close()
	if err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}

// SaveToHistory saves upload information to history file
func SaveToHistory(historyPath string, upload Upload) error {
	historyPath = strings.ReplaceAll(historyPath, "$HOME", os.Getenv("HOME"))

	if err := os.MkdirAll(historyPath, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	historyFile := filepath.Join(historyPath, "history.json")

	var history []Upload

	// Read existing history if file exists
	if data, err := os.ReadFile(historyFile); err == nil {
		if err := json.Unmarshal(data, &history); err != nil {
			// If file exists but is corrupt, start with empty history
			history = []Upload{}
		}
	}

	// Add new upload to history
	history = append(history, upload)

	// Write updated history
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(historyFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// Notify shows a desktop notification and returns the notification ID
func Notify(message string, id string, icon string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", nil // Only supported on Linux
	}

	args := []string{"-p", "Caplet", message}

	if id != "" {
		args = append([]string{"-r", id}, args...)
	}

	if icon != "" {
		args = append([]string{"-i", icon}, args...)
	}

	cmd := exec.Command("notify-send", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("notification failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func main() {
	// Define command-line flags
	var filePath string
	var inputURL string
	var url string
	var err error

	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	helpFlag := flag.Bool("help", false, "Help command")
	modeFlag := flag.String("mode", "", "Set the mode.\nf/file: Upload a file.\nfs/fullscreen: Screenshoot entire screen\ns/select: Select screen region\nc/clipboard: Upload clipboard contents\nu/url: Shorten url")
	sxcuFlag := flag.String("sxcu", "", "Path to the .sxcu config file")
	notifyFlag := flag.Bool("notify", true, "Show desktop notifications")
	clipFlag := flag.Bool("clip", true, "Copy resulting URL to clipboard.")
	historyPath := flag.String("history", config.HistoryPath, "Folder path to upload history")
	savePath := flag.String("save", config.SaveDir, "Folder path to upload screenshots/files")
	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	if *sxcuFlag != "" {
		// Load the service config from the .sxcu file
		err = ImportSXCU(*sxcuFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to import SXCU to config: %v\n", err)
			os.Exit(1)
		}

		os.Exit(0)
	}

	switch *modeFlag {
	case "s", "select":
		filePath, err = TakeScreenshot(true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to take screenshot: %v\n", err)
			os.Exit(1)
		}

		exists := FileExists(filePath)
		if !exists {
			fmt.Println("Screenshot operation cancelled by user.")
			os.Exit(0)
		}
		go PlayCaptured()

	case "fs", "fullscreen":
		filePath, err = TakeScreenshot(false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to take screenshot: %v\n", err)
			os.Exit(1)
		}

		exists := FileExists(filePath)
		if !exists {
			fmt.Println("Screenshot operation cancelled by user.")
			os.Exit(0)
		}
		go PlayCaptured()

	case "f", "file":
		if len(flag.Args()) < 1 {
			fmt.Fprintf(os.Stderr, "no file provided!")
			os.Exit(1)
		}
		filePath = flag.Args()[0]

	case "u", "url":
		if len(flag.Args()) < 1 {
			fmt.Fprintf(os.Stderr, "no url provided!")
			os.Exit(1)
		}
		inputURL = flag.Args()[0]

	case "c", "clipboard":
		clipboardContent, err := GetClipboardContent()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get clipboard contents: %v\n", err)
			os.Exit(1)
		}
		tempDir, err := os.MkdirTemp("", "caplet-")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create temp directory: %v\n", err)
			os.Exit(1)
		}
		filePath = filepath.Join(tempDir, fmt.Sprintf("paste-%s.%s", time.Now().Format("2006-01-02_15-04-05"), clipboardContent.ContentType))
		err = os.WriteFile(filePath, []byte(clipboardContent.Data), 0644)
		if err != nil {
			fmt.Println("Failed to write file:", err)
			os.Exit(1)
		}

	default:
		flag.Usage()
		os.Exit(0)
	}

	if filePath != "" {
		ext := filepath.Ext(filePath)
		isImage := ImageExtensions[ext]
		serviceName := ""

		if isImage {
			serviceName = config.DefaultImageUpload
			if *clipFlag {
				if errCp := CopyToClipboard(filePath, ext); errCp != nil { // Assumes CopyToClipboard handles image data with filePath and ext
					fmt.Fprintf(os.Stderr, "Failed to copy image to clipboard (as per -clip flag): %v\n", errCp)
					go PlayError()
					os.Exit(1) // Fatal error, as in original logic
				}
				// fmt.Println("Image copied to clipboard (due to -clip flag).") // Optional: confirmation message
			}
		} else {
			serviceName = config.DefaultFileUpload
		}

		// Check if a default uploader key is defined in the config
		if serviceName != "" {
			service, found := config.Uploaders[serviceName]
			if found { // Service is configured and exists
				// Proceed with upload
				fmt.Printf("Attempting to upload %s...\n", filePath)
				url, err = UploadFile(filePath, service, *notifyFlag, *historyPath, *savePath, config.Organized)
				if err != nil {
					go PlayError()
					fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
					os.Exit(1)
				}
				// url is now set, subsequent clipboard (for URL) and notification logic will handle it.
			} else {
				// No default service configured for this key, or key points to nil config
				fmt.Printf("No default upload service configured for '%s'.\n", serviceName)
				fmt.Println("Copying to clipboard instead of uploading.")
				if isImage {
					// If *clipFlag was false, the image wasn't copied yet.
					// The requirement is "only copy to clipboard" if no default service.
					if !(*clipFlag) { // Only copy if not already copied by the *clipFlag logic for images
						if errCp := CopyToClipboard(filePath, ext); errCp != nil {
							go PlayError()
							fmt.Fprintf(os.Stderr, "Failed to copy image to clipboard: %v\n", errCp)
							os.Exit(1)
						} else {
							fmt.Println("Image copied to clipboard.")
						}
					} else {
						fmt.Println("Image was already copied (due to -clip flag). Will not upload as no default service is configured.")
					}
				} else { // For non-image files, copy the file path as text
					if errCp := CopyToClipboard(filePath, "text"); errCp != nil { // Assuming "text" copies the string `filePath`
						go PlayError()
						fmt.Fprintf(os.Stderr, "Failed to copy file path to clipboard: %v\n", errCp)
						os.Exit(1)
					} else {
						fmt.Println("File path copied to clipboard.")
					}
				}
				// Successfully copied to clipboard, no upload happened. 'url' remains empty.
				os.Exit(0) // Operation complete
			}
		} else {
			// DefaultImageUpload or DefaultFileUpload key itself is not set in config.
			fmt.Println("No default upload service key (DefaultImageUpload/DefaultFileUpload) defined in config.")
			fmt.Println("Copying to clipboard instead of uploading.")
			if isImage {
				// Copy image data regardless of *clipFlag if here, as it's the only action.
				if errCp := CopyToClipboard(filePath, ext); errCp != nil {
					go PlayError()
					fmt.Fprintf(os.Stderr, "Failed to copy image to clipboard: %v\n", errCp)
					os.Exit(1)
				} else {
					fmt.Println("Image copied to clipboard.")
				}
			} else { // Non-image file, copy its path.
				if errCp := CopyToClipboard(filePath, "text"); errCp != nil {
					go PlayError()
					fmt.Fprintf(os.Stderr, "Failed to copy file path to clipboard: %v\n", errCp)
					os.Exit(1)
				} else {
					fmt.Println("File path copied to clipboard.")
				}
			}
			os.Exit(0) // Operation complete
		}
	} else if inputURL != "" {
		shortenerName := config.DefaultURLShortener
		if shortenerName != "" {
			service, found := config.Shorteners[shortenerName]
			if found { // Service is configured and exists
				fmt.Printf("Attempting to shorten URL %s...\n", inputURL)
				url, err = ShortenURL(inputURL, service, *notifyFlag, *historyPath)
				if err != nil {
					go PlayError()
					fmt.Fprintf(os.Stderr, "URL shortening failed: %v\n", err)
					os.Exit(1)
				}
				// url is now set
			} else {
				// Default URL shortener key is set, but not found in Shorteners map or is nil
				go PlayError()
				fmt.Fprintf(os.Stderr, "Error: Default URL shortener service ('%s') not found or not configured properly. Cannot shorten URL.\n", shortenerName)
				os.Exit(1)
			}
		} else {
			// DefaultURLShortener key itself is not set in config.
			go PlayError()
			fmt.Fprintf(os.Stderr, "Error: No default URL shortener key (DefaultURLShortener) defined in config. Cannot shorten URL.\n")
			os.Exit(1)
		}
	} else {
		// This case implies neither filePath nor inputURL was set.
		// This should typically be caught by mode-specific argument checks within the switch.
		fmt.Fprintf(os.Stderr, "No file path or input URL was available to process.\n")
		os.Exit(1)
	}

	// --- End of modified logic ---

	// If url is not empty, it means an upload or shortening was successful.
	if url != "" {
		go PlayUploaded()

		// This *clipFlag handles copying the *resulting URL* to the clipboard.
		// This is separate from the earlier image data or file path copying.
		if *clipFlag {
			errClipURL := CopyToClipboard(url, "text") // Assuming "text" copies the string `url`
			if errClipURL != nil {
				go PlayError()
				fmt.Fprintf(os.Stderr, "Failed to copy resulting URL to clipboard: %v\n", errClipURL)
				// Continue, not a fatal error for the main operation.
			}
		}

		action := "Uploaded"
		if inputURL != "" { // If original input was a URL, it was shortened.
			action = "Shortened"
		}
		fmt.Printf("%s: %s\n", action, url)

		if *notifyFlag {
			notifyMessage := fmt.Sprintf("%s successful: %s", action, url)
			var notifyErr error
			NOTIFY_ID, notifyErr = Notify(notifyMessage, NOTIFY_ID, filePath)
			if notifyErr != nil {
				go PlayError()
				fmt.Fprintf(os.Stderr, "Failed to show notification: %v\n", notifyErr)
			}
		}
	}
}
