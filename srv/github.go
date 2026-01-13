package srv

import (
	"encoding/json"
	"fmt"
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
		// Show that token exists but don't expose it
		if config.Token != "" {
			config.Token = "••••••••" // Indicate token is saved
		}
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
	
	// Keep existing token if new one not provided or is the masked value
	if config.Token == "" || config.Token == "••••••••" {
		config.Token = existing.Token
	}
	
	// Save config
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		writeError(w, "failed to save config: "+err.Error(), 500)
		return
	}
	
	writeJSON(w, map[string]string{"status": "ok", "message": "Configuration saved!"})
}

func (s *Server) HandleGitHubPull(w http.ResponseWriter, r *http.Request) {
	projectRoot := getProjectRoot()
	config := loadGitConfig()
	
	if config.Repo == "" {
		writeError(w, "No repository configured. Please save configuration first.", 400)
		return
	}
	
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	
	// Setup git remote with auth
	if err := setupGitRemote(projectRoot, config); err != nil {
		writeError(w, "Failed to setup remote: "+err.Error(), 500)
		return
	}
	
	// Git pull
	cmd := exec.Command("git", "pull", "origin", branch)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		writeError(w, "Pull failed: "+string(output), 500)
		return
	}
	
	writeJSON(w, map[string]string{"message": "Pull successful! " + string(output)})
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
	
	if config.Repo == "" {
		writeError(w, "No repository configured. Please save configuration first.", 400)
		return
	}
	
	if config.Token == "" {
		writeError(w, "No access token configured. Please add your GitHub token.", 400)
		return
	}
	
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	
	// Setup git remote with auth
	if err := setupGitRemote(projectRoot, config); err != nil {
		writeError(w, "Failed to setup remote: "+err.Error(), 500)
		return
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
	commitOutput, _ := cmd.CombinedOutput() // May fail if nothing to commit
	
	// Git push
	cmd = exec.Command("git", "push", "-u", "origin", branch)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		writeError(w, "Push failed: "+string(output), 500)
		return
	}
	
	result := "Push successful!"
	if strings.Contains(string(commitOutput), "nothing to commit") {
		result = "Nothing new to commit. " + result
	}
	
	writeJSON(w, map[string]string{"message": result})
}

func setupGitRemote(projectRoot string, config GitHubConfig) error {
	// Build authenticated URL
	remoteURL := config.Repo
	if config.Token != "" && strings.HasPrefix(remoteURL, "https://github.com") {
		// Format: https://TOKEN@github.com/user/repo.git
		remoteURL = strings.Replace(remoteURL, "https://github.com", "https://"+config.Token+"@github.com", 1)
	}
	
	// Check if origin exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = projectRoot
	if err := cmd.Run(); err != nil {
		// Add remote
		cmd = exec.Command("git", "remote", "add", "origin", remoteURL)
		cmd.Dir = projectRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add remote: %s", output)
		}
	} else {
		// Update remote URL
		cmd = exec.Command("git", "remote", "set-url", "origin", remoteURL)
		cmd.Dir = projectRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to update remote: %s", output)
		}
	}
	
	return nil
}

func getProjectRoot() string {
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
