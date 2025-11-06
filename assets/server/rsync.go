package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

func getGenerationByTime() ([]string, error) {
	// Get backup directory
	entries, err := os.ReadDir(backupDestination)
	if err != nil {
		// ディレクトリが存在しない場合は作成を試みる
		if os.IsNotExist(err) {
			if createErr := os.MkdirAll(backupDestination, 0755); createErr != nil {
				return nil, fmt.Errorf("backup destination directory does not exist and cannot be created: %w", createErr)
			}
			return []string{}, nil // ディレクトリを作成し、世代は0で返す
		}
		return nil, fmt.Errorf("failed to read backup root directory: %w", err)
	}

	// Listup generations
	var generationDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()
			// Check timestamp(simple)
			if len(dirName) == 15 && dirName[8] == '_' {
				generationDirs = append(generationDirs, filepath.Join(backupDestination, dirName))
			}
		}
	}

	if len(generationDirs) == 0 {
		return []string{}, nil
	}

	// Sort by timestamp([0] is newest)
	sort.Sort(sort.Reverse(sort.StringSlice(generationDirs)))

	return generationDirs, nil
}

func backup(w http.ResponseWriter, r *http.Request) {
	if jvm != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("JVM is already working,"))
		return
	}

	args := []string{
		"-avhP",
		"--delete",
		fmt.Sprintf("--exclude=%s", backupDestination)}

	gen, err := getGenerationByTime()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	if len(gen) > 0 {
		args = append(args, fmt.Sprintf("--link-dest=%s", gen[0]))
	}

	currentBackupDir := filepath.Join(backupDestination, time.Now().Format("20060102_150405"))
	args = append(args, backupSource, currentBackupDir)

	out, err := exec.Command("rsync", args...).CombinedOutput()
	if err != nil {
		logger.Error("Backup rsync failed", "error", err, "output", string(out))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	logger.Info("Backup rsync succeeded", "output", string(out))

	if len(gen) > (keepGenerations - 1) {
		os.Remo
	}
}

func restore(w http.ResponseWriter, r *http.Request) {
}
