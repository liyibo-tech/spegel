package oci

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRuntimeKind(t *testing.T) {
	t.Parallel()

	kind, err := normalizeRuntimeKind("")
	require.NoError(t, err)
	require.Equal(t, RuntimeKindContainerd, kind)

	kind, err = normalizeRuntimeKind(RuntimeKindContainerd)
	require.NoError(t, err)
	require.Equal(t, RuntimeKindContainerd, kind)

	kind, err = normalizeRuntimeKind(RuntimeKindCRIO)
	require.NoError(t, err)
	require.Equal(t, RuntimeKindCRIO, kind)

	_, err = normalizeRuntimeKind("cri-containerd")
	require.EqualError(t, err, `unknown runtime kind "cri-containerd"`)
}

func TestConfigureAndCleanupMirrorsCRIO(t *testing.T) {
	t.Parallel()

	confDir := filepath.Join(t.TempDir(), "registries.conf.d")
	err := ConfigureMirrors(t.Context(), MirrorConfigRequest{
		RuntimeKind:           RuntimeKindCRIO,
		CRIORegistriesConfDir: confDir,
		MirroredRegistries:    []string{"https://docker.io"},
		MirrorTargets:         []string{"http://127.0.0.1:5000"},
		ResolveTags:           true,
	})
	require.NoError(t, err)

	cfgPath := filepath.Join(confDir, crioSpegelConfigName)
	b, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Contains(t, string(b), "prefix = 'docker.io'")
	require.Contains(t, string(b), "location = '127.0.0.1:5000'")

	err = CleanupMirrors(t.Context(), MirrorCleanupRequest{
		RuntimeKind:           RuntimeKindCRIO,
		CRIORegistriesConfDir: confDir,
	})
	require.NoError(t, err)
	_, err = os.Stat(cfgPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}
