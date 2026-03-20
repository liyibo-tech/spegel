package oci

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
)

const (
	crioSpegelConfigName   = "99-spegel-mirrors.conf"
	crioSpegelBackupSuffix = ".bak"
)

// AddCRIOMirrorConfiguration writes mirror config in registries.conf.d as a drop-in file.
func AddCRIOMirrorConfiguration(
	ctx context.Context,
	registriesConfPath string,
	registriesConfDir string,
	mirroredRegistries []string,
	mirrorTargets []string,
	resolveTags bool,
	_ bool,
	username string,
	password string,
) error {
	log := logr.FromContextOrDiscard(ctx)

	if registriesConfPath != "" {
		if _, err := os.Stat(registriesConfPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	parsedMirroredRegistries, err := parseRegistries(mirroredRegistries, true)
	if err != nil {
		return err
	}
	parsedMirrorTargets, err := parseRegistries(mirrorTargets, false)
	if err != nil {
		return err
	}

	content, err := templateCRIORegistries(parsedMirroredRegistries, parsedMirrorTargets, resolveTags, username, password)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(registriesConfDir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(registriesConfDir, crioSpegelConfigName)
	backupPath := cfgPath + crioSpegelBackupSuffix
	if err := backupFileIfExists(cfgPath, backupPath); err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		return err
	}

	log.Info("added CRI-O mirror configuration", "path", cfgPath)
	return nil
}

func CleanupCRIOMirrorConfiguration(ctx context.Context, _ string, registriesConfDir string) error {
	log := logr.FromContextOrDiscard(ctx)

	cfgPath := filepath.Join(registriesConfDir, crioSpegelConfigName)
	backupPath := cfgPath + crioSpegelBackupSuffix

	if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Rename(backupPath, cfgPath); err != nil {
			return err
		}
		log.Info("recovered CRI-O mirror configuration", "path", cfgPath)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	log.Info("cleaned up CRI-O mirror configuration", "path", cfgPath)
	return nil
}

func backupFileIfExists(path string, backupPath string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(backupPath, b, 0o644)
}

func templateCRIORegistries(mirroredRegistries []url.URL, mirrorTargets []url.URL, resolveTags bool, username, password string) (string, error) {
	type mirrorHost struct {
		Location string
		Insecure bool
	}
	type registryConfig struct {
		Prefix             string
		Location           string
		Insecure           bool
		MirrorByDigestOnly bool
		Mirrors            []mirrorHost
	}

	authorization := ""
	if username != "" || password != "" {
		authorization = username + ":" + password
		authorization = base64.StdEncoding.EncodeToString([]byte(authorization))
		authorization = "Basic " + authorization
	}

	mirrors := make([]mirrorHost, 0, len(mirrorTargets))
	for _, mt := range mirrorTargets {
		mirrors = append(mirrors, mirrorHost{
			Location: mt.Host,
			Insecure: mt.Scheme == "http",
		})
	}

	configs := make([]registryConfig, 0, len(mirroredRegistries))
	for _, mr := range mirroredRegistries {
		prefix := mr.Host
		location := mr.Host
		if mr == wildcardRegistryURL {
			prefix = ""
			location = ""
		}
		configs = append(configs, registryConfig{
			Prefix:             prefix,
			Location:           location,
			Insecure:           mr.Scheme == "http",
			MirrorByDigestOnly: !resolveTags,
			Mirrors:            mirrors,
		})
	}

	tmpl, err := template.New("").Parse(`{{- if .Authorization }}
# Spegel credentials are provided in clear text and should be scoped to isolated clusters.
# Authentication for CRI-O mirrors can also be configured via auth.json.
# Authorization: {{ .Authorization }}
{{- end }}
{{- range .Configs }}
[[registry]]
prefix = '{{ .Prefix }}'
location = '{{ .Location }}'
insecure = {{ .Insecure }}
mirror-by-digest-only = {{ .MirrorByDigestOnly }}
{{- range .Mirrors }}
[[registry.mirror]]
location = '{{ .Location }}'
insecure = {{ .Insecure }}
{{- end }}

{{- end }}`)
	if err != nil {
		return "", err
	}
	payload := struct {
		Authorization string
		Configs       []registryConfig
	}{
		Authorization: authorization,
		Configs:       configs,
	}
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
