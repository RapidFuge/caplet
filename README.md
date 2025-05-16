# Caplet

Caplet is a Linux-native screenshot and file sharing utility inspired by ShareX. It provides a simple, efficient way to capture screenshots, upload files, and shorten URLs with configurable services.

## Features

- **Screenshot Capture**: Full-screen or region selection
- **File Uploading**: Upload any file type to configured services
- **URL Shortening**: Shorten URLs using configurable services
- **Clipboard Integration**: Copy screenshots directly to clipboard
- **History Tracking**: Keep track of all your uploads
- **Desktop Notifications**: Get notified about upload status
- **Sound Feedback**: Audio notifications for actions
- **SXCU Import**: Import ShareX Custom Uploader configurations

## Installation

### Prerequisites

- Go 1.18 or higher
- For screenshots on Wayland: `grim` + `slurp` or another supported screenshot tool (see below)
- For screenshots on X11: `maim`, `scrot`, or another supported screenshot tool (see below)
- For clipboard operations: `wl-clipboard` (Wayland) or `xclip` (X11)

### Building from Source

```bash
git clone https://github.com/rapidfuge/caplet.git
cd caplet
go build
```

### Installing

```bash
# Copy the binary to your PATH
sudo cp caplet /usr/local/bin/
```

## Usage

```bash
# Take a region screenshot and upload it
caplet -mode select

# Take a fullscreen screenshot and upload it
caplet -mode fullscreen

# Upload a file
caplet -mode file /path/to/file.png

# Shorten a URL
caplet -mode url https://example.com/very-long-url

# Upload clipboard contents
caplet -mode clipboard
```

### Command Line Options

```
  -clip
        Copy resulting URL to clipboard. (default true)
  -help
        Help command
  -history string
        Folder path to upload history (default "$HOME/Pictures/Screenshots/caplet")
  -mode string
        Set the mode.
        f/file: Upload a file.
        fs/fullscreen: Screenshoot entire screen
        s/select: Select screen region
        c/clipboard: Upload clipboard contents
        u/url: Shorten url
  -notify
        Show desktop notifications (default true)
  -save string
        Folder path to save screenshots/files (default "$HOME/Pictures/Screenshots/caplet")
  -sxcu string
        Path to the .sxcu config file
```

## Supported Screenshot Tools

### Wayland

- KDE Spectacle (`spectacle`)
- GNOME Screenshot (`gnome-screenshot`)
- Flameshot (`flameshot`)
- grim + slurp (`grim`, `slurp`)

### X11

- KDE Spectacle (`spectacle`)
- Flameshot (`flameshot`)
- XFCE Screenshot (`xfce4-screenshooter`)
- maim + slop (`maim`, `slop`)
- scrot (`scrot`)

## Configuration

Caplet stores its configuration in `$HOME/.config/caplet/config.json`. If the file doesn't exist, it will be created with default settings on first run.

### Default Configuration

```json
{
  "defaultFileUpload": "imgur",
  "defaultImageUpload": "imgur",
  "defaultUrlShortener": "",
  "historyPath": "$HOME/Pictures/Screenshots/caplet",
  "saveDir": "$HOME/Pictures/Screenshots/caplet",
  "organized": true,
  "uploaders": {
    "imgur": {
      "name": "Imgur",
      "requestURL": "https://api.imgur.com/3/image",
      "fileFormName": "image",
      "responseType": "json",
      "requestType": "POST",
      "regexps": {
        "url": "\\\"link\\\":\\\"(.+?)\\\""
      },
      "headers": {
        "Authorization": "Client-ID b972ecca954f246"
      }
    }
  },
  "shorteners": {}
}
```

### Importing ShareX Custom Uploaders

You can import ShareX Custom Uploader configurations (.sxcu files):

```bash
caplet -sxcu /path/to/uploader.sxcu
```

## History

Caplet keeps a history of all uploads in `$HOME/Pictures/Screenshots/caplet/history.json` (or your configured history path). Each entry contains:

- URL: The resulting URL
- File: The local file path
- Timestamp: When the upload occurred
- Service: The service used for the upload

## Creating Custom Uploaders

You can create custom uploaders by editing the config.json file. Here's an example structure:

```json
{
  "name": "MyCustomUploader",
  "requestURL": "https://example.com/upload",
  "fileFormName": "file",
  "responseType": "json",
  "requestType": "POST",
  "regexps": {
    "url": "\\\"url\\\":\\\"(.+?)\\\""
  },
  "headers": {
    "Authorization": "Bearer YOUR_TOKEN"
  },
  "arguments": {
    "visibility": "public"
  }
}
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Inspired by [ShareX](https://getsharex.com/) and [Sharenix](https://github.com/Francesco149/sharenix)
- Thanks to all the creators of the screenshot tools this program integrates with
