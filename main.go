package main

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strings"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: gocli-youtube-transcript <youtube_url>")
        os.Exit(1)
    }
    url := os.Args[1]

    // Create a temporary file base name for the VTT output
    tmpDir := os.TempDir()
    vttBase := filepath.Join(tmpDir, "yt-transcript")
    vttPath := vttBase + ".en.vtt" // yt-dlp appends .en.vtt
    // Ensure cleanup
    defer os.Remove(vttPath)

    // Call yt-dlp with Node.js runtime to get VTT subtitles
    cmd := exec.Command("yt-dlp",
        "--write-auto-sub",
        "--sub-lang", "en",
        "--skip-download",
        "--sub-format", "vtt",
        "--js-runtimes", "node",
        "-o", vttBase,
        url,
    )
    output, err := cmd.CombinedOutput()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error running yt-dlp: %v\nOutput: %s\n", err, string(output))
        os.Exit(1)
    }

    // Read and parse the VTT file
    file, err := os.Open(vttPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error opening VTT file %s: %v\n", vttPath, err)
        os.Exit(1)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        // Skip VTT headers
        if strings.HasPrefix(line, "WEBVTT") || strings.HasPrefix(line, "Kind:") || strings.HasPrefix(line, "Language:") {
            continue
        }
        // Skip timestamp lines (contain '-->')
        if strings.Contains(line, "-->") {
            continue
        }
        // Skip yt-dlp metadata lines
        if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
            continue
        }
        // Clean HTML tags like <c> and timestamps within text
        cleaned := regexp.MustCompile("<[^>]+>").ReplaceAllString(line, "")
        cleaned = strings.TrimSpace(cleaned)
        if cleaned != "" {
            fmt.Println(cleaned)
        }
    }
    if err := scanner.Err(); err != nil {
        fmt.Fprintf(os.Stderr, "Error reading VTT: %v\n", err)
    }
}
