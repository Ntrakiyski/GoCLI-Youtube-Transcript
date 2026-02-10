package main

import (
    "bufio"
    "fmt"
    "math/rand"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strings"
    "time"
)

var userAgents = []string{
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/121.0",
    "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/121.0",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: gocli-youtube-transcript [--audio] [--cookies <file>] [--cookies-from-browser <browser>] <youtube_url>")
        os.Exit(1)
    }
    var url, cookiesFile, cookiesFromBrowser string
    var downloadAudio bool
    args := os.Args[1:]
    i := 0
    for i < len(args) {
        switch args[i] {
        case "--audio":
            downloadAudio = true
            i++
        case "--cookies":
            if i+1 < len(args) {
                cookiesFile = args[i+1]
                i += 2
            } else {
                fmt.Fprintln(os.Stderr, "Error: --cookies requires a file path")
                os.Exit(1)
            }
        case "--cookies-from-browser":
            if i+1 < len(args) {
                cookiesFromBrowser = args[i+1]
                i += 2
            } else {
                fmt.Fprintln(os.Stderr, "Error: --cookies-from-browser requires a browser name")
                os.Exit(1)
            }
        default:
            url = args[i]
            i++
        }
    }
    if url == "" {
        fmt.Fprintln(os.Stderr, "Error: no URL provided")
        os.Exit(1)
    }

    rand.Seed(time.Now().UnixNano())
    userAgent := userAgents[rand.Intn(len(userAgents))]

    // Build yt-dlp command arguments
    argsList := []string{
        "--write-auto-sub",
        "--sub-lang", "en",
        "--sub-format", "vtt",
        "--js-runtimes", "node",
        "--user-agent", userAgent,
        "-o", "%(title)s.%(ext)s", // output template for both audio and vtt
    }
    if cookiesFile != "" {
        argsList = append(argsList, "--cookies", cookiesFile)
    }
    if cookiesFromBrowser != "" {
        argsList = append(argsList, "--cookies-from-browser", cookiesFromBrowser)
    }
    if downloadAudio {
        argsList = append(argsList, "-x", "--audio-format", "mp3")
    } else {
        argsList = append(argsList, "--skip-download")
    }
    argsList = append(argsList, url)

    // Call yt-dlp
    cmd := exec.Command("yt-dlp", argsList...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error running yt-dlp: %v\nOutput: %s\n", err, string(output))
        os.Exit(1)
    }

    // If not downloading audio, we need to read the VTT file for transcript
    if !downloadAudio {
        // Find the generated VTT file in current directory
        files, _ := filepath.Glob("*.en.vtt")
        if len(files) > 0 {
            vttPath := files[0]
            defer os.Remove(vttPath)
            printTranscript(vttPath)
        } else {
            fmt.Fprintln(os.Stderr, "No VTT file found")
        }
    }
    // If downloadAudio, transcript is already printed by yt-dlp? Actually yt-dlp with -x still writes subtitles to file. We could also print transcript.
    // For simplicity, when downloadAudio is true, we also try to print transcript from the generated VTT file.
    if downloadAudio {
        // Wait a bit for file to be written? yt-dlp runs synchronously, so after it returns files should exist.
        files, _ := filepath.Glob("*.en.vtt")
        if len(files) > 0 {
            vttPath := files[0]
            printTranscript(vttPath)
            defer os.Remove(vttPath)
        }
    }
}

func printTranscript(vttPath string) {
    file, err := os.Open(vttPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error opening VTT file %s: %v\n", vttPath, err)
        return
    }
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        if strings.HasPrefix(line, "WEBVTT") || strings.HasPrefix(line, "Kind:") || strings.HasPrefix(line, "Language:") {
            continue
        }
        if strings.Contains(line, "-->") {
            continue
        }
        if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
            continue
        }
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
