package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func TakeScreenshotWayland(region bool, outputPath string) error {
	// --- 1. Spectacle (KDE)
	if commandExists("spectacle") {
		args := []string{"-n", "-b", "-o", outputPath}
		if region {
			args = append(args, "-r")
		}
		cmd := exec.Command("spectacle", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("spectacle failed: %w", err)
		}
		return nil
	}

	// --- 2. gnome-screenshot
	if commandExists("gnome-screenshot") {
		args := []string{}
		if region {
			args = append(args, "-a")
		} else {
			args = append(args, "-f", outputPath)
		}
		cmd := exec.Command("gnome-screenshot", args...)
		if region {
			cmd = exec.Command("gnome-screenshot", "-a")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("gnome-screenshot (region) failed: %w", err)
			}
			// No file output path support for region, user must manually save
			return nil
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gnome-screenshot failed: %w", err)
		}
		return nil
	}

	// --- 3. Flameshot
	if commandExists("flameshot") {
		args := []string{"gui"}
		if !region {
			args = append(args, "--fullscreen", "--path", outputPath)
		}
		cmd := exec.Command("flameshot", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("flameshot failed: %w", err)
		}
		return nil
	}

	// --- 4. grim + slurp (wlroots)
	if !commandExists("grim") {
		return fmt.Errorf("no compatible screenshot tool found. Compatible wayland screenshot tool: KDE Spectacle,  gnome-screenshot, flameshot, or grim + slurp")
	}

	if region && !commandExists("slurp") {
		return fmt.Errorf("slurp is required for region screenshots for grim but not found in PATH")
	}

	if region {
		// Optional: freeze screen with hyprpicker if available
		var hyprpickerProcess *exec.Cmd
		var hyprpickerStarted bool

		if commandExists("hyprpicker") && commandExists("hyprctl") {
			_ = exec.Command("hyprctl", "keyword", "layerrule", "noanim,selection").Run()
			hyprpickerProcess = exec.Command("hyprpicker", "-r", "-z")
			if err := hyprpickerProcess.Start(); err == nil {
				hyprpickerStarted = true
				time.Sleep(200 * time.Millisecond)
			}
		}

		slurpCmd := exec.Command("slurp")
		slurpOutput, err := slurpCmd.Output()

		if hyprpickerStarted && hyprpickerProcess != nil && hyprpickerProcess.Process != nil {
			_ = hyprpickerProcess.Process.Kill()
		}

		if err != nil {
			return fmt.Errorf("failed to get region with slurp: %w", err)
		}

		geometry := strings.TrimSpace(string(slurpOutput))
		if geometry == "" {
			return fmt.Errorf("no region selected")
		}

		grimCmd := exec.Command("grim", "-g", geometry, outputPath)
		if err := grimCmd.Run(); err != nil {
			return fmt.Errorf("grim (region) failed: %w", err)
		}
	} else {
		grimCmd := exec.Command("grim", outputPath)
		if err := grimCmd.Run(); err != nil {
			return fmt.Errorf("grim (fullscreen) failed: %w", err)
		}
	}

	return nil
}

func TakeScreenshotX11(region bool, outputPath string) error {
	if commandExists("spectacle") {
		args := []string{"-n", "-b", "-o", outputPath}
		if region {
			args = append(args, "-r")
		}
		cmd := exec.Command("spectacle", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("spectacle failed: %w", err)
		}
		return nil
	}

	if commandExists("flameshot") {
		args := []string{"gui", "--path", outputPath}
		if !region {
			args = append(args, "--fullscreen")
		}
		cmd := exec.Command("flameshot", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("flameshot failed: %w", err)
		}
		return nil
	}

	if commandExists("xfce4-screenshooter") {
		args := []string{"-f", "-s", outputPath}
		if region {
			args = []string{"-r", "-s", outputPath}
		}
		cmd := exec.Command("xfce4-screenshooter", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("xfce4-screenshooter failed: %w", err)
		}
		return nil
	}

	if commandExists("maim") {
		args := []string{outputPath}
		if region {
			if !commandExists("slop") {
				return fmt.Errorf("maim requires 'slop' for region selection, but it was not found")
			}
			cmd := exec.Command("slop", "-f", "%x,%y,%w,%h")
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("slop failed: %w", err)
			}
			coords := strings.TrimSpace(string(output))
			parts := strings.Split(coords, ",")
			if len(parts) != 4 {
				return fmt.Errorf("invalid region format from slop")
			}
			args = []string{
				"-g", fmt.Sprintf("%sx%s+%s+%s", parts[2], parts[3], parts[0], parts[1]),
				outputPath,
			}
		}
		cmd := exec.Command("maim", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("maim failed: %w", err)
		}
		return nil
	}

	if commandExists("scrot") {
		args := []string{outputPath}
		if region {
			args = []string{"-s", outputPath}
		}
		cmd := exec.Command("scrot", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("scrot failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no compatible screenshot tool found for X11")
}

// TakeScreenshot captures a screenshot
func TakeScreenshot(region bool) (string, error) {
	// Create a temporary directory for the screenshot
	tempDir, err := os.MkdirTemp("", "caplet-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	outputPath := filepath.Join(tempDir, fmt.Sprintf("screenshot-%s.png", time.Now().Format("2006-01-02_15-04-05")))
	if runtime.GOOS == "linux" {
		// Check if we're using Wayland or X11
		waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
		if waylandDisplay != "" {
			if err := TakeScreenshotWayland(region, outputPath); err != nil {
				return "", err
			}
		} else {
			if err := TakeScreenshotX11(region, outputPath); err != nil {
				return "", err
			}
		}
	} else {
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return outputPath, nil
}
