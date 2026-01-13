package srv

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const gitConfigFile = ".github-config.json"

type GitHubConfig struct {
	Repo   string `json:"repo"`
	Token  string `json:"token,omitempty"`
	Branch string `json:"branch"`
}

func (s *Server) HandleGitHubConfig(w http.ResponseWriter, r *http.Request) {
	configPath := filepath.Join(getProjectRoot(), gitConfigFile)
	
	if r.Method == "GET" {
		// Return config (without token for security)
		data, err := os.ReadFile(configPath)
		if err != nil {
			writeJSON(w, GitHubConfig{Branch: "main"})
			return
		}
		var config GitHubConfig
		json.Unmarshal(data, &config)
		config.Token = "" // Don't expose token
		writeJSON(w, config)
		return
	}
	
	// POST - save config
	var config GitHubConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	
	if config.Branch == "" {
		config.Branch = "main"
	}
	
	// Load existing config to preserve token if not provided
	existingData, _ := os.ReadFile(configPath)
	var existing GitHubConfig
	json.Unmarshal(existingData, &existing)
	
	if config.Token == "" {
		config.Token = existing.Token
	}
	
	// Save config
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		writeError(w, "failed to save config: "+err.Error(), 500)
		return
	}
	
	// Configure git remote if repo changed
	if config.Repo != "" {
		projectRoot := getProjectRoot()
		
		// Setup remote with token embedded for auth
		remoteURL := config.Repo
		if config.Token != "" && strings.HasPrefix(remoteURL, "https://") {
			// Insert token into URL: https://token@github.com/...
			remoteURL = strings.Replace(remoteURL, "https://", "https://"+config.Token+"@", 1)
		}
		
		// Check if remote exists
		cmd := exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = projectRoot
		if err := cmd.Run(); err != nil {
			// Add remote
			cmd = exec.Command("git", "remote", "add", "origin", remoteURL)
			cmd.Dir = projectRoot
			cmd.Run()
		} else {
			// Update remote
			cmd = exec.Command("git", "remote", "set-url", "origin", remoteURL)
			cmd.Dir = projectRoot
			cmd.Run()
		}
	}
	
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) HandleGitHubPull(w http.ResponseWriter, r *http.Request) {
	projectRoot := getProjectRoot()
	
	// Load config for branch
	config := loadGitConfig()
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	
	// Git pull
	cmd := exec.Command("git", "pull", "origin", branch)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		writeError(w, "Pull failed: "+string(output), 500)
		return
	}
	
	writeJSON(w, map[string]string{"message": string(output)})
}

func (s *Server) HandleGitHubPush(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	
	if req.Message == "" {
		req.Message = "Update bookmark manager"
	}
	
	projectRoot := getProjectRoot()
	config := loadGitConfig()
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	
	// Git add all changes
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		writeError(w, "Git add failed: "+string(output), 500)
		return
	}
	
	// Git commit
	cmd = exec.Command("git", "commit", "-m", req.Message)
	cmd.Dir = projectRoot
	cmd.CombinedOutput() // Ignore error if nothing to commit
	
	// Git push
	cmd = exec.Command("git", "push", "origin", branch)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		writeError(w, "Push failed: "+string(output), 500)
		return
	}
	
	writeJSON(w, map[string]string{"message": "Pushed successfully"})
}

func getProjectRoot() string {
	// Return the project root directory
	return "/home/exedev/bookmark-manager"
}

func loadGitConfig() GitHubConfig {
	configPath := filepath.Join(getProjectRoot(), gitConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return GitHubConfig{Branch: "main"}
	}
	var config GitHubConfig
	json.Unmarshal(data, &config)
	return config
}
