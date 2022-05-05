package api

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNew(t *testing.T) {
	c, err := NewClient("https://1.2.3.4:5678", "", "")
	require.NoError(t, err)
	require.NotNil(t, c)
}
