# GoCLI-Youtube-Transcript

A Go CLI tool to download and extract transcripts from YouTube videos using yt-dlp.

## Features
- Fetches auto-generated English subtitles
- Outputs clean plain text transcript
- Removes timestamps and formatting tags

## Prerequisites
- Go 1.21+
- yt-dlp
- Node.js (for JavaScript runtime)

## Installation
```bash
go build -o gocli-youtube-transcript .
```

## Usage
```bash
./gocli-youtube-transcript <youtube_url>
```

## Example
```bash
./gocli-youtube-transcript https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

## How it works
1. Uses yt-dlp with Node.js runtime to fetch VTT subtitles
2. Parses the VTT file to extract text
3. Removes timestamps, headers, and HTML tags
4. Prints clean transcript to stdout

## License
MIT
