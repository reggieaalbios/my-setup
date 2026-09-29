package capture

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var allowed = []string{"content/", "schemas/", "go.mod", "go.sum", "packaging/winget/"}
var forbidden = []string{"credential", "token", "secret", "history", "cache", "cookie", "session", "backup", ".db", ".sqlite", "globalstorage", "workspacestorage", "hosts.yml"}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	Candidates    []string  `json:"candidates"`
	Diff          string    `json:"diff"`
}

func Run(in io.Reader, out io.Writer) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("capture is a Linux maintenance command")
	}
	rootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("capture must run inside the my-setup checkout")
	}
	root := strings.TrimSpace(string(rootBytes))
	if filepath.Base(root) != "my-setup" {
		return fmt.Errorf("capture must run inside the my-setup checkout")
	}
	fmt.Fprint(out, "Describe the intended setup change: ")
	description, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return fmt.Errorf("description is required")
	}
	status, err := git(root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	candidates := []string{}
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if len(line) < 4 {
			continue
		}
		name := strings.TrimSpace(line[3:])
		lower := strings.ToLower(name)
		if rejected(lower) {
			continue
		}
		if allow(name) {
			candidates = append(candidates, name)
		}
	}
	diff, _ := git(root, "diff", "--", "content", "schemas", "go.mod", "go.sum", "packaging/winget")
	report := Report{SchemaVersion: 1, Description: description, CreatedAt: time.Now().UTC(), Candidates: candidates, Diff: diff}
	dir := filepath.Join(root, ".mysetup", "captures")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	name := filepath.Join(dir, report.CreatedAt.Format("20060102T150405Z")+".json")
	data, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(name, append(data, '\n'), 0600); err != nil {
		return err
	}
	fmt.Fprintf(out, "Capture report: %s\n", name)
	prompt := fmt.Sprintf("Use $mysetup-maintainer to process capture report %s. Intent: %s", name, description)
	codex, err := exec.LookPath("codex")
	if err != nil {
		fmt.Fprintf(out, "Codex is unavailable. Run manually:\n  codex %q\n", prompt)
		return nil
	}
	cmd := exec.Command(codex, prompt)
	cmd.Dir = root
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(out, "Codex did not start or authenticate. Report preserved. Run:\n  codex %q\n", prompt)
	}
	return nil
}
func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return string(out), nil
}
func rejected(name string) bool {
	name = strings.ToLower(name)
	for _, term := range forbidden {
		if strings.Contains(name, term) {
			return true
		}
	}
	return false
}
func allow(name string) bool {
	for _, prefix := range allowed {
		if name == prefix || strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
