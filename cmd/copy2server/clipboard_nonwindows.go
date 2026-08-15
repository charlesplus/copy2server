//go:build !windows

package main

import (
	"context"
	"errors"
)

func readWindowsClipboardFiles(ctx context.Context) ([]uploadCandidate, error) {
	return nil, errors.New("windows clipboard files are unsupported on this platform")
}

func writeWindowsClipboardText(ctx context.Context, text string) error {
	return errors.New("windows clipboard text is unsupported on this platform")
}
