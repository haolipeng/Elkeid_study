package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	plugins "github.com/bytedance/plugins"
	plog "github.com/bytedance/plugins/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogFormat represents the type of log source
type LogFormat int

const (
	LogFormatJSON LogFormat = iota
	LogFormatLog
)

// JournalEntry represents a parsed journal entry (JSON format)
type JournalEntry struct {
	Message   string `json:"MESSAGE"`
	PID       string `json:"_PID"`
	Timestamp string `json:"__REALTIME_TIMESTAMP"`
}

// Entry represents a normalized log entry
type Entry struct {
	Message   string
	PID       string
	Timestamp string
}

var (
	logger     *zap.Logger
	client     *plugins.Client
	childCmd   *exec.Cmd
	childMu    sync.Mutex
	parser     *SSHDParser
)

func findCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func main() {
	// Initialize client
	client = plugins.New()

	// Initialize logger
	logger = plog.New(plog.Config{
		MaxSize:     5, // 5 MB
		Path:        "./journal_watcher.log",
		FileLevel:   zapcore.InfoLevel,
		RemoteLevel: zapcore.ErrorLevel,
		MaxBackups:  10,
		Compress:    true,
		Client:      client,
	})
	defer logger.Sync()

	logger.Info("journal_watcher startup")

	// Initialize parser
	parser = NewSSHDParser()

	// Start the record sending goroutine
	go sendRecordLoop()

	// Main task receiving loop
	receiveTaskLoop()

	logger.Info("journal_watcher exited")
}

func sendRecordLoop() {
	for {
		var cmd *exec.Cmd
		var format LogFormat

		// Determine log source
		if findCommand("journalctl") {
			cmd = exec.Command("journalctl", "-f", "_COMM=sshd", "-o", "json")
			format = LogFormatJSON
		} else if _, err := os.Stat("/var/log/auth.log"); err == nil {
			cmd = exec.Command("tail", "-F", "/var/log/auth.log")
			format = LogFormatLog
		} else if _, err := os.Stat("/var/log/secure"); err == nil {
			cmd = exec.Command("tail", "-F", "/var/log/secure")
			format = LogFormatLog
		} else {
			logger.Error("no supported log source")
			return
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error("failed to get stdout pipe", zap.Error(err))
			return
		}

		if err := cmd.Start(); err != nil {
			logger.Error("spawn subprocess failed", zap.Error(err))
			return
		}

		childMu.Lock()
		childCmd = cmd
		childMu.Unlock()

		reader := bufio.NewScanner(stdout)
		for reader.Scan() {
			line := reader.Text()
			logger.Debug("read line", zap.String("line", line))

			entry, ok := parseLine(line, format)
			if !ok {
				continue
			}

			rec := processEntry(entry)
			if rec == nil {
				continue
			}

			if err := client.SendRecord(rec); err != nil {
				logger.Error("failed to send record", zap.Error(err))
				killChild()
				return
			}
		}

		if err := reader.Err(); err != nil {
			logger.Error("when reading a line, an error occurred", zap.Error(err))
		}

		// Wait for child process to exit
		childMu.Lock()
		if childCmd != nil {
			childCmd.Process.Kill()
			err := childCmd.Wait()
			if err != nil {
				logger.Error("journalctl has exited with error", zap.Error(err))
			} else {
				logger.Info("journalctl has exited")
			}
			childCmd = nil
		}
		childMu.Unlock()

		// Sleep before retry
		time.Sleep(10 * time.Second)
	}
}

func parseLine(line string, format LogFormat) (*Entry, bool) {
	switch format {
	case LogFormatJSON:
		var journalEntry JournalEntry
		if err := json.Unmarshal([]byte(line), &journalEntry); err != nil {
			logger.Warn("when parsing a line, an error occurred", zap.Error(err))
			return nil, false
		}

		// Convert microseconds to seconds
		timestamp := journalEntry.Timestamp
		if len(timestamp) > 6 {
			timestamp = timestamp[:len(timestamp)-6]
		} else {
			timestamp = "0"
		}

		return &Entry{
			Message:   journalEntry.Message,
			PID:       journalEntry.PID,
			Timestamp: timestamp,
		}, true

	case LogFormatLog:
		fields := strings.Fields(line)
		if len(fields) < 6 {
			return nil, false
		}
		if !strings.HasPrefix(fields[4], "sshd[") {
			return nil, false
		}

		// Extract PID from "sshd[1234]:"
		pidField := fields[4]
		pid := strings.TrimPrefix(pidField, "sshd[")
		pid = strings.TrimSuffix(pid, "]:")
		pid = strings.TrimSuffix(pid, "]")

		message := strings.Join(fields[5:], " ")

		// Parse timestamp (format: "Jan  1 12:34:56")
		dateStr := strings.Join(fields[0:3], " ")
		timestamp := parseLogTimestamp(dateStr)

		return &Entry{
			Message:   message,
			PID:       pid,
			Timestamp: timestamp,
		}, true
	}

	return nil, false
}

func parseLogTimestamp(dateStr string) string {
	// Try to parse syslog format: "Jan  2 15:04:05"
	layouts := []string{
		"Jan  2 15:04:05",
		"Jan 2 15:04:05",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, dateStr)
		if err == nil {
			// Add current year since syslog doesn't include year
			now := time.Now()
			t = t.AddDate(now.Year(), 0, 0)
			return strconv.FormatInt(t.Unix(), 10)
		}
	}

	return ""
}

func processEntry(entry *Entry) *plugins.Record {
	// Try to parse as login event
	if loginEvent, ok := parser.ParseLogin(entry.Message); ok {
		rec := &plugins.Record{
			DataType: 4000,
			Data: &plugins.Payload{
				Fields: map[string]string{
					"status":  loginEvent.Status,
					"types":   loginEvent.Types,
					"invalid": loginEvent.Invalid,
					"user":    loginEvent.User,
					"sip":     loginEvent.SIP,
					"sport":   loginEvent.SPort,
					"extra":   loginEvent.Extra,
					"pid":     entry.PID,
					"rawlog":  entry.Message,
				},
			},
		}
		setTimestamp(rec, entry.Timestamp)
		return rec
	}

	// Try to parse as certify event
	if certifyEvent, ok := parser.ParseCertify(entry.Message); ok {
		rec := &plugins.Record{
			DataType: 4001,
			Data: &plugins.Payload{
				Fields: map[string]string{
					"authorized": certifyEvent.Authorized,
					"principal":  certifyEvent.Principal,
					"pid":        entry.PID,
					"rawlog":     entry.Message,
				},
			},
		}
		setTimestamp(rec, entry.Timestamp)
		return rec
	}

	return nil
}

func setTimestamp(rec *plugins.Record, timestamp string) {
	if ts, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		rec.Timestamp = ts
	} else {
		rec.Timestamp = time.Now().Unix()
	}
}

func killChild() {
	childMu.Lock()
	defer childMu.Unlock()
	if childCmd != nil && childCmd.Process != nil {
		childCmd.Process.Kill()
		childCmd = nil
	}
}

func receiveTaskLoop() {
	for {
		_, err := client.ReceiveTask()
		if err != nil {
			logger.Error("when receiving task, an error occurred", zap.Error(err))
			killChild()
			return
		}
		// Handle task if needed
	}
}
