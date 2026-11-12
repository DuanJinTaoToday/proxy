package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/recorder"
	"gopkg.in/yaml.v3"
)

type gostProcess struct {
	boot   string
	exited chan struct{}
}

func startGOST(ctx context.Context, binary, template, spool, authEndpoint, token string) (gostProcess, error) {
	boot := rand.Text()
	dir := filepath.Join(spool, boot)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return gostProcess{}, err
	}
	path := filepath.Join(dir, recorder.SessionFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return gostProcess{}, err
	}
	if err := file.Close(); err != nil {
		return gostProcess{}, err
	}
	rendered, err := renderGOST(template, path, token)
	if err != nil {
		return gostProcess{}, err
	}
	config, err := os.CreateTemp("", "proxy-gost-*.json")
	if err != nil {
		return gostProcess{}, err
	}
	configPath := config.Name()
	if _, err = config.Write(rendered); err != nil {
		_ = config.Close()
		_ = os.Remove(configPath)
		return gostProcess{}, err
	}
	if err := config.Close(); err != nil {
		_ = os.Remove(configPath)
		return gostProcess{}, err
	}
	if err := validateGOSTConfig(configPath, authEndpoint, token); err != nil {
		_ = os.Remove(configPath)
		return gostProcess{}, err
	}
	childCtx, stop := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, binary, "-C", configPath)
	cmd.Env = gostEnvironment(os.Environ())
	cmd.Cancel = func() error {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdout = gostRecorderLog{stop: stop, path: strings.ToLower(path)}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		stop()
		_ = os.Remove(configPath)
		return gostProcess{}, err
	}
	process := gostProcess{boot: boot, exited: make(chan struct{})}
	go func() {
		defer close(process.exited)
		defer stop()
		err := cmd.Wait()
		exitedAt := time.Now().UTC().Format(time.RFC3339Nano)
		if markerErr := persistExit(dir, exitedAt); markerErr != nil {
			slog.Error("persist GOST exit marker", "boot", boot, "error", markerErr)
		}
		if removeErr := os.Remove(configPath); removeErr != nil {
			slog.Warn("remove rendered GOST configuration", "error", removeErr)
		}
		if err != nil && ctx.Err() == nil {
			slog.Warn("GOST exited", "boot", boot, "error", err)
		}
	}()
	return process, nil
}

func renderGOST(template, path, token string) ([]byte, error) {
	raw, err := os.ReadFile(template)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := yaml.Unmarshal(raw, &config); err != nil || config == nil {
		return nil, errors.New("invalid GOST YAML template")
	}
	if _, exists := config["recorders"]; exists {
		return nil, errors.New("recorder is managed by the GOST wrapper")
	}
	sid, err := rand.Int(rand.Reader, big.NewInt(1_000_000_000))
	if err != nil {
		return nil, err
	}
	values := map[string]string{
		"PROXY_GATEWAY_LOCAL_AUTH_TOKEN": token,
		"WUKONG_SID":                     fmt.Sprintf("%09d", sid.Int64()),
		"WUKONG_ADDRESS":                 os.Getenv("WUKONG_ADDRESS"),
		"WUKONG_USERNAME":                os.Getenv("WUKONG_USERNAME"),
		"WUKONG_PASSWORD":                os.Getenv("WUKONG_PASSWORD"),
	}
	var substitutions []string
	for key, value := range values {
		marker := "${" + key + "}"
		if value == "" && strings.Contains(string(raw), marker) {
			return nil, fmt.Errorf("set %s for the GOST template", key)
		}
		substitutions = append(substitutions, marker, value)
	}
	replaceValues(config, strings.NewReplacer(substitutions...))
	config["recorders"] = []map[string]any{{"name": "proxy-sessions", "file": map[string]any{"path": path, "sep": "\n"}}}
	services, _ := config["services"].([]any)
	for _, value := range services {
		service, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("invalid GOST service")
		}
		handler, ok := service["handler"].(map[string]any)
		if !ok {
			return nil, errors.New("invalid GOST handler")
		}
		if _, exists := service["recorders"]; exists {
			return nil, errors.New("service Recorder is managed by the GOST wrapper")
		}
		service["recorders"] = []map[string]any{{"name": "proxy-sessions", "record": "recorder.service.handler"}}
		metadata, _ := handler["metadata"].(map[string]any)
		if metadata == nil {
			metadata = map[string]any{}
			handler["metadata"] = metadata
		}
		metadata["recorder.period"] = "1s"
	}
	return json.Marshal(config)
}

func replaceValues(value any, replacer *strings.Replacer) any {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			item[key] = replaceValues(child, replacer)
		}
	case []any:
		for i, child := range item {
			item[i] = replaceValues(child, replacer)
		}
	case string:
		return replacer.Replace(item)
	}
	return value
}

type gostRecorderLog struct {
	stop context.CancelFunc
	path string
}

var _ io.Writer = gostRecorderLog{}

func (l gostRecorderLog) Write(raw []byte) (int, error) {
	message := strings.ReplaceAll(strings.ToLower(string(raw)), `\\`, `\`)
	if strings.Contains(message, "open "+l.path) {
		slog.Error("GOST could not open the Recorder file")
		l.stop()
	} else if strings.Contains(message, "record:") && strings.Contains(message, l.path) {
		slog.Error("GOST Recorder write failed")
	}
	return len(raw), nil
}

func gostEnvironment(environ []string) []string {
	result := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if !strings.HasPrefix(key, "PROXY_GATEWAY_") && !strings.HasPrefix(key, "WUKONG_") {
			result = append(result, entry)
		}
	}
	return result
}

func persistExit(dir, exitedAt string) error {
	var err error
	for attempt := range 3 {
		if attempt != 0 {
			time.Sleep(200 * time.Millisecond)
		}
		if err = recorder.WriteDurable(dir, recorder.ExitFile, exitedAt); err == nil {
			return nil
		}
	}
	return err
}
