package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDemoConfigManager(t *testing.T) {
	err := DemoConfigManager()
	require.NoError(t, err)
}

func TestDemoConfigManagerFeatures(t *testing.T) {
	// This test just runs the features demo
	DemoConfigManagerFeatures()
}