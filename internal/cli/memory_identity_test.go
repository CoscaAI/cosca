package cli

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfirmPresence_Match(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("abc123\n"))
	require.NoError(t, confirmPresence(r, "abc123"))
}

func TestConfirmPresence_Mismatch(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("wrong\n"))
	require.Error(t, confirmPresence(r, "abc123"))
}

func TestConfirmPresence_Empty(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	require.Error(t, confirmPresence(r, "abc123"))
}
