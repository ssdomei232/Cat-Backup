package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/studio-b12/gowebdav"
)

const (
	LogPrefixInfo  = "[INFO] "
	LogPrefixWarn  = "[WARN] "
	LogPrefixError = "[ERROR] "
)

type Config struct {
	WebdavURL      string
	WebdavUser     string
	WebdavPassword string
	CacheDir       string
	SMTPhost       string
	SMTPort        int
	SMTPUser       string
	SMTPPassword   string
	SMTPTo         []string
	Backups        []BackupConfig
}

type BackupConfig struct {
	Name       string
	Source     string
	RemotePath string
	Frequency  string
	Retries    int
}

func main() {
	envFile := flag.String("c", ".env", "Path to .env file")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println(LogPrefixInfo + "Starting backup service...")

	config, err := loadConfig(*envFile)
	if err != nil {
		log.Fatalf(LogPrefixError+"Config error: %v", err)
	}

	if err := validateConfig(config); err != nil {
		log.Fatalf(LogPrefixError+"Config validation failed: %v", err)
	}

	c := cron.New()
	for _, backup := range config.Backups {
		backup := backup
		log.Printf(LogPrefixInfo+"Scheduling backup '%s' with cron: %s", backup.Name, backup.Frequency)

		_, err := c.AddFunc(backup.Frequency, func() {
			startTime := time.Now()
			log.Printf(LogPrefixInfo+"[%s] Backup started", backup.Name)

			if err := processBackup(backup, config); err != nil {
				log.Printf(LogPrefixError+"[%s] Backup failed: %v", backup.Name, err)
				sendAlert(config, backup.Name, err)
			} else {
				log.Printf(LogPrefixInfo+"[%s] Backup completed in %s",
					backup.Name, time.Since(startTime).Round(time.Second))
			}
		})

		if err != nil {
			log.Printf(LogPrefixError+"[%s] Schedule failed: %v", backup.Name, err)
		}
	}
	c.Start()

	log.Println(LogPrefixInfo + "Service started successfully")
	select {} // Keep main goroutine alive
}

func loadConfig(envFile string) (*Config, error) {
	envMap, err := godotenv.Read(envFile)
	if err != nil {
		return nil, err
	}

	backupCount := 0
	fmt.Sscan(envMap["BACKUP_COUNT"], &backupCount)

	backups := make([]BackupConfig, backupCount)
	for i := 0; i < backupCount; i++ {
		prefix := fmt.Sprintf("BACKUP_%d_", i+1)
		backups[i] = BackupConfig{
			Name:       envMap[prefix+"NAME"],
			Source:     envMap[prefix+"SOURCE"],
			RemotePath: envMap[prefix+"REMOTE_PATH"],
			Frequency:  envMap[prefix+"FREQUENCY"],
			Retries:    3,
		}
	}

	return &Config{
		WebdavURL:      envMap["WEBDAV_URL"],
		WebdavUser:     envMap["WEBDAV_USER"],
		WebdavPassword: envMap["WEBDAV_PASSWORD"],
		CacheDir:       envMap["CACHE_DIR"],
		SMTPhost:       envMap["SMTP_HOST"],
		SMTPort:        587,
		SMTPUser:       envMap["SMTP_USER"],
		SMTPPassword:   envMap["SMTP_PASSWORD"],
		SMTPTo:         strings.Split(envMap["SMTP_TO"], ","),
		Backups:        backups,
	}, nil
}

func validateConfig(config *Config) error {
	var errs []string

	if config.WebdavURL == "" {
		errs = append(errs, "WEBDAV_URL is required")
	}
	if config.WebdavUser == "" {
		errs = append(errs, "WEBDAV_USER is required")
	}
	if config.CacheDir == "" {
		errs = append(errs, "CACHE_DIR is required")
	} else if !dirWritable(config.CacheDir) {
		errs = append(errs, fmt.Sprintf("CACHE_DIR '%s' is not writable", config.CacheDir))
	}

	for i, backup := range config.Backups {
		prefix := fmt.Sprintf("BACKUP_%d", i+1)
		if backup.Name == "" {
			errs = append(errs, prefix+".NAME is required")
		}
		if !dirExists(backup.Source) {
			errs = append(errs, fmt.Sprintf("%s.SOURCE '%s' does not exist", prefix, backup.Source))
		}
		if backup.RemotePath == "" {
			errs = append(errs, prefix+".REMOTE_PATH is required")
		}
		if _, err := cron.ParseStandard(backup.Frequency); err != nil {
			errs = append(errs, fmt.Sprintf("%s.FREQUENCY '%s' is invalid: %v",
				prefix, backup.Frequency, err))
		}
	}

	if config.SMTPhost != "" {
		if len(config.SMTPTo) == 0 {
			errs = append(errs, "SMTP_TO is required when SMTP is configured")
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func dirWritable(path string) bool {
	testFile := filepath.Join(path, ".testwrite")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return false
	}
	os.Remove(testFile)
	return true
}

func processBackup(backup BackupConfig, config *Config) error {
	log.Printf(LogPrefixInfo+"[%s] Creating archive...", backup.Name)

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.tar.gz", backup.Name, timestamp)
	tempPath := filepath.Join(config.CacheDir, filename)

	if err := createArchive(backup.Source, tempPath); err != nil {
		return fmt.Errorf("archive creation failed: %w", err)
	}
	defer func() {
		if err := os.Remove(tempPath); err != nil {
			log.Printf(LogPrefixWarn+"[%s] Failed to clean cache: %v", backup.Name, err)
		}
	}()

	fileInfo, _ := os.Stat(tempPath)
	log.Printf(LogPrefixInfo+"[%s] Created archive (%.2f MB)",
		backup.Name, float64(fileInfo.Size())/1024/1024)

	client := gowebdav.NewClient(config.WebdavURL, config.WebdavUser, config.WebdavPassword)
	remotePath := fmt.Sprintf("%s/%s", backup.RemotePath, filename)

	var lastErr error
	for i := 0; i < backup.Retries; i++ {
		log.Printf(LogPrefixInfo+"[%s] Uploading (attempt %d/%d)...",
			backup.Name, i+1, backup.Retries)

		if err := uploadFile(client, tempPath, remotePath); err == nil {
			log.Printf(LogPrefixInfo+"[%s] Upload successful", backup.Name)
			return nil
		} else {
			lastErr = err
			waitTime := time.Second * time.Duration(1<<i)
			log.Printf(LogPrefixWarn+"[%s] Upload failed: %v (retrying in %s)",
				backup.Name, err, waitTime)
			time.Sleep(waitTime)
		}
	}
	return fmt.Errorf("upload failed after %d attempts: %w", backup.Retries, lastErr)
}

func createArchive(sourceDir string, targetPath string) error {
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(sourceDir, path)
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

func uploadFile(client *gowebdav.Client, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return client.WriteStream(remotePath, file, 0644)
}

func sendAlert(config *Config, backupName string, err error) {
	if config.SMTPhost == "" {
		return
	}

	auth := smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPhost)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: Backup Failed: %s\r\n"+
		"\r\n"+
		"Backup '%s' failed with error:\n%s",
		strings.Join(config.SMTPTo, ","),
		backupName,
		backupName,
		err.Error(),
	))

	smtpAddr := fmt.Sprintf("%s:%d", config.SMTPhost, config.SMTPort)
	if sendErr := smtp.SendMail(smtpAddr, auth, config.SMTPUser, config.SMTPTo, msg); sendErr != nil {
		log.Printf(LogPrefixError+"Failed to send alert: %v", sendErr)
	}
}
