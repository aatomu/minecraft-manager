package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

func backup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
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

	backupTime := time.Now().Format("20060102_150405")
	currentBackupDir := filepath.Join(backupDestination, backupTime)
	args = append(args, backupSource, currentBackupDir)

	out, err := exec.Command("rsync", args...).CombinedOutput()
	writeLog(fmt.Sprintf("backup_%s.log", backupTime), out)
	if err != nil {
		logger.Error("Backup rsync failed", "error", err, "output", string(out))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	logger.Info("Backup rsync succeeded", "output", string(out))

	err = clearOldGeneration()
	if err != nil {
		logger.Error("Remove old generation failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)

	return
}

func clearOldGeneration() error {
	gen, err := getGenerationByTime()
	if err != nil {
		return err
	}

	if len(gen) <= keepGenerations {
		return nil
	}

	sort.Strings(gen)

	for _, fullPath := range gen[:len(gen)-keepGenerations] {
		logger.Info("Deleting old generation", "path", fullPath)
		if err := os.RemoveAll(fullPath); err != nil {
			logger.Error("Failed to delete old generation", "path", fullPath, "error", err)
		}
	}
	return nil
}

func restore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if jvm != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("JVM is already working,"))
		return
	}

	restoreTime := r.URL.Query().Get("t")

	if restoreTime == "" {
		gen, err := getGenerationByTime()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Failed to list generations: %s", err.Error())))
			return
		}

		w.WriteHeader(http.StatusOK)

		timestamps := []string{}
		for _, fullPath := range gen {
			timestamps = append(timestamps, filepath.Base(fullPath))
		}
		b, err := json.MarshalIndent(timestamps, "", "\t")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Failed to list generations: %s", err.Error())))
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.Write(b)
		return
	}

	// Make restore path (backupDestination/YYYYMMDD_hhmmss)
	if !checkTimeFormat(restoreTime) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad timestamp,"))
		return
	}
	restoreSourceDir := filepath.Join(backupDestination, restoreTime)

	if _, err := os.Stat(restoreSourceDir); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Backup generation not found: %s", restoreTime)))
		return
	}

	args := []string{
		"-avhP",
		"--delete",
		restoreSourceDir + "/",
		backupSource,
	}

	logger.Info("Starting restore", "source", restoreSourceDir, "destination", backupSource)

	out, err := exec.Command("rsync", args...).CombinedOutput()
	writeLog(fmt.Sprintf("restore_%s_by_%s.log", time.Now().Format("20060102_150405"), restoreTime), out)
	if err != nil {
		logger.Error("Restore rsync failed", "error", err, "output", string(out))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Restore failed: %s", string(out))))
		return
	}

	logger.Info("Restore rsync succeeded", "generation", restoreTime, "output", string(out))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Restore from generation %s completed successfully to %s.\n%s", restoreTime, backupSource, string(out))))

	return
}

func checkTimeFormat(s string) bool {
	const timeFormat = "20060102_150405"
	_, err := time.Parse(timeFormat, s)
	return err == nil
}

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
			if checkTimeFormat(dirName) {
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

func writeLog(fileName string, s []byte) {
	filePath := filepath.Join(backupDestination, fileName)

	// ログファイルに書き込み
	logFile, writeErr := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if writeErr != nil {
		logger.Error("Failed to open log file", "path", filePath, "error", writeErr)
	} else {
		_, writeErr = logFile.Write(s)
		if writeErr != nil {
			logger.Error("Failed to write rsync output to log file", "path", filePath, "error", writeErr)
		}
		logFile.Close()
	}

	return
}
