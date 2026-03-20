package oci

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var errCRIOStoreUnimplemented = errors.New("crio store is not implemented yet")

type CRIO struct {
	sock        string
	storagePath string
}

func NewCRIO(_ context.Context, sock string, storagePath string) (*CRIO, error) {
	return nil, fmt.Errorf("%w: runtime=%s sock=%s storage=%s", errCRIOStoreUnimplemented, RuntimeKindCRIO, sock, storagePath)
}

func (c *CRIO) Close() error {
	return nil
}

func (c *CRIO) Name() string {
	return RuntimeKindCRIO
}

func (c *CRIO) ListImages(_ context.Context) ([]Image, error) {
	return nil, errCRIOStoreUnimplemented
}

func (c *CRIO) ListContent(_ context.Context) ([][]Reference, error) {
	return nil, errCRIOStoreUnimplemented
}

func (c *CRIO) Resolve(_ context.Context, _ string) (digest.Digest, error) {
	return "", errCRIOStoreUnimplemented
}

func (c *CRIO) Descriptor(_ context.Context, _ digest.Digest) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{}, errCRIOStoreUnimplemented
}

func (c *CRIO) Open(_ context.Context, _ digest.Digest) (io.ReadSeekCloser, error) {
	return nil, errCRIOStoreUnimplemented
}

func (c *CRIO) Subscribe(_ context.Context) (<-chan OCIEvent, error) {
	return nil, errCRIOStoreUnimplemented
}
