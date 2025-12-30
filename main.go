package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
    "strings"
    "os/exec"
)


const banner = `
░█▀▀░▀█▀░▀█▀░█▀▀░█░█░█░█░█░█░█▄█░█▀▀
░█░█░░█░░░█░░█▀▀░▄▀▄░█▀█░█░█░█░█░█▀▀
░▀▀▀░▀▀▀░░▀░░▀▀▀░▀░▀░▀░▀░▀▀▀░▀░▀░▀▀▀`
const version = `1.0`


// readWordlist loads a wordlist file from disk and returns all
// keywords as a slice of strings.
// The program returns an error if the file cannot be read.
func readWordlist(wordlist string) ([]string, error) {
    data, err := os.ReadFile(wordlist)
    if err != nil {
        return nil, err
    }
    
    return strings.Fields(string(data)), nil
}


// fetchRepos retrieves all public GitHub repositories for the given username
// using the GitHub REST API and returns them as a slice of Repo.
// The program returns an error on network, API, or decoding errors.
func fetchRepos(username string) ([]Repo, error) {
    userUrl := fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=100", username)

    resp, err := http.Get(userUrl)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("Github API error (%d): %s", resp.StatusCode, body)
    }

    var repos []Repo
    if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
        return nil, err
    }

    return repos, nil
}


// filterRepos restricts the given repository list to the names specified
// in srepos. If srepos is empty, all repositories are returned.
// The program returns an error if any requested repository does not exist.
func filterRepos(repos []Repo, srepos []string) ([]Repo, error) {
    if len(srepos) == 0 {
        return repos, nil
    }

    repoMap := make(map[string]Repo)
    for _, r := range repos {
        repoMap[r.Name] = r
    }

    var filtered []Repo
    for _, name := range srepos {
        r, ok := repoMap[name]
        if !ok {
            return nil, fmt.Errorf("repository '%s' not found", name)
        }
        filtered = append(filtered, r)
    }

    return filtered, nil
}


// presentRepos prints an overview of the repositories and their total size,
// then prompts the user to continue.
// It returns the user's response as a string.
func presentRepos(repos []Repo) string {
    var totalSize int = 0;
    fmt.Println("Repositories:")
    for _, r := range repos {
        fmt.Printf(" - %s (%d KB)\n", r.Name, r.Size)
        totalSize += r.Size
    }

    var Continue string
    fmt.Printf("Total size is %v KB, continue? (Y/n): ", totalSize)
    fmt.Scanf("%s\n", &Continue)
    
    return Continue
}


// storeRepos clones all repositories into OutputDir using git.
// Each repository is cloned into its own subdirectory.
// The program returns an error if directory creation or cloning fails.
func storeRepos(repos []Repo, OutputDir string, username string) {
    if err := os.Mkdir(OutputDir, os.ModePerm); err != nil {
        fmt.Printf("Error creating %s directory: %s\n", OutputDir, err)
        os.Exit(1)
    }

    // Clone git repositories in OutputDir
    for _, r := range repos {
        repoURL := fmt.Sprintf("https://github.com/%s/%s.git", username, r.Name)
        targetDir := fmt.Sprintf("%s/%s", OutputDir, r.Name)

        cmd := exec.Command("git", "clone", repoURL, targetDir)
        cmd.Stdout = nil
        cmd.Stderr = nil

        if err := cmd.Run(); err != nil {
            fmt.Printf("Error cloning %s: %v\n", r.Name, err)
            os.Exit(1)
        }
    }

    fmt.Printf("\033[32mAll %d repositories have been cloned successfully\033[0m\n", len(repos))
}


// searchRepos scans the full commit history of each repository for the
// configured keywords using git grep. Matching lines are printed once
// per file and content combination to avoid duplicates.
func searchRepos(repos []Repo, OutputDir string, words []string) {
    for _, r := range repos {
        repoDir := fmt.Sprintf("%s/%s", OutputDir, r.Name)
        pattern := strings.Join(words, "|")

        cmd := exec.Command(
            "sh", "-c",
            fmt.Sprintf(
                "cd %s && git grep -n --color=always -E '%s' $(git rev-list --all)",
                repoDir, pattern,
            ),
        )

        out, err := cmd.Output()
        if err != nil {
            continue // no matches
        }

        seen := make(map[string]bool) // reset per repo
        lines := strings.Split(strings.TrimSpace(string(out)), "\n")
        for _, line := range lines {
            // commit:file:line:content
            parts := strings.SplitN(line, ":", 4)
            if len(parts) < 4 {
                continue
            }

            commit := parts[0][:11]
            file := parts[1]
            lineNr := parts[2]
            content := parts[3]

            // dedup key: same file + same content
            key := file + "|" + content
            if seen[key] {
                continue
            }
            seen[key] = true

            fmt.Printf("[%s] %s %s:%s\t", r.Name, commit, file, lineNr)
            fmt.Printf("  %s\n", content)
        }
    }
}


func main() {
    // Obtain and handle flags
    var uFlag = flag.String("u", "", "Username of repositories")
    var rFlag = flag.String("r", "", "Repositories, separated by comma")
    var wFlag = flag.String("w", DefaultWordlist, "Wordlist file")
    var sFlag = flag.String("s", "", "Scan existing directory (skip cloning)")
    var vFlag = flag.Bool("version", false, "Print version and exit")
    flag.Parse()

    // Validate and handle flags:
    // Display version
    if *vFlag {
        fmt.Printf("%s\nVersion: %s\n", banner, version)
        os.Exit(0)
    }
    // -u is required unless -s is specified
    if *sFlag == "" && *uFlag == "" {
        fmt.Println("Error: -u is required unless -s is specified")
        flag.Usage()
        os.Exit(1)
    }
    // Parse selected repositories
    var srepos []string
    if *rFlag != "" {
        srepos = strings.Split(*rFlag, ",")
    }
    // Read wordlist
    words, err := readWordlist(*wFlag)
    if err != nil {
        fmt.Fprintf(os.Stderr, "wordlist error: %v\n", err)
        os.Exit(1)
    }
    

    fmt.Printf("%v\nVersion: %v\n\n", banner, version)

    // SCAN-ONLY MODE
    if *sFlag != "" {
        fmt.Printf("Scanning existing directory: %s\n", *sFlag)

        entries, err := os.ReadDir(*sFlag)
        if err != nil {
            fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
            os.Exit(1)
        }

        var repos []Repo
        for _, e := range entries {
            if e.IsDir() {
                repos = append(repos, Repo{Name: e.Name()})
            }
        }

        searchRepos(repos, *sFlag, words)
        os.Exit(0)
    }

    // COMPLETE MODE (fetch, clone, scan)
    repos, err := fetchRepos(*uFlag)
    if err != nil {
        fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
        os.Exit(1)
    }

    repos, err = filterRepos(repos, srepos)
    if err != nil {
        fmt.Fprintf(os.Stderr, "filter error: %v\n", err)
        os.Exit(1)
    }

    Continue := presentRepos(repos)

    if Continue == "Y" || Continue == "y" {
        fmt.Println("Continuing...")
        storeRepos(repos, OutputDir, *uFlag)
        searchRepos(repos, OutputDir, words)
    } else {
        fmt.Println("Not Continuing...")
    }

    os.Exit(0)
}

