package main

import (
    "bufio"
    "bytes"
    "compress/gzip"
    "encoding/json"
    "fmt"
    "io"
    "math/rand"
    "net/http"
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
    
    // Try direct captionTracks baseUrl first (most reliable for public videos)
    fmt.Fprintln(os.Stderr, "Attempting direct captionTracks extraction...")
    html, err := fetchPageHTML(url, userAgent, cookiesFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to fetch page HTML: %v\n", err)
    } else {
        if baseURL := extractCaptionBaseURL(html); baseURL != "" {
            if err := fetchAndPrintTranscript(baseURL, userAgent, cookiesFile); err == nil {
                if downloadAudio {
                    fmt.Fprintln(os.Stderr, "\nNote: Audio download requires yt-dlp fallback")
                }
                os.Exit(0)
            } else {
                fmt.Fprintf(os.Stderr, "Direct caption fetch failed: %v\n", err)
            }
        }
    }
    
    // Fallback to youtubei native API
    fmt.Fprintln(os.Stderr, "Attempting transcript extraction via YouTube native API...")
    if html != "" && err == nil {
        config, err := extractBootstrapConfig(html)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to extract bootstrap config: %v\n", err)
        } else {
            segments, err := fetchTranscriptViaYoutubei(url, userAgent, cookiesFile, config)
            if err != nil {
                fmt.Fprintf(os.Stderr, "youtubei API failed: %v\n", err)
            } else {
                printTranscriptFromSegments(segments)
                if downloadAudio {
                    fmt.Fprintln(os.Stderr, "\nNote: Audio download requires yt-dlp fallback")
                }
                os.Exit(0)
            }
        }
    }
    
    // Fallback to yt-dlp
    fmt.Fprintln(os.Stderr, "Falling back to yt-dlp...")
    if err := runYtDlp(url, userAgent, cookiesFile, cookiesFromBrowser, downloadAudio); err != nil {
        fmt.Fprintf(os.Stderr, "yt-dlp failed: %v\n", err)
        os.Exit(1)
    }
    
    if !downloadAudio {
        files, _ := filepath.Glob("*.en.vtt")
        if len(files) > 0 {
            vttPath := files[0]
            printTranscriptFromVTT(vttPath)
            defer os.Remove(vttPath)
        } else {
            fmt.Fprintln(os.Stderr, "No VTT file found")
        }
    }
    if downloadAudio {
        files, _ := filepath.Glob("*.en.vtt")
        if len(files) > 0 {
            vttPath := files[0]
            printTranscriptFromVTT(vttPath)
            defer os.Remove(vttPath)
        }
    }
}

// fetchPageHTML retrieves the YouTube video page HTML
func fetchPageHTML(url, userAgent, cookiesFile string) (string, error) {
    client := &http.Client{Timeout: 30 * time.Second}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return "", err
    }
    req.Header.Set("User-Agent", userAgent)
    req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
    req.Header.Set("Accept-Language", "en-US,en;q=0.5")
    req.Header.Set("Accept-Encoding", "gzip, deflate")
    req.Header.Set("DNT", "1")
    req.Header.Set("Connection", "keep-alive")
    req.Header.Set("Upgrade-Insecure-Requests", "1")
    
    if cookiesFile != "" {
        data, err := os.ReadFile(cookiesFile)
        if err != nil {
            return "", fmt.Errorf("failed to read cookies file: %v", err)
        }
        req.Header.Set("Cookie", string(data))
    }
    
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    var reader io.ReadCloser
    switch resp.Header.Get("Content-Encoding") {
    case "gzip":
        gz, err := gzip.NewReader(resp.Body)
        if err != nil {
            return "", err
        }
        defer gz.Close()
        reader = gz
    default:
        reader = resp.Body
    }
    
    body, err := io.ReadAll(reader)
    if err != nil {
        return "", err
    }
    
    if resp.StatusCode != 200 {
        return "", fmt.Errorf("HTTP %d", resp.StatusCode)
    }
    
    return string(body), nil
}

