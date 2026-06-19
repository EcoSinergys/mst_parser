package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	logFile        *os.File
	logger         *log.Logger
	loggingEnabled bool
)

// InitLogger инициализирует файловое логгирование. Создаёт файл parser_YYYY-MM-DD.log.
func InitLogger() error {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку логов %s: %v", logDir, err)
	}

	logFileName := filepath.Join(logDir, fmt.Sprintf("parser_%s.log", time.Now().Format("2006-01-02")))
	var err error
	logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл лога %s: %v", logFileName, err)
	}

	logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	loggingEnabled = true

	LogInfo("Логгирование инициализировано: %s", logFileName)
	fmt.Printf("📝 Лог-файл: %s\n", logFileName)
	return nil
}

// CloseLogger закрывает файл лога.
func CloseLogger() {
	if logFile != nil {
		LogInfo("Логгирование завершено")
		logFile.Close()
	}
}

// LogInfo пишет информационное сообщение в лог и в консоль.
func LogInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("ℹ️ " + msg)
	if loggingEnabled && logger != nil {
		logger.Printf("[INFO] %s", msg)
	}
}

// LogWarn пишет предупреждение в лог и в консоль.
func LogWarn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("⚠️ " + msg)
	if loggingEnabled && logger != nil {
		logger.Printf("[WARN] %s", msg)
	}
}

// LogError пишет ошибку в лог и в консоль.
func LogError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("❌ " + msg)
	if loggingEnabled && logger != nil {
		logger.Printf("[ERROR] %s", msg)
	}
}

// LogDebug пишет отладочное сообщение в лог (без вывода в консоль).
func LogDebug(format string, args ...interface{}) {
	if loggingEnabled && logger != nil {
		msg := fmt.Sprintf(format, args...)
		logger.Printf("[DEBUG] %s", msg)
	}
}
