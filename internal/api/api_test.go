package api

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv(stationAIWikiRAGDisableEnv, "1")
	os.Exit(m.Run())
}
