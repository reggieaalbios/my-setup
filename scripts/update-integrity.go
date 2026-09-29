//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type index struct {
	SchemaVersion int               `json:"schema_version"`
	Files         map[string]string `json:"files"`
}

func main() {
	root := "content"
	names := []string{}
	_ = filepath.WalkDir(root, func(name string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(root, name)
		relative = filepath.ToSlash(relative)
		if relative == "integrity.json" || relative == "embed.go" {
			return nil
		}
		names = append(names, relative)
		return nil
	})
	sort.Strings(names)
	result := index{SchemaVersion: 1, Files: map[string]string{}}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			panic(err)
		}
		sum := sha256.Sum256(data)
		result.Files[name] = hex.EncodeToString(sum[:])
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(root, "integrity.json"), []byte(strings.TrimSpace(string(data))+"\n"), 0644); err != nil {
		panic(err)
	}
}