// extractCaptionBaseURL finds captionTracks baseUrl in ytInitialPlayerResponse
func extractCaptionBaseURL(html string) string {
    re := regexp.MustCompile(`var ytInitialPlayerResponse = ({[^;]+});`)
    if matches := re.FindStringSubmatch(html); len(matches) > 1 {
        var player map[string]interface{}
        if err := json.Unmarshal([]byte(matches[1]), &player); err != nil {
            return ""
        }
        if captions, ok := player["captions"].(map[string]interface{}); ok {
            if playerCaptions, ok := captions["playerCaptionsTracklistRenderer"].(map[string]interface{}); ok {
                if captionTracks, ok := playerCaptions["captionTracks"].([]interface{}); ok && len(captionTracks) > 0 {
                    // Find English track or first track
                    for _, ct := range captionTracks {
                        if track, ok := ct.(map[string]interface{}); ok {
                            lang, _ := track["languageCode"].(string)
                            kind, _ := track["kind"].(string)
                            if lang == "en" || lang == "en-US" || kind == "asr" {
                                if baseURL, ok := track["baseUrl"].(string); ok {
                                    return baseURL
                                }
                            }
                        }
                    }
                    // Fallback to first track
                    if track, ok := captionTracks[0].(map[string]interface{}); ok {
                        if baseURL, ok := track["baseUrl"].(string); ok {
                            return baseURL
                        }
                    }
                }
            }
        }
    }
    return ""
}

// fetchAndPrintTranscript downloads caption file and prints clean text
func fetchAndPrintTranscript(baseURL, userAgent, cookiesFile string) error {
    client := &http.Client{Timeout: 30 * time.Second}
    req, err := http.NewRequest("GET", baseURL, nil)
    if err != nil {
        return err
    }
    req.Header.Set("User-Agent", userAgent)
    if cookiesFile != "" {
        data, err := os.ReadFile(cookiesFile)
        if err != nil {
            return fmt.Errorf("failed to read cookies file: %v", err)
        }
        req.Header.Set("Cookie", string(data))
    }
    
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("HTTP %d", resp.StatusCode)
    }
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    
    // The caption format is usually VTT or XML. Try to parse as VTT.
    content := string(body)
    scanner := bufio.NewScanner(strings.NewReader(content))
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
    return scanner.Err()
}

// BootstrapConfig holds extracted YouTube bootstrap configuration
type BootstrapConfig struct {
    APIKey         string
    Context        string
    VisitorData    string
    ClientVersion  string
    ClientName     string
    PageCL         string
    PageLabel      string
}

// extractBootstrapConfig parses HTML to get YouTube API configuration
func extractBootstrapConfig(html string) (*BootstrapConfig, error) {
    config := &BootstrapConfig{}
    
    reAPIKey := regexp.MustCompile(`"INNERTUBE_API_KEY":"([^"]+)"`)
    if matches := reAPIKey.FindStringSubmatch(html); len(matches) > 1 {
        config.APIKey = matches[1]
    } else {
        return nil, fmt.Errorf("INNERTUBE_API_KEY not found")
    }
    
    reContext := regexp.MustCompile(`"INNERTUBE_CONTEXT":({[^}]+})`)
    if matches := reContext.FindStringSubmatch(html); len(matches) > 1 {
        config.Context = matches[1]
    } else {
        return nil, fmt.Errorf("INNERTUBE_CONTEXT not found")
    }
    
    reVisitor := regexp.MustCompile(`"VISITOR_DATA":"([^"]+)"`)
    if matches := reVisitor.FindStringSubmatch(html); len(matches) > 1 {
        config.VisitorData = matches[1]
    } else {
        return nil, fmt.Errorf("VISITOR_DATA not found")
    }
    
    reClientVersion := regexp.MustCompile(`"INNERTUBE_CLIENT_VERSION":"([^"]+)"`)
    if matches := reClientVersion.FindStringSubmatch(html); len(matches) > 1 {
        config.ClientVersion = matches[1]
    } else {
        config.ClientVersion = "2.20240214.01.00"
    }
    
    reClientName := regexp.MustCompile(`"INNERTUBE_CLIENT_NAME":"([^"]+)"`)
    if matches := reClientName.FindStringSubmatch(html); len(matches) > 1 {
        config.ClientName = matches[1]
    } else {
        config.ClientName = "WEB"
    }
    
    rePageCL := regexp.MustCompile(`"pageCl":"([^"]+)"`)
    if matches := rePageCL.FindStringSubmatch(html); len(matches) > 1 {
        config.PageCL = matches[1]
    } else {
        config.PageCL = ""
    }
    
    rePageLabel := regexp.MustCompile(`"pageLabel":"([^"]+)"`)
    if matches := rePageLabel.FindStringSubmatch(html); len(matches) > 1 {
        config.PageLabel = matches[1]
    } else {
        config.PageLabel = ""
    }
    
    return config, nil
}

