package config

/*
 * @Author: lwnmengjing
 * @Date: 2021/5/18 1:37 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/18 1:37 下午
 */

import (
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/logger"

	"github.com/mss-boot-io/mss-boot/core/logger/writer"
)

// Logger logger配置
type Logger struct {
	Path       string     `yaml:"path" json:"path"`
	Level      slog.Level `yaml:"level" json:"level"`
	Stdout     string     `yaml:"stdout" json:"stdout"`
	AddSource  bool       `yaml:"addSource" json:"addSource"`
	Cap        uint       `yaml:"cap" json:"cap"`
	Json       bool       `yaml:"json" json:"json"`
	BufferSize uint       `yaml:"bufferSize" json:"bufferSize"`
	// Loki       *Loki      `yaml:"loki" json:"loki"`
}

type Loki struct {
	URL      string            `yaml:"url" json:"url"`
	Labels   map[string]string `yaml:"labels" json:"labels"`
	Interval time.Duration     `yaml:"interval" json:"interval"`
}

func (l *Loki) MergeLabels(labels map[string]string) {
	for k, v := range l.Labels {
		labels[k] = v
	}
	l.Labels = labels
}

// Init 初始化日志
func (e *Logger) Init() {
	var err error
	var output io.Writer
	switch e.Stdout {
	case "file":
		if !pathExist(e.Path) {
			err = pathCreate(e.Path)
			if err != nil {
				slog.Error("create dir error", slog.Any("error", err))
				os.Exit(-1)
			}
		}
		output, err = writer.NewFileWriter(
			writer.WithPath(e.Path),
			writer.WithSuffix("log"),
			writer.WithCap(e.Cap),
		)
		if err != nil {
			slog.Error("logger setup error", slog.Any("error", err))
			os.Exit(-1)
		}
	// case "loki":
	//	opts := make([]writer.Option, 0)
	//	if e.Loki != nil && e.Loki.URL != "" {
	//		opts = append(opts, writer.WithLokiEndpoint(e.Loki.URL))
	//	}
	//	if e.Loki != nil && len(e.Loki.Labels) > 0 {
	//		opts = append(opts, writer.WithLokiLabels(e.Loki.Labels))
	//	}
	//	if e.Loki != nil && e.Loki.Interval > 0 {
	//		opts = append(opts, writer.WithLokiInterval(e.Loki.Interval))
	//	}
	//	output, err = writer.NewLokiWriter(opts...)
	//	if err != nil {
	//		slog.Error("logger setup error", slog.Any("error", err))
	//		os.Exit(-1)
	//	}
	default:
		output = os.Stderr
	}
	if e.Json {
		slog.SetDefault(slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
			AddSource: e.AddSource,
			Level:     e.Level,
		})))
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		AddSource: e.AddSource,
		Level:     e.Level,
	})))

	// set gorm default logger
	logger.Default = logger.New(log.New(output, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  e.GormLevel(),
		IgnoreRecordNotFoundError: false,
		Colorful:                  true,
	})

	gin.DefaultWriter = output
}

func (e *Logger) GormLevel() logger.LogLevel {
	switch e.Level {
	case slog.LevelDebug, slog.LevelInfo:
		return logger.Info
	case slog.LevelWarn:
		return logger.Warn
	case slog.LevelError:
		return logger.Error
	default:
		return logger.Silent
	}
}

func pathCreate(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

// pathExist 判断目录是否存在
func pathExist(addr string) bool {
	s, err := os.Stat(addr)
	if err != nil {
		return false
	}
	return s.IsDir()
}
