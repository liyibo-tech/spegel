package oci

import (
	"context"
	"fmt"
)

const (
	RuntimeKindContainerd = "containerd"
	RuntimeKindCRIO       = "crio"
)

type MirrorConfigRequest struct {
	RuntimeKind            string
	ContainerdConfigPath   string
	CRIORegistriesConfPath string
	CRIORegistriesConfDir  string
	MirroredRegistries     []string
	MirrorTargets          []string
	ResolveTags            bool
	PrependExisting        bool
	Username               string
	Password               string
}

type MirrorCleanupRequest struct {
	RuntimeKind            string
	ContainerdConfigPath   string
	CRIORegistriesConfPath string
	CRIORegistriesConfDir  string
}

func ConfigureMirrors(ctx context.Context, req MirrorConfigRequest) error {
	runtimeKind, err := normalizeRuntimeKind(req.RuntimeKind)
	if err != nil {
		return err
	}

	switch runtimeKind {
	case RuntimeKindContainerd:
		return AddMirrorConfiguration(
			ctx,
			req.ContainerdConfigPath,
			req.MirroredRegistries,
			req.MirrorTargets,
			req.ResolveTags,
			req.PrependExisting,
			req.Username,
			req.Password,
		)
	case RuntimeKindCRIO:
		return AddCRIOMirrorConfiguration(
			ctx,
			req.CRIORegistriesConfPath,
			req.CRIORegistriesConfDir,
			req.MirroredRegistries,
			req.MirrorTargets,
			req.ResolveTags,
			req.PrependExisting,
			req.Username,
			req.Password,
		)
	default:
		return fmt.Errorf("unsupported runtime kind %q", runtimeKind)
	}
}

func CleanupMirrors(ctx context.Context, req MirrorCleanupRequest) error {
	runtimeKind, err := normalizeRuntimeKind(req.RuntimeKind)
	if err != nil {
		return err
	}

	switch runtimeKind {
	case RuntimeKindContainerd:
		return CleanupMirrorConfiguration(ctx, req.ContainerdConfigPath)
	case RuntimeKindCRIO:
		return CleanupCRIOMirrorConfiguration(ctx, req.CRIORegistriesConfPath, req.CRIORegistriesConfDir)
	default:
		return fmt.Errorf("unsupported runtime kind %q", runtimeKind)
	}
}

func normalizeRuntimeKind(runtimeKind string) (string, error) {
	switch runtimeKind {
	case "", RuntimeKindContainerd:
		return RuntimeKindContainerd, nil
	case RuntimeKindCRIO:
		return RuntimeKindCRIO, nil
	default:
		return "", fmt.Errorf("unknown runtime kind %q", runtimeKind)
	}
}