// fetchTranscriptViaYoutubei calls YouTube's internal transcript API
func fetchTranscriptViaYoutubei(url, userAgent, cookiesFile string, config *BootstrapConfig) ([]TranscriptSegment, error) {
    videoID := extractVideoID(url)
    if videoID == "" {
        return nil, fmt.Errorf("invalid YouTube URL")
    }
    
    // Parse context JSON
    var contextMap map[string]interface{}
    if err := json.Unmarshal([]byte(config.Context), &contextMap); err != nil {
        return nil, fmt.Errorf("failed to parse context: %v", err)
    }
    
    reqBody := map[string]interface{}{
        "context": contextMap,
        "params": "EgVob3N0X2F2dF9sYWIoZ2VzdXJlX2F2dF9sYWJzKXIMIhC8htf_48C4h4o4EnRlbnNpdHlfZm9ybXMsIHZlcnNpb249MS4wLjAuMCx0aW1lc3RhbXA9MTcwNzgwMDAwMl8xMDc1MDk0MTY1JnJlbGlzdF9wYXJ0X2lkPTQzJnJlZmVycmVkX2NvbnRlbnRfYXN5bmNocm9zXSJ9Cg%3D%3D",
        "videoId": videoID,
    }
    
    jsonBody, err := json.Marshal(reqBody)
    if err != nil {
        return nil, err
    }
    
    endpoint := fmt.Sprintf("https://www.youtube.com/youtubei/v1/get_transcript?key=%s", config.APIKey)
    
    client := &http.Client{Timeout: 30 * time.Second}
    httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBody))
    if err != nil {
        return nil, err
    }
    
    httpReq.Header.Set("User-Agent", userAgent)
    httpReq.Header.Set("Accept", "application/json")
    httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Origin", "https://www.youtube.com")
    httpReq.Header.Set("Referer", url)
    httpReq.Header.Set("X-Goog-Visitor-Id", config.VisitorData)
    httpReq.Header.Set("X-Youtube-Client-Name", config.ClientName)
    httpReq.Header.Set("X-Youtube-Client-Version", config.ClientVersion)
    if config.PageCL != "" {
        httpReq.Header.Set("X-Youtube-Page-CL", config.PageCL)
    }
    if config.PageLabel != "" {
        httpReq.Header.Set("X-Youtube-Page-Label", config.PageLabel)
    }
    
    if cookiesFile != "" {
        data, err := os.ReadFile(cookiesFile)
        if err != nil {
            return nil, fmt.Errorf("failed to read cookies file: %v", err)
        }
        httpReq.Header.Set("Cookie", string(data))
    }
    
    resp, err := client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }
    
    var result struct {
        Actions []struct {
            UpdateEndpoint struct {
                Actions []struct {
                    TranscriptSegment struct {
                        Snippet struct {
                            Text string `json:"text"`
                        } `json:"snippet"`
                        StartMs  int `json:"startMs"`
                        Duration int `json:"durationMs"`
                    } `json:"transcriptSegmentRenderer"`
                } `json:"actions"`
            } `json:"updateEndpoint"`
        } `json:"actions"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    var segments []TranscriptSegment
    for _, action := range result.Actions {
        for _, act := range action.UpdateEndpoint.Actions {
            seg := act.TranscriptSegment
            if seg.Snippet.Text != "" {
                segments = append(segments, TranscriptSegment{
                    Text:     seg.Snippet.Text,
                    StartMs:  seg.StartMs,
                    Duration: seg.Duration,
                })
            }
        }
    }
    
    if len(segments) == 0 {
        return nil, fmt.Errorf("no transcript segments found")
    }
    
    return segments, nil
}

func extractVideoID(url string) string {
    patterns := []string{
        `v=([^&]+)`,
        `youtu\.be/([^?&]+)`,
        `embed/([^?&]+)`,
        `^([a-zA-Z0-9_-]{11})$`,
    }
    for _, pattern := range patterns {
        re := regexp.MustCompile(pattern)
        if matches := re.FindStringSubmatch(url); len(matches) > 1 {
            return matches[1]
        }
    }
    return ""
}

func runYtDlp(url, userAgent, cookiesFile, cookiesFromBrowser string, downloadAudio bool) error {
    argsList := []string{
        "--write-auto-sub",
        "--sub-lang", "en",
        "--sub-format", "vtt",
        "--js-runtimes", "node",
        "--user-agent", userAgent,
        "-o", "%(title)s.%(ext)s",
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
    
    cmd := exec.Command("yt-dlp", argsList...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("yt-dlp error: %v\nOutput: %s", err, string(output))
    }
    return nil
}

func printTranscriptFromSegments(segments []TranscriptSegment) {
    for _, seg := range segments {
        fmt.Println(seg.Text)
    }
}

func printTranscriptFromVTT(vttPath string) {
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

// New struct for transcript segments
type TranscriptSegment struct {
    Text     string
    StartMs  int
    Duration int
}
