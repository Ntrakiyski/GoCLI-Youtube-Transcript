# GoCLI-Youtube-Transcript

A Go CLI tool to download and extract transcripts from YouTube videos using yt-dlp.

## Features
- Fetches auto-generated English subtitles
- Outputs clean plain text transcript
- Removes timestamps and formatting tags
- **Bypasses bot detection** using browser cookies or rotated user agents

## Prerequisites
- Go 1.21+
- yt-dlp
- Node.js (for JavaScript runtime)

## Installation
```bash
go build -o gocli-youtube-transcript .
sudo cp gocli-youtube-transcript /usr/local/bin/
```

## Usage
### Basic (may fail on bot-protected videos)
```bash
gocli-youtube-transcript <youtube_url>
```

### With Cookies (Recommended for bot-protected videos)
1. Install a cookie export extension (e.g., "Get cookies.txt" for Chrome/Firefox)
2. While logged into YouTube, export cookies to a file (e.g., `cookies.txt`)
3. Run:
```bash
gocli-youtube-transcript --cookies cookies.txt <youtube_url>
```

### With Browser Name (yt-dlp manages cookies automatically)
```bash
gocli-youtube-transcript --cookies-from-browser chrome <youtube_url>
# or
 gocli-youtube-transcript --cookies-from-browser firefox <youtube_url>
```

## Example
```bash
gocli-youtube-transcript --cookies cookies.txt https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

## How it works
1. Uses yt-dlp with Node.js runtime to fetch VTT subtitles
2. Randomly rotates user agents to mimic real browsers
3. Optionally uses browser cookies for authentication
4. Parses the VTT file to extract clean text
5. Removes timestamps, headers, and HTML tags
6. Prints clean transcript to stdout

## Troubleshooting
- **"Sign in to confirm you're not a bot"**: Use `--cookies` or `--cookies-from-browser`
- **No subtitles available**: Video may not have auto-generated or manual subtitles
- **yt-dlp errors**: Ensure Node.js is installed and accessible

## License
MIT
