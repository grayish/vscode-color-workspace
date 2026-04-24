package runner

import (
	"fmt"
	"path/filepath"

	"github.com/sang-bin/vscode-color-workspace/internal/color"
	"github.com/sang-bin/vscode-color-workspace/internal/vscodesettings"
)

// ColorSource indicates where the final color came from.
type ColorSource int

const (
	SourceFlag ColorSource = iota + 1
	SourceSettings
	SourceRandom
)

// ResolveColor applies the priority rules:
//  1. Explicit flag wins
//  2. peacock.color from .vscode/settings.json
//  3. Random
func ResolveColor(targetDir, flag string) (color.Color, ColorSource, error) {
	if flag != "" {
		c, err := color.Parse(flag)
		if err != nil {
			return color.Color{}, 0, fmt.Errorf("--color: %w", err)
		}
		return c, SourceFlag, nil
	}
	s, err := vscodesettings.Read(filepath.Join(targetDir, ".vscode", "settings.json"))
	if err != nil {
		return color.Color{}, 0, err
	}
	if s != nil {
		if pc, ok := s.PeacockColor(); ok {
			c, err := color.Parse(pc)
			if err != nil {
				return color.Color{}, 0, fmt.Errorf("peacock.color in settings: %w", err)
			}
			return c, SourceSettings, nil
		}
	}
	return color.Random(), SourceRandom, nil
}
