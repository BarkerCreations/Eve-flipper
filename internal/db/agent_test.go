package db

import (
	"testing"
	"time"
)

func TestAgentCooldowns_SetAndCheck(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	if d.IsAgentCooldownActive(111, 34) {
		t.Fatal("expected no cooldown before setting one")
	}

	d.SetAgentCooldown(111, 34, 6*time.Minute)

	if !d.IsAgentCooldownActive(111, 34) {
		t.Fatal("expected cooldown to be active after setting")
	}
}

func TestAgentCooldowns_ExpiredIsInactive(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	d.SetAgentCooldownUntil(222, 34, time.Now().Add(-1*time.Second))

	if d.IsAgentCooldownActive(222, 34) {
		t.Fatal("expected expired cooldown to be inactive")
	}
}

func TestAgentGetLatestStationScanID_ReturnsZeroWhenEmpty(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	_, ok := d.GetLatestStationScanID()
	if ok {
		t.Fatal("expected no scan ID when history is empty")
	}
}
